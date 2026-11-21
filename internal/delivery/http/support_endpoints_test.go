package httpdelivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

const supportOtherTicket = "a0000000-0000-4000-8000-000000000001"

// supportMemory replays by key and hash; the hash is the server's canonical one.
type supportMemory struct {
	accountErr error
	byKey      map[string]string
	tickets    []entity.SupportTicket
	limit      int
}

func (m *supportMemory) ActiveAccount(context.Context, string) error { return m.accountErr }
func (m *supportMemory) CreateTicket(_ context.Context, in service.NewSupportTicket) (entity.SupportTicket, bool, error) {
	if m.accountErr != nil {
		return entity.SupportTicket{}, false, m.accountErr
	}
	if hash, ok := m.byKey[in.IdempotencyKey]; ok {
		if hash != in.RequestHash {
			return entity.SupportTicket{}, false, service.ErrSupportTicketConflict
		}
		return m.tickets[0], false, nil
	}
	if m.limit > 0 && len(m.tickets) >= m.limit {
		return entity.SupportTicket{}, false, service.ErrSupportTicketLimit
	}
	t := entity.SupportTicket{ID: "b0000000-0000-4000-8000-000000000001", Kind: in.Kind, Subject: in.Subject, Message: in.Message, Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.byKey[in.IdempotencyKey] = in.RequestHash
	m.tickets = append(m.tickets, t)
	return t, true, nil
}
func (m *supportMemory) Tickets(_ context.Context, _ string, _ *string, _ int, _ *service.Cursor) ([]entity.SupportTicket, error) {
	return m.tickets, nil
}
func (m *supportMemory) Ticket(_ context.Context, _, id string) (entity.SupportTicket, error) {
	for _, t := range m.tickets {
		if t.ID == id {
			return t, nil
		}
	}
	return entity.SupportTicket{}, service.ErrSupportTicketNotFound
}

func supportRouter(store *supportMemory) http.Handler {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "u"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterSupportRoutes(mux, verifier, handler.NewSupportHandler(service.NewSupportService(store)))
	return mux
}

func supportPost(router http.Handler, key, contentType, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/me/support-tickets", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", contentType)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestSupportFAQHTTP(t *testing.T) {
	router := supportRouter(&supportMemory{byKey: map[string]string{}})
	w := accountRequest(router, "GET", "/api/v1/help/faqs", "")
	if w.Code != 200 {
		t.Fatalf("faq: %d %s", w.Code, w.Body)
	}
	var out struct{ Data []entity.FAQ }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Data) < 4 {
		t.Fatalf("faq body: %v %s", err, w.Body)
	}
	for _, f := range out.Data {
		if !f.IsDemo || f.Question == "" || f.Answer == "" {
			t.Fatalf("faq item: %+v", f)
		}
	}
	for _, want := range []string{"10 menit", "7 hari", "tidak dikembalikan", "XP"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("faq missing %q", want)
		}
	}
	if w := accountRequest(router, "GET", "/api/v1/help/faqs?x=1", ""); w.Code != 400 {
		t.Fatalf("query: %d", w.Code)
	}
	for _, tt := range []struct {
		err  error
		want int
	}{{service.ErrAccountForbidden, 403}, {service.ErrAccountNotFound, 404}} {
		if w := accountRequest(supportRouter(&supportMemory{accountErr: tt.err, byKey: map[string]string{}}), "GET", "/api/v1/help/faqs", ""); w.Code != tt.want {
			t.Fatalf("inactive %v: %d", tt.err, w.Code)
		}
	}
}

