package defense

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestActiveResponseLedgerSerializationRetriesThenSucceeds(t *testing.T) {
	attempts := 0
	result, err := retryShadowLedgerSerialization(context.Background(), func() (ActiveResponseLedgerResult, error) {
		attempts++
		if attempts <= 2 {
			return ActiveResponseLedgerResult{}, fmt.Errorf("database transaction: %w", &pq.Error{Code: "40001"})
		}
		return ActiveResponseLedgerResult{Status: ActiveResponseLedgerRecorded}, nil
	})
	if err != nil || attempts != 3 || result.Status != ActiveResponseLedgerRecorded || result.ExecutionPerformed {
		t.Fatalf("retries did not resolve serialization failure: attempts=%d result=%+v err=%v", attempts, result, err)
	}
}

func TestActiveResponseLedgerRetryExhaustionFailsClosed(t *testing.T) {
	attempts := 0
	result, err := retryShadowLedgerSerialization(context.Background(), func() (ActiveResponseLedgerResult, error) {
		attempts++
		return ActiveResponseLedgerResult{}, &pq.Error{Code: "40001"}
	})
	if err == nil || attempts != 5 || result.ExecutionPerformed || result.Status != "" {
		t.Fatalf("retry exhaustion did not fail closed: attempts=%d result=%+v err=%v", attempts, result, err)
	}
}

func TestActiveResponseLedgerDoesNotRetryNonSerializationErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "uniqueness", err: &pq.Error{Code: "23505"}},
		{name: "permission", err: &pq.Error{Code: "42501"}},
		{name: "connection_failure", err: &pq.Error{Code: "08006"}},
		{name: "unknown_commit", err: errors.New("commit outcome unknown")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			result, err := retryShadowLedgerSerialization(context.Background(), func() (ActiveResponseLedgerResult, error) {
				count++
				return ActiveResponseLedgerResult{}, tc.err
			})
			if err == nil || count != 1 || result.ExecutionPerformed {
				t.Fatalf("unsafe retry: count=%d result=%+v err=%v", count, result, err)
			}
		})
	}
}

func TestActiveResponseLedgerRetryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	count := 0
	result, err := retryShadowLedgerSerialization(ctx, func() (ActiveResponseLedgerResult, error) {
		count++
		return ActiveResponseLedgerResult{}, &pq.Error{Code: "40001"}
	})
	if !errors.Is(err, context.Canceled) || count != 0 || result.ExecutionPerformed {
		t.Fatalf("canceled request attempted DB write: count=%d result=%+v err=%v", count, result, err)
	}
}

func TestActiveResponseLedgerStopsAfterCancellationDuringSerialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	result, err := retryShadowLedgerSerialization(ctx, func() (ActiveResponseLedgerResult, error) {
		calls++
		cancel()
		return ActiveResponseLedgerResult{}, &pq.Error{Code: "40001"}
	})
	if !errors.Is(err, context.Canceled) || calls != 1 || result.ExecutionPerformed {
		t.Fatalf("retry ignored cancellation: calls=%d result=%+v err=%v", calls, result, err)
	}
}
