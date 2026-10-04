package retentionexport

import (
	"context"
	"errors"
	"testing"
)

type connectionSink struct {
	payload []byte
	corrupt bool
	fail    bool
}

func (s *connectionSink) Name() string { return "test" }
func (s *connectionSink) Put(_ context.Context, _ string, payload []byte) (string, error) {
	if s.fail {
		return "", errors.New("unavailable")
	}
	s.payload = append([]byte(nil), payload...)
	return "test://connection", nil
}
func (s *connectionSink) Get(context.Context, string) ([]byte, error) {
	if s.corrupt {
		return []byte("corrupt"), nil
	}
	return s.payload, nil
}
func TestSinkConnectionRequiresMatchingReadBack(t *testing.T) {
	for _, tc := range []struct {
		sink    connectionSink
		success bool
	}{{connectionSink{}, true}, {connectionSink{corrupt: true}, false}, {connectionSink{fail: true}, false}} {
		if err := CheckSinkConnection(context.Background(), &tc.sink, "radar-retention"); (err == nil) != tc.success {
			t.Fatalf("connection check success=%v error=%v", tc.success, err)
		}
	}
}
