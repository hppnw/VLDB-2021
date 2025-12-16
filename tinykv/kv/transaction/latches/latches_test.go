package latches

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAcquireLatches(t *testing.T) {
	l := NewLatches()

	// Acquiring a new latch is ok.
	wg := l.AcquireLatches([][]byte{{}, {3}, {3, 0, 42}})
	assert.Nil(t, wg)

	// Can only acquire once.
	wg = l.AcquireLatches([][]byte{{}})
	assert.NotNil(t, wg)
	wg = l.AcquireLatches([][]byte{{3, 0, 42}})
	assert.NotNil(t, wg)

	// Release then acquire is ok.
	l.ReleaseLatches([][]byte{{3}, {3, 0, 43}})
	wg = l.AcquireLatches([][]byte{{3}})
	assert.Nil(t, wg)
	wg = l.AcquireLatches([][]byte{{3, 0, 42}})
	assert.NotNil(t, wg)
}

func BenchmarkLatches_AcquireRelease(b *testing.B) {
	l := NewLatches()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			keys := [][]byte{
				[]byte(fmt.Sprintf("k-%d", r.Int())),
				[]byte(fmt.Sprintf("k-%d", r.Int())),
				[]byte(fmt.Sprintf("k-%d", r.Int())),
			}
			l.WaitForLatches(keys)
			l.ReleaseLatches(keys)
		}
	})
}
