package async

import (
	"context"
	"testing"
)

func FuzzMapAndFan(f *testing.F) {
	f.Add(1, 1, 0, false)
	f.Add(10, -1, 5, true)
	f.Add(0, 100, -5, false)
	f.Add(-5, 0, 10, true)

	f.Fuzz(func(t *testing.T, sliceSize, mapConcurrency, errorAtIdx int, triggerPanic bool) {
		if sliceSize < 0 {
			sliceSize = 0
		}
		if sliceSize > 1000 {
			sliceSize = 1000
		}

		items := make([]int, sliceSize)
		for i := 0; i < sliceSize; i++ {
			items[i] = i
		}

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("async map/fan panicked: sliceSize=%d mapConcurrency=%d errorAtIdx=%d triggerPanic=%v panic=%v",
					sliceSize, mapConcurrency, errorAtIdx, triggerPanic, r)
			}
		}()

		_, _ = Map(context.Background(), items, mapConcurrency, func(ctx context.Context, item int) (int, error) {
			if triggerPanic && item == errorAtIdx {
				panic("fuzz test panic")
			}
			return item * 2, nil
		})

		_ = Fan(context.Background(), items, func(ctx context.Context, item int) error {
			if triggerPanic && item == errorAtIdx {
				panic("fuzz test panic")
			}
			return nil
		})
	})
}
