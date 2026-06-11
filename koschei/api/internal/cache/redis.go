package cache

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Redis struct {
	addr, password string
	db             int
	useTLS         bool
	timeout        time.Duration
}

func NewRedis(rawURL string, useTLS bool) (*Redis, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("redis url required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	db := 0
	if p := strings.Trim(u.Path, "/"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			db = n
		}
	}
	password, _ := u.User.Password()
	return &Redis{addr: u.Host, password: password, db: db, useTLS: useTLS || u.Scheme == "rediss", timeout: 3 * time.Second}, nil
}

func (r *Redis) dial(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	d := net.Dialer{Timeout: r.timeout}
	conn, err := d.DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return nil, nil, err
	}
	if r.useTLS {
		conn = tls.Client(conn, &tls.Config{ServerName: strings.Split(r.addr, ":")[0], MinVersion: tls.VersionTLS12})
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(r.timeout))
	}
	br := bufio.NewReader(conn)
	if r.password != "" {
		if _, err := r.command(conn, br, "AUTH", r.password); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}
	if r.db > 0 {
		if _, err := r.command(conn, br, "SELECT", strconv.Itoa(r.db)); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}
	return conn, br, nil
}
func (r *Redis) command(conn net.Conn, br *bufio.Reader, args ...string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return "", err
	}
	line, err := br.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch line[0] {
	case '+':
		return line[1:], nil
	case '-':
		return "", errors.New(line[1:])
	case ':':
		return line[1:], nil
	case '$':
		n, _ := strconv.Atoi(line[1:])
		if n < 0 {
			return "", ErrMiss
		}
		buf := make([]byte, n+2)
		if _, err := br.Read(buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	default:
		return "", fmt.Errorf("unexpected redis response: %q", line)
	}
}
func (r *Redis) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	conn, br, err := r.dial(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	s, err := r.command(conn, br, "GET", key)
	if errors.Is(err, ErrMiss) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, UnmarshalJSON([]byte(s), dst)
}
func (r *Redis) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := MarshalJSON(value)
	if err != nil {
		return err
	}
	conn, br, err := r.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if ttl > 0 {
		_, err = r.command(conn, br, "SET", key, string(data), "EX", strconv.Itoa(int(ttl.Seconds())))
		return err
	}
	_, err = r.command(conn, br, "SET", key, string(data))
	return err
}
func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	conn, br, err := r.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = r.command(conn, br, append([]string{"DEL"}, keys...)...)
	return err
}
func (r *Redis) Ping(ctx context.Context) error {
	conn, br, err := r.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = r.command(conn, br, "PING")
	return err
}
func (r *Redis) Close() error { return nil }
