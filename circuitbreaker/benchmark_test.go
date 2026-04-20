package circuitbreaker

import (
	"testing"
)

func BenchmarkBreaker_Execute_Success(b *testing.B) {
	breaker := New(DefaultConfig())
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = breaker.Execute(func() error {
				return nil
			})
		}
	})
}
