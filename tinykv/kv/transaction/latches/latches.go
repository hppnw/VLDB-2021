package latches

import (
	"hash/fnv"
	"sort"
	"sync"

	"github.com/pingcap-incubator/tinykv/kv/transaction/mvcc"
)

// Latching provides atomicity of TinyKV commands. This should not be confused with SQL transactions which provide atomicity
// for multiple TinyKV commands. For example, consider two commit commands, these write to multiple keys/CFs so if they race,
// then it is possible for inconsistent data to be written. By latching the keys each command might write, we ensure that the
// two commands will not race to write the same keys.
//
// A latch is a per-key lock. There is only one latch per user key, not one per CF or one for each encoded key. Latches are
// only needed for writing. Only one thread can hold a latch at a time and all keys that a command might write must be locked
// at once.
//
// Latching is implemented using a set of slots, each guarding a subset of keys. This reduces contention compared to a single
// global lock.

const numSlots = 128

type latchSlot struct {
	sync.Mutex
	latchMap map[string]*sync.WaitGroup
}

type Latches struct {
	slots [numSlots]latchSlot
	// An optional validation function, only used for testing.
	Validation func(txn *mvcc.MvccTxn, keys [][]byte)
}

// NewLatches creates a new Latches object for managing a databases latches. There should only be one such object, shared
// between all threads.
func NewLatches() *Latches {
	l := new(Latches)
	for i := 0; i < numSlots; i++ {
		l.slots[i].latchMap = make(map[string]*sync.WaitGroup)
	}
	return l
}

func (l *Latches) getSlotID(key []byte) int {
	h := fnv.New32a()
	h.Write(key)
	return int(h.Sum32()) % numSlots
}

// AcquireLatches tries lock all Latches specified by keys. If this succeeds, nil is returned. If any of the keys are
// locked, then AcquireLatches requires a WaitGroup which the thread can use to be woken when the lock is free.
func (l *Latches) AcquireLatches(keysToLatch [][]byte) *sync.WaitGroup {
	// Identify involved slots
	slotIDs := make(map[int]struct{})
	for _, key := range keysToLatch {
		slotIDs[l.getSlotID(key)] = struct{}{}
	}

	// Sort slot IDs to avoid deadlock
	sortedSlots := make([]int, 0, len(slotIDs))
	for id := range slotIDs {
		sortedSlots = append(sortedSlots, id)
	}
	sort.Ints(sortedSlots)

	// Lock slots
	for _, id := range sortedSlots {
		l.slots[id].Lock()
	}
	defer func() {
		for _, id := range sortedSlots {
			l.slots[id].Unlock()
		}
	}()

	// Check none of the keys we want to write are locked.
	for _, key := range keysToLatch {
		slot := &l.slots[l.getSlotID(key)]
		if latchWg, ok := slot.latchMap[string(key)]; ok {
			// Return a wait group to wait on.
			return latchWg
		}
	}

	// All Latches are available, lock them all with a new wait group.
	wg := new(sync.WaitGroup)
	wg.Add(1)
	for _, key := range keysToLatch {
		slot := &l.slots[l.getSlotID(key)]
		slot.latchMap[string(key)] = wg
	}

	return nil
}

// ReleaseLatches releases the latches for all keys in keysToUnlatch. It will wakeup any threads blocked on one of the
// latches. All keys in keysToUnlatch must have been locked together in one call to AcquireLatches.
func (l *Latches) ReleaseLatches(keysToUnlatch [][]byte) {
	// Identify involved slots
	slotIDs := make(map[int]struct{})
	for _, key := range keysToUnlatch {
		slotIDs[l.getSlotID(key)] = struct{}{}
	}

	// Sort slot IDs
	sortedSlots := make([]int, 0, len(slotIDs))
	for id := range slotIDs {
		sortedSlots = append(sortedSlots, id)
	}
	sort.Ints(sortedSlots)

	// Lock slots
	for _, id := range sortedSlots {
		l.slots[id].Lock()
	}
	defer func() {
		for _, id := range sortedSlots {
			l.slots[id].Unlock()
		}
	}()

	first := true
	for _, key := range keysToUnlatch {
		slot := &l.slots[l.getSlotID(key)]
		if first {
			if wg, ok := slot.latchMap[string(key)]; ok {
				wg.Done()
				first = false
			}
		}
		delete(slot.latchMap, string(key))
	}
}

// WaitForLatches attempts to lock all keys in keysToLatch using AcquireLatches. If a latch ia already locked, then =
// WaitForLatches will wait for it to become unlocked then try again. Therefore WaitForLatches may block for an unbounded
// length of time.
func (l *Latches) WaitForLatches(keysToLatch [][]byte) {
	for {
		wg := l.AcquireLatches(keysToLatch)
		if wg == nil {
			return
		}
		wg.Wait()
	}
}

// Validate calls the function in Validation, if it exists.
func (l *Latches) Validate(txn *mvcc.MvccTxn, latched [][]byte) {
	if l.Validation != nil {
		l.Validation(txn, latched)
	}
}
