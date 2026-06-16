package services

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PumpPortalClient struct {
	Config PumpPortalConfig
}

type PumpPortalEvent struct {
	Type       string         `json:"type"`
	Signature  string         `json:"signature,omitempty"`
	Mint       string         `json:"mint,omitempty"`
	TokenMint  string         `json:"tokenMint,omitempty"`
	Name       string         `json:"name,omitempty"`
	Symbol     string         `json:"symbol,omitempty"`
	Creator    string         `json:"creator,omitempty"`
	Trader     string         `json:"traderPublicKey,omitempty"`
	TxType     string         `json:"txType,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
	ReceivedAt time.Time      `json:"received_at"`
}

func NewPumpPortalClient(cfg PumpPortalConfig) *PumpPortalClient {
	return &PumpPortalClient{Config: cfg}
}

func (c *PumpPortalClient) Start(ctx context.Context, onEvent func(context.Context, PumpPortalEvent) error) {
	if c == nil || !c.Config.Enabled {
		return
	}
	if onEvent == nil {
		return
	}
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := c.run(ctx, onEvent); err != nil && ctx.Err() == nil {
			log.Printf("pumpportal data websocket disconnected: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
		}
	}
}

func (c *PumpPortalClient) run(ctx context.Context, onEvent func(context.Context, PumpPortalEvent) error) error {
	conn, err := dialPumpPortalWebSocket(ctx, c.Config.websocketURL())
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("pumpportal data websocket connected: %s", c.Config.redactedWebsocketHost())
	for _, msg := range []map[string]any{{"method": "subscribeNewToken"}, {"method": "subscribeMigration"}} {
		if err := writeWebSocketText(conn, msg); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			_ = writeWebSocketClose(conn)
			return nil
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
		payload, opcode, err := readWebSocketFrame(conn)
		if err != nil {
			return err
		}
		switch opcode {
		case 1:
			event, ok := parsePumpPortalEvent(payload)
			if !ok {
				continue
			}
			if err := onEvent(ctx, event); err != nil {
				log.Printf("pumpportal event adapter failed: %v", err)
			}
		case 8:
			return fmt.Errorf("websocket closed by server")
		case 9:
			_ = writeWebSocketControl(conn, 10, payload)
		}
	}
}

func parsePumpPortalEvent(payload []byte) (PumpPortalEvent, bool) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return PumpPortalEvent{}, false
	}
	event := PumpPortalEvent{Raw: sanitizePumpPortalRaw(raw), ReceivedAt: time.Now().UTC()}
	event.Type = firstPumpPortalString(raw, "type", "method", "event")
	event.Signature = firstPumpPortalString(raw, "signature", "signatureId", "tx", "txHash")
	event.Mint = firstPumpPortalString(raw, "mint", "tokenMint", "ca", "address")
	event.TokenMint = firstPumpPortalString(raw, "tokenMint", "mint", "ca", "address")
	event.Name = firstPumpPortalString(raw, "name", "tokenName")
	event.Symbol = firstPumpPortalString(raw, "symbol", "ticker")
	event.Creator = firstPumpPortalString(raw, "creator", "creatorPublicKey", "deployer")
	event.Trader = firstPumpPortalString(raw, "traderPublicKey", "trader", "buyer")
	event.TxType = firstPumpPortalString(raw, "txType", "transactionType")
	if strings.TrimSpace(event.Mint) == "" {
		event.Mint = event.TokenMint
	}
	if strings.TrimSpace(event.Mint) == "" {
		return PumpPortalEvent{}, false
	}
	if event.Type == "" {
		event.Type = event.TxType
	}
	if event.Type == "" {
		event.Type = "pumpportal_event"
	}
	return event, true
}

func firstPumpPortalString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case fmt.Stringer:
				if strings.TrimSpace(v.String()) != "" {
					return strings.TrimSpace(v.String())
				}
			}
		}
	}
	return ""
}

func sanitizePumpPortalRaw(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "key") || strings.Contains(lk, "secret") || strings.Contains(lk, "private") {
			continue
		}
		out[k] = v
	}
	return out
}

func dialPumpPortalWebSocket(ctx context.Context, rawURL string) (net.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "wss" && u.Scheme != "ws" {
		return nil, fmt.Errorf("unsupported websocket scheme")
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nUser-Agent: Koschei-Web3-Hub\r\n\r\n", path, u.Host, key)
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	res, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade failed: %s", res.Status)
	}
	accept := res.Header.Get("Sec-WebSocket-Accept")
	if accept != websocketAcceptKey(key) {
		conn.Close()
		return nil, fmt.Errorf("websocket accept key mismatch")
	}
	return &bufferedConn{Conn: conn, reader: reader}, nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func websocketAcceptKey(key string) string {
	h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

func writeWebSocketText(conn net.Conn, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeWebSocketFrame(conn, 1, payload, true)
}

func writeWebSocketClose(conn net.Conn) error { return writeWebSocketControl(conn, 8, nil) }

func writeWebSocketControl(conn net.Conn, opcode byte, payload []byte) error {
	return writeWebSocketFrame(conn, opcode, payload, true)
}

func writeWebSocketFrame(conn net.Conn, opcode byte, payload []byte, mask bool) error {
	header := []byte{0x80 | (opcode & 0x0f)}
	length := len(payload)
	maskBit := byte(0)
	if mask {
		maskBit = 0x80
	}
	if length < 126 {
		header = append(header, maskBit|byte(length))
	} else if length <= 65535 {
		header = append(header, maskBit|126, byte(length>>8), byte(length))
	} else {
		header = append(header, maskBit|127)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(length))
		header = append(header, buf...)
	}
	if mask {
		key := make([]byte, 4)
		if _, err := rand.Read(key); err != nil {
			return err
		}
		header = append(header, key...)
		masked := make([]byte, len(payload))
		for i := range payload {
			masked[i] = payload[i] ^ key[i%4]
		}
		payload = masked
	}
	_, err := conn.Write(append(header, payload...))
	return err
}

func readWebSocketFrame(conn net.Conn) ([]byte, byte, error) {
	h := make([]byte, 2)
	if _, err := io.ReadFull(conn, h); err != nil {
		return nil, 0, err
	}
	opcode := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	length := int64(h[1] & 0x7f)
	if length == 126 {
		buf := make([]byte, 2)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return nil, 0, err
		}
		length = int64(binary.BigEndian.Uint16(buf))
	} else if length == 127 {
		buf := make([]byte, 8)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return nil, 0, err
		}
		length = int64(binary.BigEndian.Uint64(buf))
	}
	if length > 4<<20 {
		return nil, 0, fmt.Errorf("websocket frame too large")
	}
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(conn, maskKey); err != nil {
			return nil, 0, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, 0, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return payload, opcode, nil
}
