package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestServeHTTPForceClosesStuckHandler asserts serveHTTP returns even when a
// handler outlives the shutdown grace period.
//
// http.Server.Shutdown waits for in-flight handlers but never cancels their
// request context, so a slow handler (upstream lookup, ffprobe tag read) is
// unaffected by it. Without the srv.Close() that follows an expired grace
// period, serveHTTP blocks on <-serveErr for as long as the handler runs and
// Ctrl-C appears to hang.
func TestServeHTTPForceClosesStuckHandler(t *testing.T) {
	// Long enough that a serveHTTP which waits for it fails the test rather than
	// passing slowly.
	const handlerBlock = 30 * time.Second

	entered := make(chan struct{})
	handlerCtxErr := make(chan error, 1)
	srv := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			select {
			case <-r.Context().Done():
				handlerCtxErr <- r.Context().Err()
			case <-time.After(handlerBlock):
				handlerCtxErr <- nil
			}
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// serveHTTP listens on srv.Addr itself, so claim a free port and hand it over.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := probe.Addr().String()
	if cerr := probe.Close(); cerr != nil {
		t.Fatalf("release port: %v", cerr)
	}
	srv.Addr = addr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() {
		served <- serveHTTP(ctx, srv, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	}()

	// A raw connection, so the request stays in-flight while the client waits
	// rather than being torn down by a client-side timeout.
	conn, err := dialWithRetry(t, addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := fmt.Fprint(conn, "GET /slow HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never ran")
	}

	cancel()

	// The grace period is 10s; allow margin for it to expire and Close to land.
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serveHTTP returned error: %v", err)
		}
	case <-time.After(handlerBlock - 5*time.Second):
		t.Fatal("serveHTTP did not return after the shutdown grace period expired")
	}

	// The force-close is what cancels the stuck handler's context.
	select {
	case err := <-handlerCtxErr:
		if err == nil {
			t.Fatal("handler ran to completion; its context was never canceled")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler context was not canceled")
	}
}

// dialWithRetry waits for serveHTTP's listener to come up.
func dialWithRetry(t *testing.T, addr string) (net.Conn, error) {
	t.Helper()
	var lastErr error
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	return nil, lastErr
}
