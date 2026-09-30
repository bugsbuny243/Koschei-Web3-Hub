package jobs

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/workerwake"
)

const (
	NATSWakeHealthID         = "worker.cross-process-job-wake"
	natsWakeReconnectBackoff = 5 * time.Second
	natsWakeStopTimeout      = 5 * time.Second
)

type natsWakeBinding struct {
	Subject  string
	WakeName string
}

func StartNATSWakeBridge(ctx context.Context, rawURL, prefix string, health *runtimehealth.Registry) func() {
	rawURL = strings.TrimSpace(rawURL)
	if health != nil {
		health.Register(NATSWakeHealthID, "worker", "", rawURL != "")
	}
	if rawURL == "" {
		return func() {}
	}

	bridgeCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runNATSWakeBridge(bridgeCtx, rawURL, prefix, health)
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(natsWakeStopTimeout):
		}
		if health != nil {
			health.Stop(NATSWakeHealthID)
		}
	}
}

func runNATSWakeBridge(ctx context.Context, rawURL, prefix string, health *runtimehealth.Registry) {
	for ctx.Err() == nil {
		closed := make(chan struct{}, 1)
		conn, err := nats.Connect(
			rawURL,
			nats.Name("koschei-web3-cross-process-wake"),
			nats.Timeout(natsPublishTimeout),
			nats.MaxReconnects(-1),
			nats.ReconnectWait(natsReconnectWait),
			nats.DisconnectErrHandler(func(_ *nats.Conn, disconnectErr error) {
				if disconnectErr == nil || ctx.Err() != nil {
					return
				}
				log.Printf("NATS cross-process wake disconnected: %v", disconnectErr)
				if health != nil {
					health.Failure(NATSWakeHealthID, disconnectErr)
				}
			}),
			nats.ReconnectHandler(func(conn *nats.Conn) {
				log.Printf("NATS cross-process wake reconnected server=%s", conn.ConnectedUrl())
				if health != nil {
					health.Success(NATSWakeHealthID, 0)
				}
			}),
			nats.ClosedHandler(func(_ *nats.Conn) {
				select {
				case closed <- struct{}{}:
				default:
				}
			}),
		)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("NATS cross-process wake connection failed: %v", err)
			if health != nil {
				health.Failure(NATSWakeHealthID, err)
			}
			if !waitNATSWakeRetry(ctx) {
				return
			}
			continue
		}

		bindings := natsWakeBindings(prefix)
		subscriptions := make([]*nats.Subscription, 0, len(bindings))
		subscribeFailed := false
		for _, binding := range bindings {
			wakeName := binding.WakeName
			sub, subErr := conn.Subscribe(binding.Subject, func(_ *nats.Msg) {
				workerwake.Signal(wakeName)
				if health != nil {
					health.Success(NATSWakeHealthID, 1)
				}
			})
			if subErr != nil {
				err = subErr
				subscribeFailed = true
				break
			}
			subscriptions = append(subscriptions, sub)
		}
		if !subscribeFailed {
			err = conn.FlushTimeout(natsPublishTimeout)
			subscribeFailed = err != nil
		}
		if subscribeFailed {
			for _, sub := range subscriptions {
				_ = sub.Unsubscribe()
			}
			conn.Close()
			if ctx.Err() != nil {
				return
			}
			log.Printf("NATS cross-process wake subscribe failed: %v", err)
			if health != nil {
				health.Failure(NATSWakeHealthID, err)
			}
			if !waitNATSWakeRetry(ctx) {
				return
			}
			continue
		}

		if health != nil {
			health.Success(NATSWakeHealthID, 0)
		}
		log.Printf("NATS cross-process wake live subjects=%d", len(bindings))

		select {
		case <-ctx.Done():
		case <-closed:
		}
		for _, sub := range subscriptions {
			_ = sub.Unsubscribe()
		}
		conn.Close()
		if ctx.Err() != nil {
			return
		}
		if !waitNATSWakeRetry(ctx) {
			return
		}
	}
}

func natsWakeBindings(prefix string) []natsWakeBinding {
	return []natsWakeBinding{
		{Subject: natsJobSubject(prefix, "canonical_investigation"), WakeName: workerwake.CanonicalInvestigation},
		{Subject: natsJobSubject(prefix, "token_scan"), WakeName: workerwake.CanonicalInvestigation},
	}
}

func waitNATSWakeRetry(ctx context.Context) bool {
	timer := time.NewTimer(natsWakeReconnectBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
