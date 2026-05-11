package grpckit

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
)

func FuzzInterceptors(f *testing.F) {
	f.Add("/service.Method/Call", "garbage input", true, false)
	f.Add("/another.Service/AnotherCall", "another input", false, true)
	f.Add("", "", true, false)
	f.Add("InvalidMethodPath", "SomeRandomPayload\x00\xff", false, false)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	recoveryInterceptor := RecoveryUnaryInterceptor(logger)
	loggingInterceptor := LoggingUnaryInterceptor(logger)

	f.Fuzz(func(t *testing.T, fullMethod, reqData string, triggerPanic, returnError bool) {
		ctx := context.Background()
		info := &grpc.UnaryServerInfo{
			Server:     nil,
			FullMethod: fullMethod,
		}

		handler := func(ctx context.Context, req any) (any, error) {
			if triggerPanic {
				panic(req)
			}
			if returnError {
				return nil, context.DeadlineExceeded
			}
			return req, nil
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RecoveryUnaryInterceptor panicked: fullMethod=%q reqData=%q panic=%v", fullMethod, reqData, r)
				}
			}()
			_, _ = recoveryInterceptor(ctx, reqData, info, handler)
		}()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("LoggingUnaryInterceptor panicked: fullMethod=%q reqData=%q panic=%v", fullMethod, reqData, r)
				}
			}()
			if !triggerPanic {
				_, _ = loggingInterceptor(ctx, reqData, info, handler)
			}
		}()
	})
}
