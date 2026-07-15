package redisconn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

type Client struct {
	Addr     string
	Password string
	Timeout  time.Duration
}

func New(addr string, password string) Client {
	return Client{Addr: strings.TrimSpace(addr), Password: password, Timeout: time.Second}
}

func (c Client) Ping(ctx context.Context) error {
	if c.Addr == "" {
		return errors.New("redis address is not configured")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = time.Second
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(timeout)
	}
	_ = conn.SetDeadline(deadline)
	reader := bufio.NewReader(conn)
	if c.Password != "" {
		if err := writeCommand(conn, "AUTH", c.Password); err != nil {
			return err
		}
		if err := readSimpleOK(reader); err != nil {
			return fmt.Errorf("redis auth failed: %w", err)
		}
	}
	if err := writeCommand(conn, "PING"); err != nil {
		return err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "+PONG" {
		return fmt.Errorf("unexpected redis ping response: %s", strings.TrimSpace(line))
	}
	return nil
}

func writeCommand(conn net.Conn, parts ...string) error {
	if _, err := fmt.Fprintf(conn, "*%d\r\n", len(parts)); err != nil {
		return err
	}
	for _, part := range parts {
		if _, err := fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(part), part); err != nil {
			return err
		}
	}
	return nil
}

func readSimpleOK(reader *bufio.Reader) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(line)
	if trimmed != "+OK" {
		return errors.New(trimmed)
	}
	return nil
}
