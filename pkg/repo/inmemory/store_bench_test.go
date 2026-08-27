package inmemory

import (
	"context"
	"fmt"
	"testing"

	"github.com/ksysoev/mimir/pkg/core"
)

const (
	storeBenchKeys  = 10_000
	storeBenchValue = `{"data":"benchmark-value"}`
)

func seedStore(b *testing.B, s *Store) {
	b.Helper()

	ctx := context.Background()

	for i := range storeBenchKeys {
		_, err := s.Put(ctx, core.Item{
			Key:         fmt.Sprintf("key-%d", i),
			Value:       []byte(storeBenchValue),
			ContentType: "application/json",
		})
		if err != nil {
			b.Fatalf("seed Put failed: %v", err)
		}
	}
}

// BenchmarkStore_Get_ReadHeavy — parallel reads across many pre-seeded keys.
func BenchmarkStore_Get_ReadHeavy(b *testing.B) {
	s := NewStore(Config{MaxKeys: storeBenchKeys + 1})
	seedStore(b, s)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%storeBenchKeys)
			_, _ = s.Get(ctx, key)

			i++
		}
	})
}

// BenchmarkStore_Put_WriteHeavy — parallel writes to many distinct keys (all existing).
func BenchmarkStore_Put_WriteHeavy(b *testing.B) {
	s := NewStore(Config{MaxKeys: storeBenchKeys + 1})
	seedStore(b, s)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%storeBenchKeys)
			_, _ = s.Put(ctx, core.Item{
				Key:         key,
				Value:       []byte(storeBenchValue),
				ContentType: "application/json",
			})

			i++
		}
	})
}

// BenchmarkStore_MixedReadWrite — 90% reads, 10% writes across many pre-seeded keys.
func BenchmarkStore_MixedReadWrite(b *testing.B) {
	s := NewStore(Config{MaxKeys: storeBenchKeys + 1})
	seedStore(b, s)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%storeBenchKeys)
			if i%10 == 0 {
				_, _ = s.Put(ctx, core.Item{
					Key:         key,
					Value:       []byte(storeBenchValue),
					ContentType: "application/json",
				})
			} else {
				_, _ = s.Get(ctx, key)
			}

			i++
		}
	})
}

// BenchmarkStore_HotKey — all goroutines hammer a single hot key with
// equal reads and writes (worst case for sync.Map).
func BenchmarkStore_HotKey(b *testing.B) {
	s := NewStore(Config{})
	ctx := context.Background()

	_, err := s.Put(ctx, core.Item{
		Key:         "hot-key",
		Value:       []byte(storeBenchValue),
		ContentType: "application/json",
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Get(ctx, "hot-key")
			_, _ = s.Put(ctx, core.Item{
				Key:         "hot-key",
				Value:       []byte(storeBenchValue),
				ContentType: "application/json",
			})
		}
	})
}

// BenchmarkStore_Get_NotFound — parallel reads for keys that don't exist.
func BenchmarkStore_Get_NotFound(b *testing.B) {
	s := NewStore(Config{})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("missing-%d", i%storeBenchKeys)
			_, _ = s.Get(ctx, key)

			i++
		}
	})
}

// BenchmarkStore_ListKeys — cost of a full key snapshot under concurrent writes.
func BenchmarkStore_ListKeys(b *testing.B) {
	s := NewStore(Config{MaxKeys: storeBenchKeys + 1})
	seedStore(b, s)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%20 == 0 {
				_ = s.ListKeys(ctx)
			} else {
				key := fmt.Sprintf("key-%d", i%storeBenchKeys)
				_, _ = s.Get(ctx, key)
			}

			i++
		}
	})
}
