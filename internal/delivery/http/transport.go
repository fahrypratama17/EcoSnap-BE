package httpdelivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
)

// requestTimeout bounds the whole request context, including Firebase token
// verification. The deadline is cooperative: it only cancels the context.
// Downstream code that ignores the context is neither interrupted nor has its
// response replaced; the server write timeout remains the hard stop. A client
// disconnect cancels the same context and the connection is simply dropped.
const requestTimeout = 15 * time.Second

// RequestIDHeader carries the server-generated request identifier.
const RequestIDHeader = "X-Request-ID"

// NewHandler wraps the fully registered mux with request IDs, the request
// deadline, panic containment, request logging and JSON bodies for the mux's
// built-in 404/405 responses. Call it once after every route is registered.
func NewHandler(mux *http.ServeMux) http.Handler {
	return newHandler(mux, requestTimeout)
}

func newHandler(mux *http.ServeMux, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		// Incoming X-Request-ID is untrusted and deliberately ignored.
		w.Header().Set(RequestIDHeader, id)

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		req := r.WithContext(ctx)

		rw := &responseWriter{ResponseWriter: w, req: req}
		defer func() {
			rec := recover()
			if rec == nil {
				logRequest(id, req, rw.status, start)
				return
			}
			if rec == http.ErrAbortHandler {
				logRequest(id, req, rw.status, start)
				panic(rec)
			}
			// The panic value may hold secrets: never log or send it.
			if rw.committed {
				log.Printf("http panic after response commit; aborting connection request_id=%s method=%s pattern=%q", id, req.Method, patternOf(req))
				logRequest(id, req, rw.status, start)
				panic(http.ErrAbortHandler)
			}
			log.Printf("http panic recovered request_id=%s method=%s pattern=%q", id, req.Method, patternOf(req))
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Del("Content-Length")
			rw.status = http.StatusInternalServerError
			response.Error(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server.")
			logRequest(id, req, rw.status, start)
		}()

		mux.ServeHTTP(rw, req)
	})
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // never fails since Go 1.24
	return hex.EncodeToString(b[:])
}

func patternOf(r *http.Request) string {
	if r.Pattern == "" {
		return "unmatched"
	}
	return r.Pattern
}

func logRequest(id string, r *http.Request, status int, start time.Time) {
	if status == 0 {
		status = http.StatusOK
	}
	log.Printf("http request request_id=%s method=%s pattern=%q status=%d duration=%s", id, r.Method, patternOf(r), status, time.Since(start))
}

// responseWriter tracks whether the response is committed and swaps the mux's
// plain-text 404/405 for the API's JSON error. Only unmatched requests
// (empty Pattern) are rewritten, so handler-authored 404 bodies, redirects and
// streamed bodies such as PDFs pass through untouched and unbuffered.
// It deliberately implements no Flusher/Hijacker; http.ResponseController
// reaches the underlying writer through Unwrap.
type responseWriter struct {
	http.ResponseWriter
	req       *http.Request
	status    int
	committed bool
	discard   bool // drop the mux's plain-text body after replacing it
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// FlushError lets http.ResponseController flush through this wrapper while
// recording the commit: flushing sends headers (implicit 200) even before any
// Write. An unsupported flush sends nothing and must not mark the response
// committed. It is not http.Flusher, so no capability is faked.
func (w *responseWriter) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if !errors.Is(err, http.ErrNotSupported) && !w.committed {
		w.committed = true
		w.status = http.StatusOK
	}
	return err
}

func (w *responseWriter) WriteHeader(code int) {
	if w.committed {
		return
	}
	if code >= 200 || code == http.StatusSwitchingProtocols {
		w.committed = true
	}
	w.status = code
	if w.req.Pattern == "" && (code == http.StatusNotFound || code == http.StatusMethodNotAllowed) {
		h := w.Header()
		h.Del("X-Content-Type-Options")
		h.Del("Content-Length")
		h.Set("Cache-Control", "no-store")
		w.discard = true
		if code == http.StatusNotFound {
			response.Error(w.ResponseWriter, code, "not_found", "Endpoint tidak ditemukan.")
		} else {
			response.Error(w.ResponseWriter, code, "method_not_allowed", "Metode HTTP tidak diizinkan untuk endpoint ini.")
		}
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	if w.discard {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}
