package retentionexport

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// CheckSinkConnection proves authenticated write/read access even when the
// archive queue is empty. It is not an archive restore or source-row proof.
func CheckSinkConnection(ctx context.Context, sink Sink, prefix string) error {
	if sink == nil {
		return fmt.Errorf("archive sink unavailable")
	}
	payload := []byte("{\"version\":\"koschei.archive-sink-connection.v1\"}\n")
	key := strings.Trim(strings.TrimSpace(prefix), "/") + "/_connection/" + sha256Hex(payload) + ".json"
	if _, err := sink.Put(ctx, key, payload); err != nil {
		return fmt.Errorf("archive sink connection write failed: %w", err)
	}
	stored, err := sink.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("archive sink connection read failed: %w", err)
	}
	if !bytes.Equal(stored, payload) {
		return fmt.Errorf("archive sink connection checksum mismatch")
	}
	return nil
}
