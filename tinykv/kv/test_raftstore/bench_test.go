package test_raftstore

import (
	"fmt"
	"testing"
	"time"

	"github.com/pingcap-incubator/tinykv/kv/config"
)

func BenchmarkRaftStore_BasicPut(b *testing.B) {
	// Disable logging to avoid affecting performance
	// log.SetLevel(zap.FatalLevel)

	cfg := config.NewDefaultConfig()
	cfg.Raft = true
	cfg.RaftBaseTickInterval = 50 * time.Millisecond
	cfg.RaftHeartbeatTicks = 2
	cfg.RaftElectionTimeoutTicks = 10
	cluster := NewTestCluster(1, cfg)
	cluster.Start()
	defer cluster.Shutdown()

	// Warmup to ensure leader is elected
	cluster.MustPut([]byte("warmup"), []byte("warmup"))

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("key_%d", i))
		val := []byte(fmt.Sprintf("val_%d", i))
		cluster.MustPut(key, val)
	}
	elapsed := time.Since(start)
	
	// Report metrics
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
	b.ReportAllocs()
}

func BenchmarkRaftStore_ParallelPut(b *testing.B) {
	cfg := config.NewDefaultConfig()
	cfg.Raft = true
	cfg.RaftBaseTickInterval = 50 * time.Millisecond
	cfg.RaftHeartbeatTicks = 2
	cfg.RaftElectionTimeoutTicks = 10
	cluster := NewTestCluster(3, cfg)
	cluster.Start()
	defer cluster.Shutdown()

	// Warmup to ensure leader is elected
	cluster.MustPut([]byte("warmup"), []byte("warmup"))

	b.ResetTimer()
	start := time.Now()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			// Use unique keys to avoid conflicts if any
			key := []byte(fmt.Sprintf("key_%d_%d", time.Now().UnixNano(), i))
			val := []byte(fmt.Sprintf("val_%d_%d", time.Now().UnixNano(), i))
			cluster.MustPut(key, val)
		}
	})
	elapsed := time.Since(start)
	
	// Report metrics
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
	b.ReportAllocs()
}

func BenchmarkRaftStore_Get(b *testing.B) {
	cfg := config.NewDefaultConfig()
	cfg.Raft = true
	cfg.RaftBaseTickInterval = 50 * time.Millisecond
	cfg.RaftHeartbeatTicks = 2
	cfg.RaftElectionTimeoutTicks = 10
	cluster := NewTestCluster(1, cfg)
	cluster.Start()
	defer cluster.Shutdown()

	// Pre-populate data
	for i := 0; i < 1000; i++ {
		key := []byte(fmt.Sprintf("key_%d", i))
		val := []byte(fmt.Sprintf("val_%d", i))
		cluster.MustPut(key, val)
	}

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("key_%d", i%1000))
		val := []byte(fmt.Sprintf("val_%d", i%1000))
		cluster.MustGet(key, val)
	}
	elapsed := time.Since(start)
	
	// Report metrics
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
	b.ReportAllocs()
}
