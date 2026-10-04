package http

import "context"

func StartTradePIAgentRuntime(ctx context.Context, workers bool) func() {
	return tradePIAgentService.StartRuntime(ctx, workers)
}
