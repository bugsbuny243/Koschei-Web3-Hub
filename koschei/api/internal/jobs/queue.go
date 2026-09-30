package jobs

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultNATSSubjectPrefix = "koschei.web3"
	natsPublishTimeout       = 3 * time.Second
	natsReconnectWait        = 2 * time.Second
)

type NoopQueue struct{}

func (NoopQueue) Publish(Job) error { return nil }
func (NoopQueue) Close() error      { return nil }

type NATSQueue struct {
	url    string
	prefix string

	mu   sync.Mutex
	conn *nats.Conn
}

func NewNATSQueue(rawURL, prefix string) *NATSQueue {
	return &NATSQueue{
		url:    strings.TrimSpace(rawURL),
		prefix: normalizedNATSPrefix(prefix),
	}
}

func (q *NATSQueue) Publish(job Job) error {
	if q == nil || strings.TrimSpace(q.url) == "" {
		return nil
	}
	conn, err := q.connection()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode NATS wake payload: %w", err)
	}
	if err := conn.Publish(natsJobSubject(q.prefix, job.Type), payload); err != nil {
		return fmt.Errorf("publish NATS wake hint: %w", err)
	}
	if err := conn.FlushTimeout(natsPublishTimeout); err != nil {
		return fmt.Errorf("flush NATS wake hint: %w", err)
	}
	return nil
}

func (q *NATSQueue) connection() (*nats.Conn, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.conn != nil && !q.conn.IsClosed() {
		return q.conn, nil
	}
	conn, err := nats.Connect(
		q.url,
		nats.Name("koschei-web3-job-wake-publisher"),
		nats.Timeout(natsPublishTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(natsReconnectWait),
	)
	if err != nil {
		return nil, fmt.Errorf("connect NATS wake publisher: %w", err)
	}
	q.conn = conn
	return conn, nil
}

func (q *NATSQueue) Close() error {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	conn := q.conn
	q.conn = nil
	q.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	return nil
}

func normalizedNATSPrefix(value string) string {
	value = strings.Trim(strings.TrimSpace(value), ".")
	if value == "" {
		return defaultNATSSubjectPrefix
	}
	if strings.ContainsAny(value, " \t\r\n*>") {
		return defaultNATSSubjectPrefix
	}
	return value
}

func natsJobSubject(prefix, jobType string) string {
	prefix = normalizedNATSPrefix(prefix)
	token := strings.ToLower(strings.TrimSpace(jobType))
	token = strings.ReplaceAll(token, "_", "-")
	token = strings.Trim(token, ".")
	if token == "" || strings.ContainsAny(token, " \t\r\n*>") {
		token = "unknown"
	}
	return prefix + "." + token
}