func TestSupportTicketHTTP(t *testing.T) {
	store := &supportMemory{byKey: map[string]string{}}
	router := supportRouter(store)
	const body = `{"kind":"problem","subject":"  QR gagal  ","message":"Tidak bisa dipindai\r\ndi toko"}`
	w := supportPost(router, "ticket-0001", "application/json", body)
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"subject":"QR gagal"`) || !strings.Contains(w.Body.String(), `"staff_response":null`) {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if w := supportPost(router, "ticket-0001", "application/json", `{"kind":"problem","subject":"QR gagal","message":"Tidak bisa dipindai\ndi toko"}`); w.Code != 200 {
		t.Fatalf("replay with equivalent content: %d %s", w.Code, w.Body)
	}
	if w := supportPost(router, "ticket-0001", "application/json", `{"kind":"feedback","subject":"QR gagal","message":"lain"}`); w.Code != 409 || !strings.Contains(w.Body.String(), "idempotency_conflict") {
		t.Fatalf("conflict: %d %s", w.Code, w.Body)
	}
	long := strings.Repeat("あ", 4000)
	for _, tt := range []struct {
		name, key, ct, body string
		want                int
		code                string
	}{
		{"missing key", "", "application/json", body, 400, "invalid_idempotency_key"},
		{"short key", "short", "application/json", body, 400, "invalid_idempotency_key"},
		{"wrong media", "ticket-0002", "text/plain", body, 415, ""},
		{"unknown field", "ticket-0002", "application/json", `{"kind":"problem","subject":"a","message":"b","contact_email":"a@b.c"}`, 400, "invalid_request"},
		{"status field not accepted", "ticket-0002", "application/json", `{"kind":"problem","subject":"a","message":"b","status":"resolved"}`, 400, "invalid_request"},
		{"trailing json", "ticket-0002", "application/json", `{"kind":"problem","subject":"a","message":"b"}{}`, 400, "invalid_request"},
		{"bad kind", "ticket-0002", "application/json", `{"kind":"bug","subject":"a","message":"b"}`, 400, "invalid_ticket"},
		{"blank subject", "ticket-0002", "application/json", `{"kind":"problem","subject":"   ","message":"b"}`, 400, "invalid_ticket"},
		{"blank message", "ticket-0002", "application/json", `{"kind":"problem","subject":"a","message":" \n "}`, 400, "invalid_ticket"},
		{"subject 151", "ticket-0002", "application/json", `{"kind":"problem","subject":"` + strings.Repeat("a", 151) + `","message":"b"}`, 400, "invalid_ticket"},
		{"message 4001", "ticket-0002", "application/json", `{"kind":"problem","subject":"a","message":"` + strings.Repeat("a", 4001) + `"}`, 400, "invalid_ticket"},
		{"control character", "ticket-0002", "application/json", `{"kind":"problem","subject":"a\u0000","message":"b"}`, 400, "invalid_ticket"},
		{"4000 multibyte characters accepted", "ticket-0003", "application/json", `{"kind":"feedback","subject":"a","message":"` + long + `"}`, 201, ""},
		{"too large", "ticket-0004", "application/json", `{"kind":"problem","subject":"a","message":"` + strings.Repeat("a", 70000) + `"}`, 413, "body_too_large"},
	} {
		if w := supportPost(router, tt.key, tt.ct, tt.body); w.Code != tt.want || !strings.Contains(w.Body.String(), tt.code) {
			t.Fatalf("%s: %d %s", tt.name, w.Code, w.Body.String()[:min(w.Body.Len(), 200)])
		}
	}
	if w := accountRequest(router, "POST", "/api/v1/me/support-tickets?x=1", body); w.Code != 400 {
		t.Fatalf("query on create: %d", w.Code)
	}
	for path, want := range map[string]int{
		"/api/v1/me/support-tickets":                                          200,
		"/api/v1/me/support-tickets?limit=100&status=open":                    200,
		"/api/v1/me/support-tickets?limit=101":                                400,
		"/api/v1/me/support-tickets?status=closed":                            400,
		"/api/v1/me/support-tickets?uid=x":                                    400,
		"/api/v1/me/support-tickets/b0000000-0000-4000-8000-000000000001":     200,
		"/api/v1/me/support-tickets/" + supportOtherTicket:                    404,
		"/api/v1/me/support-tickets/not-a-uuid":                               404,
		"/api/v1/me/support-tickets/b0000000-0000-4000-8000-000000000001?a=1": 400,
	} {
		if w := accountRequest(router, "GET", path, ""); w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, method := range []string{"PATCH", "PUT", "DELETE"} {
		if w := accountRequest(router, method, "/api/v1/me/support-tickets/b0000000-0000-4000-8000-000000000001", `{"status":"resolved"}`); w.Code != 405 && w.Code != 404 {
			t.Fatalf("%s must not be routed: %d", method, w.Code)
		}
	}
}

func TestSupportTicketLimitHTTP(t *testing.T) {
	store := &supportMemory{byKey: map[string]string{}, limit: 1}
	router := supportRouter(store)
	const body = `{"kind":"feedback","subject":"a","message":"b"}`
	if w := supportPost(router, "ticket-0001", "application/json", body); w.Code != 201 {
		t.Fatalf("first: %d", w.Code)
	}
	if w := supportPost(router, "ticket-0002", "application/json", body); w.Code != 429 || !strings.Contains(w.Body.String(), "ticket_limit_reached") {
		t.Fatalf("cap: %d %s", w.Code, w.Body)
	}
	if w := supportPost(router, "ticket-0001", "application/json", body); w.Code != 200 {
		t.Fatalf("replay at cap: %d", w.Code)
	}
}
