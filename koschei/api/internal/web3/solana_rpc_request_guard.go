package web3

import (
	"context"
	"errors"
)

// logRPCFailureUnlessCallerCanceled keeps caller cancellations (an investigation
// that ended or a client that disconnected) out of the provider failure log;
// they are not provider failures, and in production they made up most
// "solana rpc failure ... context canceled" lines, hiding real 429s and
// timeouts. Deadline expiry and every provider error are still logged.
func logRPCFailureUnlessCallerCanceled(ctx context.Context, method, endpoint string, statusCode int, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	LogRPCFailure(method, endpoint, statusCode, err)
}
