package jobs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

type NoopQueue struct{}

func (NoopQueue) Publish(Job) error { return nil }
func (NoopQueue) Close() error      { return nil }

type NATSQueue struct {
	url    string
	prefix string
}

func NewNATSQueue(rawURL, prefix string) *NATSQueue {
	if prefix == "" {
		prefix = "koschei.web3"
	}
	return &NATSQueue{url: rawURL, prefix: prefix}
}
func (q *NATSQueue) Publish(job Job) error {
	if strings.TrimSpace(q.url) == "" {
		return nil
	}
	u, err := url.Parse(q.url)
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp", u.Host, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	br := bufio.NewReader(conn)
	_, _ = br.ReadString('\n')
	subject := q.prefix + "." + strings.ReplaceAll(job.Type, "_", "-")
	payload, _ := json.Marshal(job)
	_, err = fmt.Fprintf(conn, "PUB %s %d\r\n%s\r\n", subject, len(payload), payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(conn, "PING\r\n")
	if err != nil {
		return err
	}
	_, _ = br.ReadString('\n')
	return nil
}
func (q *NATSQueue) Close() error { return nil }
