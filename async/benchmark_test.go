package async

import (
	"context"
	"testing"
)

func BenchmarkFan(b *testing.B) {
	items := make([]int, 100)
	for i := 0; i < 100; i++ {
		items[i] = i
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Fan(ctx, items, func(_ context.Context, n int) error {
			return nil
		})
	}
}
