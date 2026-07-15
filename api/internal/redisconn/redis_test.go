package redisconn

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClientPing(t *testing.T) {
	addr, stop := startRedisStub(t, "")
	defer stop()
	client := New(addr, "")
	client.Timeout = time.Second
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("ping redis stub: %v", err)
	}
}

func TestClientPingWithAuth(t *testing.T) {
	addr, stop := startRedisStub(t, "secret")
	defer stop()
	client := New(addr, "secret")
	client.Timeout = time.Second
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("auth ping redis stub: %v", err)
	}
}

func startRedisStub(t *testing.T, password string) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen redis stub: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		if password != "" {
			line, _ := reader.ReadString('\n')
			if strings.TrimSpace(line) != "*2" {
				return
			}
			for i := 0; i < 4; i++ {
				_, _ = reader.ReadString('\n')
			}
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "*1" {
			return
		}
		for i := 0; i < 2; i++ {
			_, _ = reader.ReadString('\n')
		}
		_, _ = conn.Write([]byte("+PONG\r\n"))
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}
