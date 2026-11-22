package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
)

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	sink := &syncBuffer{}
	log.SetOutput(sink)
	t.Cleanup(func() { log.SetOutput(io.Discard) })
	return sink
}

func newTransportServer(t *testing.T, timeout time.Duration, register func(*http.ServeMux)) (*httptest.Server, *http.Client) {
	t.Helper()
	mux := http.NewServeMux()
	register(mux)
	srv := httptest.NewServer(newHandler(mux, timeout))
	t.Cleanup(srv.Close)
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return srv, client
}

func do(t *testing.T, c *http.Client, method, url string, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body is not error JSON: %q", body)
	}
	return payload.Error.Code
}

func TestBuiltinNotFoundAndMethodNotAllowedAreJSON(t *testing.T) {
	srv, c := newTransportServer(t, time.Second, func(m *http.ServeMux) {
		m.HandleFunc("GET /only-get", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		m.HandleFunc("POST /only-post", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	})

	resp, body := do(t, c, "GET", srv.URL+"/missing", nil)
	if resp.StatusCode != 404 || errorCode(t, body) != "not_found" {
		t.Fatalf("404: %d %q", resp.StatusCode, body)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("404 headers: %v", resp.Header)
	}
	if resp.Header.Get("X-Content-Type-Options") != "" {
		t.Fatalf("stale nosniff header: %v", resp.Header)
	}

	resp, body = do(t, c, "DELETE", srv.URL+"/only-get", nil)
	if resp.StatusCode != 405 || errorCode(t, body) != "method_not_allowed" {
		t.Fatalf("405: %d %q", resp.StatusCode, body)
	}
	allow := resp.Header.Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
		t.Fatalf("Allow = %q", allow)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("405 headers: %v", resp.Header)
	}

	// HEAD semantics: GET routes answer HEAD; POST-only routes give 405
	// with Allow; unknown paths give 404. HEAD bodies stay empty.
	resp, body = do(t, c, "HEAD", srv.URL+"/only-get", nil)
	if resp.StatusCode != 204 || len(body) != 0 {
		t.Fatalf("HEAD on GET route: %d", resp.StatusCode)
	}
	resp, body = do(t, c, "HEAD", srv.URL+"/only-post", nil)
	if resp.StatusCode != 405 || !strings.Contains(resp.Header.Get("Allow"), "POST") || len(body) != 0 {
		t.Fatalf("HEAD on POST route: %d allow=%q body=%q", resp.StatusCode, resp.Header.Get("Allow"), body)
	}
	resp, body = do(t, c, "HEAD", srv.URL+"/missing", nil)
	if resp.StatusCode != 404 || len(body) != 0 {
		t.Fatalf("HEAD 404: %d %q", resp.StatusCode, body)
	}
}

func TestRedirectsAndBusiness404AreUnchanged(t *testing.T) {
	register := func(m *http.ServeMux) {
		m.HandleFunc("GET /dir/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		m.HandleFunc("GET /business", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"partner_not_found","message":"x"}}`))
		})
	}
	srv, c := newTransportServer(t, time.Second, register)
	bareMux := http.NewServeMux()
	register(bareMux)
	bare := httptest.NewServer(bareMux)
	t.Cleanup(bare.Close)

	// Redirect behaviour is whatever the Go mux does natively (the status
	// code differs between Go versions); the wrapper must not change it.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/dir"},
		{"HEAD", "/dir"},
		{"GET", "/a/../dir/"},
		{"GET", "/dir/../dir"},
	} {
		want, wantBody := do(t, c, tc.method, bare.URL+tc.path, nil)
		got, gotBody := do(t, c, tc.method, srv.URL+tc.path, nil)
		if want.StatusCode < 300 || want.StatusCode > 399 || want.Header.Get("Location") == "" {
			t.Fatalf("%s %s: baseline is not a redirect: %d", tc.method, tc.path, want.StatusCode)
		}
		if got.StatusCode != want.StatusCode || got.Header.Get("Location") != want.Header.Get("Location") ||
			got.Header.Get("Content-Type") != want.Header.Get("Content-Type") || !bytes.Equal(gotBody, wantBody) {
			t.Fatalf("%s %s changed: got %d %q %q, want %d %q %q", tc.method, tc.path,
				got.StatusCode, got.Header.Get("Location"), gotBody,
				want.StatusCode, want.Header.Get("Location"), wantBody)
		}
	}

	resp, body := do(t, c, "GET", srv.URL+"/business", nil)
	if resp.StatusCode != 404 || errorCode(t, body) != "partner_not_found" {
		t.Fatalf("business 404 rewritten: %d %q", resp.StatusCode, body)
	}
}

