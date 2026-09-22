package services

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestMinimalWebSocketKeepaliveRunsIndependentlyOfReads(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ws := &minimalWSConn{conn: client, r: bufio.NewReader(client)}
	defer ws.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ws.startKeepalive(ctx, 10*time.Millisecond)

	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(server, header); err != nil {
		t.Fatalf("read ping header: %v", err)
	}
	if got := header[0] & 0x0f; got != 0x9 {
		t.Fatalf("expected ping opcode, got 0x%x", got)
	}
	if header[1]&0x80 == 0 {
		t.Fatal("client websocket frame must be masked")
	}
	length := int(header[1] & 0x7f)
	if length >= 126 {
		t.Fatalf("unexpected keepalive payload length marker %d", length)
	}
	mask := make([]byte, 4)
	if _, err := io.ReadFull(server, mask); err != nil {
		t.Fatalf("read ping mask: %v", err)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(server, payload); err != nil {
		t.Fatalf("read ping payload: %v", err)
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	if string(payload) != "koschei" {
		t.Fatalf("unexpected ping payload %q", payload)
	}
}

func TestMinimalWebSocketKeepaliveCancellationUnblocksRead(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ws := &minimalWSConn{conn: client, r: bufio.NewReader(client)}

	ctx, cancel := context.WithCancel(context.Background())
	go ws.startKeepalive(ctx, time.Hour)

	result := make(chan error, 1)
	go func() {
		_, err := ws.ReadText(ctx)
		result <- err
	}()

	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected blocked read to terminate after context cancellation")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "closed") &&
			!strings.Contains(strings.ToLower(err.Error()), "pipe") &&
			err != io.EOF {
			t.Logf("read terminated with provider/socket error after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("websocket read stayed blocked after keepalive context cancellation")
	}
}
