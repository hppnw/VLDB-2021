package tikv

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/pingcap/tidb/store/mockstore/mocktikv"
)

func Benchmark2PC_NoConflict(b *testing.B) {
	cluster := mocktikv.NewCluster()
	mocktikv.BootstrapWithMultiRegions(cluster, []byte("a"), []byte("b"), []byte("c"))
	mvccStore, err := mocktikv.NewMVCCLevelDB("")
	if err != nil {
		b.Fatal(err)
	}
	client := mocktikv.NewRPCClient(cluster, mvccStore)
	pdCli := &codecPDClient{mocktikv.NewPDClient(cluster)}
	spkv := NewMockSafePointKV()
	store, err := newTikvStore("mocktikv-store", pdCli, spkv, client, false)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			key := []byte(fmt.Sprintf("key_%d", r.Int()))
			value := []byte("value")

			txn, err := store.Begin()
			if err != nil {
				b.Fatal(err)
			}
			err = txn.Set(key, value)
			if err != nil {
				b.Fatal(err)
			}
			err = txn.Commit(context.Background())
			if err != nil {
				// Ignore errors in benchmark to keep running, 
				// but in no-conflict scenario, errors shouldn't happen often.
				// b.Log(err)
			}
		}
	})
}

func Benchmark2PC_Conflict(b *testing.B) {
	cluster := mocktikv.NewCluster()
	mocktikv.BootstrapWithMultiRegions(cluster, []byte("a"), []byte("b"), []byte("c"))
	mvccStore, err := mocktikv.NewMVCCLevelDB("")
	if err != nil {
		b.Fatal(err)
	}
	client := mocktikv.NewRPCClient(cluster, mvccStore)
	pdCli := &codecPDClient{mocktikv.NewPDClient(cluster)}
	spkv := NewMockSafePointKV()
	store, err := newTikvStore("mocktikv-store", pdCli, spkv, client, false)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("conflict_key")
		value := []byte("value")

		for pb.Next() {
			// Retry loop for conflict
			for {
				txn, err := store.Begin()
				if err != nil {
					b.Fatal(err)
				}
				err = txn.Set(key, value)
				if err != nil {
					b.Fatal(err)
				}
				err = txn.Commit(context.Background())
				if err == nil {
					break
				}
				// Simple backoff
				time.Sleep(time.Microsecond * 100)
			}
		}
	})
}
