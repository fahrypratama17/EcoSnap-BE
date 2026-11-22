package bootstrap

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

const testWait = 5 * time.Second

func newListener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return l
}

func startServe(ctx context.Context, srv *http.Server, l net.Listener, grace time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, srv, l, grace) }()
	return done
}

func waitErr(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(testWait):
		t.Fatal("timeout menunggu Serve selesai")
		return nil
	}
}

func TestServeDrainsActiveRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})}
	l := newListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startServe(ctx, srv, l, testWait)

	respCh := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + l.Addr().String())
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				err = errors.New("status bukan 200")
			}
		}
		respCh <- err
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		t.Fatalf("Serve kembali sebelum request selesai: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-respCh; err != nil {
		t.Fatalf("request aktif gagal: %v", err)
	}
	if err := waitErr(t, done); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
}

func TestServeRejectsLateConnections(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})}
	l := newListener(t)
	addr := l.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startServe(ctx, srv, l, testWait)

	go func() {
		if resp, err := http.Get("http://" + addr); err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()

	deadline := time.Now().Add(testWait)
	for {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			break
		}
		c.Close()
		if time.Now().After(deadline) {
			t.Fatal("koneksi baru masih diterima setelah shutdown")
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(release)
	if err := waitErr(t, done); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
}

func TestServeForcedCloseCancelsRequestContext(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})}
	l := newListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startServe(ctx, srv, l, 50*time.Millisecond)

	go func() {
		if resp, err := http.Get("http://" + l.Addr().String()); err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()

	err := waitErr(t, done)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, ingin DeadlineExceeded", err)
	}
	select {
	case <-canceled:
	case <-time.After(testWait):
		t.Fatal("context request tidak dibatalkan oleh forced close")
	}
}

func TestServeSurfacesListenerFailure(t *testing.T) {
	srv := &http.Server{Handler: http.NotFoundHandler()}
	l := newListener(t)
	l.Close() // Accept akan gagal dengan error non-ErrServerClosed.

	done := startServe(context.Background(), srv, l, time.Second)
	err := waitErr(t, done)
	if err == nil {
		t.Fatal("ingin error dari listener rusak")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("err tidak boleh ErrServerClosed: %v", err)
	}
}

func TestServePreCancelledContext(t *testing.T) {
	srv := &http.Server{Handler: http.NotFoundHandler()}
	l := newListener(t)
	addr := l.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := waitErr(t, startServe(ctx, srv, l, time.Second)); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("listener masih terbuka setelah Serve kembali")
	}
}