func TestRequestIDIsServerGenerated(t *testing.T) {
	srv, c := newTransportServer(t, time.Second, func(m *http.ServeMux) {
		m.HandleFunc("GET /x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	})
	shape := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := map[string]bool{}
	for _, in := range []string{"", "attacker-controlled", strings.Repeat("A", 500), "<script>"} {
		hdr := map[string]string{}
		if in != "" {
			hdr["X-Request-ID"] = in
		}
		resp, _ := do(t, c, "GET", srv.URL+"/x", hdr)
		id := resp.Header.Get("X-Request-ID")
		if !shape.MatchString(id) || id == in || seen[id] {
			t.Fatalf("request id %q for input %q", id, in)
		}
		seen[id] = true
	}
	resp, _ := do(t, c, "GET", srv.URL+"/missing", map[string]string{"X-Request-ID": "attacker"})
	if !shape.MatchString(resp.Header.Get("X-Request-ID")) {
		t.Fatalf("404 request id: %q", resp.Header.Get("X-Request-ID"))
	}
}

func TestPanicBeforeCommitReturnsGeneric500(t *testing.T) {
	sink := captureLog(t)
	const secret = "s3cret-token-uid-123"
	srv, c := newTransportServer(t, time.Second, func(m *http.ServeMux) {
		m.HandleFunc("GET /boom", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			panic(secret)
		})
	})
	resp, body := do(t, c, "GET", srv.URL+"/boom?token="+secret, map[string]string{"Authorization": "Bearer " + secret})
	if resp.StatusCode != 500 || errorCode(t, body) != "internal_error" {
		t.Fatalf("panic response: %d %q", resp.StatusCode, body)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("headers: %v", resp.Header)
	}
	if strings.Contains(string(body), secret) {
		t.Fatalf("client saw panic value: %q", body)
	}
	logged := sink.String()
	if strings.Contains(logged, secret) || strings.Contains(logged, "token=") {
		t.Fatalf("secret leaked to log: %q", logged)
	}
	if !strings.Contains(logged, "GET /boom") {
		t.Fatalf("log lacks matched pattern: %q", logged)
	}
}

func TestPanicAfterCommitAbortsConnection(t *testing.T) {
	sink := captureLog(t)
	const secret = "post-commit-secret"
	srv, c := newTransportServer(t, time.Second, func(m *http.ServeMux) {
		m.HandleFunc("GET /pdf", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.7 partial"))
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("Flush through Unwrap: %v", err)
			}
			panic(secret)
		})
	})
	resp, err := c.Get(srv.URL + "/pdf")
	if err != nil {
		t.Fatalf("headers should already be committed: %v", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr == nil {
		t.Fatalf("connection was not aborted; body=%q", body)
	}
	if bytes.Contains(body, []byte("internal_error")) || bytes.Contains(body, []byte(secret)) {
		t.Fatalf("JSON/secret appended to committed response: %q", body)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if strings.Contains(sink.String(), secret) {
		t.Fatalf("secret leaked to log: %q", sink.String())
	}
}

func TestPanicAfterFlushWithoutWriteAbortsConnection(t *testing.T) {
	sink := captureLog(t)
	const secret = "flush-only-secret"
	srv, c := newTransportServer(t, time.Second, func(m *http.ServeMux) {
		m.HandleFunc("GET /flush", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			// Flush commits the headers (200) without any prior Write.
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("Flush through Unwrap: %v", err)
			}
			panic(secret)
		})
	})
	resp, err := c.Get(srv.URL + "/flush")
	if err != nil {
		t.Fatalf("headers should already be committed: %v", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want committed 200", resp.StatusCode)
	}
	if readErr == nil {
		t.Fatalf("connection was not aborted; body=%q", body)
	}
	if bytes.Contains(body, []byte("internal_error")) || bytes.Contains(body, []byte(secret)) {
		t.Fatalf("JSON/secret appended to committed response: %q", body)
	}
	if strings.Contains(sink.String(), secret) {
		t.Fatalf("secret leaked to log: %q", sink.String())
	}
}

func TestStreamedBinaryBodyIsUnchanged(t *testing.T) {
	pdf := make([]byte, 1<<20)
	for i := range pdf {
		pdf[i] = byte(i*31 + 7)
	}
	copy(pdf, "%PDF-1.7\n")
	srv, c := newTransportServer(t, time.Second, func(m *http.ServeMux) {
		m.HandleFunc("GET /receipt.pdf", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(pdf)
		})
	})
	resp, body := do(t, c, "GET", srv.URL+"/receipt.pdf", nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/pdf" || !bytes.Equal(body, pdf) {
		t.Fatalf("pdf changed: %d %s len=%d", resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
	}
}

type blockingVerifier struct{}

func (blockingVerifier) VerifyIDToken(ctx context.Context, _ string) (*auth.Token, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRequestDeadlineMapsAuthTimeoutTo504(t *testing.T) {
	var deadlineSet atomic.Bool
	srv, c := newTransportServer(t, 50*time.Millisecond, func(m *http.ServeMux) {
		m.Handle("GET /secure", middleware.RequireAuth(blockingVerifier{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})))
		m.HandleFunc("GET /deadline", func(w http.ResponseWriter, r *http.Request) {
			_, ok := r.Context().Deadline()
			deadlineSet.Store(ok)
			w.WriteHeader(http.StatusNoContent)
		})
	})
	resp, body := do(t, c, "GET", srv.URL+"/secure", map[string]string{"Authorization": "Bearer abc"})
	if resp.StatusCode != http.StatusGatewayTimeout || errorCode(t, body) != "request_timeout" {
		t.Fatalf("timeout: %d %q", resp.StatusCode, body)
	}
	do(t, c, "GET", srv.URL+"/deadline", nil)
	if !deadlineSet.Load() {
		t.Fatal("handler context has no deadline")
	}
	if requestTimeout != 15*time.Second {
		t.Fatalf("requestTimeout = %s", requestTimeout)
	}
}

// plainWriter supports neither Flusher nor Unwrap, like a minimal server.
type plainWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (p *plainWriter) Header() http.Header         { return p.header }
func (p *plainWriter) Write(b []byte) (int, error) { return p.body.Write(b) }
func (p *plainWriter) WriteHeader(code int)        { p.status = code }

func TestUnsupportedFlushDoesNotCommitSoPanicStillGets500(t *testing.T) {
	captureLog(t)
	var flushErr error
	mux := http.NewServeMux()
	mux.HandleFunc("GET /flush", func(w http.ResponseWriter, _ *http.Request) {
		flushErr = http.NewResponseController(w).Flush()
		panic("unsupported-flush-secret")
	})
	pw := &plainWriter{header: http.Header{}}
	newHandler(mux, time.Second).ServeHTTP(pw, httptest.NewRequest("GET", "/flush", nil))
	if !errors.Is(flushErr, http.ErrNotSupported) {
		t.Fatalf("flush error = %v, want ErrNotSupported", flushErr)
	}
	if pw.status != 500 || errorCode(t, pw.body.Bytes()) != "internal_error" {
		t.Fatalf("response: %d %q", pw.status, pw.body.String())
	}
}
