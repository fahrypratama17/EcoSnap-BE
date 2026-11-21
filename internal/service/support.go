package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

// MaxUnresolvedSupportTickets caps tickets in status open or in_progress per
// account so one account cannot flood the owner's queue.
const MaxUnresolvedSupportTickets = 20

const (
	supportSubjectMax = 150
	supportTextMax    = 4000
)

var (
	ErrInvalidSupportTicket   = errors.New("tiket tidak valid: kind problem atau feedback, subject 1-150 karakter, message 1-4000 karakter")
	ErrSupportTicketNotFound  = errors.New("tiket tidak ditemukan")
	ErrSupportTicketConflict  = errors.New("Idempotency-Key sudah dipakai untuk isi tiket lain")
	ErrSupportTicketLimit     = errors.New("terlalu banyak tiket yang belum selesai; tunggu balasan atau penyelesaian tiket sebelumnya")
	ErrInvalidSupportStatus   = errors.New("status tiket harus open, in_progress, atau resolved")
	ErrInvalidSupportReply    = errors.New("balasan wajib 1-4000 karakter dan tidak berisi karakter kontrol")
	ErrInvalidSupportTransit  = errors.New("perpindahan status tiket tidak diizinkan (open->in_progress|resolved, in_progress->resolved, resolved->in_progress)")
	ErrSupportReplyExists     = errors.New("tiket sudah memiliki balasan; gunakan --replace untuk menggantinya (balasan lama hilang)")
	ErrSupportNothingToChange = errors.New("tidak ada perubahan yang diminta")
	supportTransitions        = map[string][]string{"open": {"in_progress", "resolved"}, "in_progress": {"resolved"}, "resolved": {"in_progress"}}
	supportFAQs               = supportFAQContent()
)

func supportFAQContent() []entity.FAQ {
	return []entity.FAQ{
		{ID: "qr-setoran", Category: "setoran", Question: "Bagaimana QR setoran bekerja dan apa risikonya?",
			Answer: "QR setoran adalah token pembawa (bearer): siapa pun yang memegang QR yang masih berlaku dapat mengklaim poin setoran tersebut, bukan hanya pemilik setoran. QR berlaku 10 menit sejak diterbitkan, lalu tidak dapat dipakai lagi. Tunjukkan QR hanya kepada orang yang akan mengklaim dan jangan bagikan tangkapan layarnya.", IsDemo: true},
		{ID: "voucher-kedaluwarsa", Category: "reward", Question: "Berapa lama voucher berlaku dan apakah poin dikembalikan?",
			Answer: "Voucher berlaku 7 hari sejak penukaran. Voucher yang kedaluwarsa tidak dapat dipakai dan poin yang sudah dipakai untuk menukar tidak dikembalikan. Tukar reward hanya saat Anda siap memakainya.", IsDemo: true},
		{ID: "poin-dan-xp", Category: "poin", Question: "Apa beda EcoPoints dan XP?",
			Answer: "EcoPoints adalah saldo dari setoran yang diklaim dan dapat ditukar dengan reward. XP berasal dari klaim misi mingguan, hanya dipakai untuk papan peringkat, tidak masuk saldo, dan tidak dapat ditukar dengan reward.", IsDemo: true},
		{ID: "konten-demo", Category: "umum", Question: "Mengapa ada label demo pada tarif, reward, dan artikel?",
			Answer: "Tarif poin, reward, voucher, dan artikel yang berlabel demo adalah data simulasi untuk pengujian aplikasi. Voucher tidak berhubungan dengan penyedia nyata dan tidak ada uang yang dipindahkan.", IsDemo: true},
		{ID: "hubungi-kami", Category: "bantuan", Question: "Bagaimana melapor masalah atau mengirim masukan?",
			Answer: "Kirim tiket dari aplikasi dengan jenis masalah atau masukan. Tidak ada email, telepon, atau pesan instan yang dapat dihubungi. Balasan dari pengelola tidak dikirim lewat email atau notifikasi; buka daftar tiket Anda di aplikasi untuk melihat status dan balasan. Balasan dapat memerlukan waktu karena ditulis secara manual.", IsDemo: true},
	}
}

// SupportTicketRequest is the validated-on-use client payload.
type SupportTicketRequest struct{ Kind, Subject, Message string }

type NewSupportTicket struct {
	UID, IdempotencyKey, RequestHash, Kind, Subject, Message string
}

type SupportStore interface {
	// ActiveAccount returns ErrAccountNotFound or ErrAccountForbidden unless
	// uid is an active account of any role.
	ActiveAccount(ctx context.Context, uid string) error
	// CreateTicket returns created=false when uid already created a ticket
	// with the same key and request hash. It returns
	// ErrSupportTicketConflict for the same key with different content and
	// ErrSupportTicketLimit at the unresolved-ticket cap.
	CreateTicket(ctx context.Context, in NewSupportTicket) (entity.SupportTicket, bool, error)
	Tickets(ctx context.Context, uid string, status *string, limit int, after *Cursor) ([]entity.SupportTicket, error)
	Ticket(ctx context.Context, uid, id string) (entity.SupportTicket, error)
}

type SupportService struct{ store SupportStore }

func NewSupportService(store SupportStore) *SupportService { return &SupportService{store: store} }

// FAQs returns the reviewed help content to active accounts of any role.
func (s *SupportService) FAQs(ctx context.Context, uid string) ([]entity.FAQ, error) {
	if err := s.store.ActiveAccount(ctx, uid); err != nil {
		return nil, err
	}
	return append([]entity.FAQ(nil), supportFAQs...), nil
}

// cleanSupportText normalises line endings, trims, and checks the length in
// characters (runes). Control characters are rejected, except newline and
// tab when multiline.
func cleanSupportText(s string, max int, multiline bool) (string, bool) {
	if !utf8.ValidString(s) {
		return "", false
	}
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if n := utf8.RuneCountInString(s); n < 1 || n > max {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t')) {
			return "", false
		}
	}
	return s, true
}

func supportRequestHash(kind, subject, message string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + subject + "\x00" + message))
	return hex.EncodeToString(sum[:])
}

// Create stores the ticket once per (account, Idempotency-Key). A retry with
// the same content returns the stored ticket with created=false.
func (s *SupportService) Create(ctx context.Context, uid, key string, in SupportTicketRequest) (entity.SupportTicket, bool, error) {
	if !idempotencyKey.MatchString(key) {
		return entity.SupportTicket{}, false, ErrInvalidIdempotencyKey
	}
	subject, okSubject := cleanSupportText(in.Subject, supportSubjectMax, false)
	message, okMessage := cleanSupportText(in.Message, supportTextMax, true)
	if (in.Kind != "problem" && in.Kind != "feedback") || !okSubject || !okMessage {
		return entity.SupportTicket{}, false, ErrInvalidSupportTicket
	}
	return s.store.CreateTicket(ctx, NewSupportTicket{UID: uid, IdempotencyKey: key, RequestHash: supportRequestHash(in.Kind, subject, message), Kind: in.Kind, Subject: subject, Message: message})
}

func validSupportStatus(s string) bool {
	return s == "open" || s == "in_progress" || s == "resolved"
}

func (s *SupportService) Tickets(ctx context.Context, uid string, q HistoryQuery) (Page[entity.SupportTicket], error) {
	limit, after, err := pageArgs(q)
	if err != nil {
		return Page[entity.SupportTicket]{}, err
	}
	var status *string
	if q.Status != "" {
		if !validSupportStatus(q.Status) {
			return Page[entity.SupportTicket]{}, ErrInvalidHistoryQuery
		}
		status = &q.Status
	}
	items, err := s.store.Tickets(ctx, uid, status, limit+1, after)
	if err != nil {
		return Page[entity.SupportTicket]{}, err
	}
	return trim(items, limit, func(t entity.SupportTicket) (time.Time, string) { return t.CreatedAt, t.ID }), nil
}

// Ticket returns only the caller's ticket; other accounts get not-found.
func (s *SupportService) Ticket(ctx context.Context, uid, id string) (entity.SupportTicket, error) {
	if !partnerID.MatchString(id) {
		return entity.SupportTicket{}, ErrSupportTicketNotFound
	}
	return s.store.Ticket(ctx, uid, strings.ToLower(id))
}

// CheckSupportTransition enforces the owner status workflow. It never
// changes status on its own: nothing acknowledges a ticket automatically.
func CheckSupportTransition(from, to string) error {
	if !validSupportStatus(to) {
		return ErrInvalidSupportStatus
	}
	for _, next := range supportTransitions[from] {
		if next == to {
			return nil
		}
	}
	return ErrInvalidSupportTransit
}

// SupportOwnerUpdate is one owner mutation. Status and Response may be set
// together; the store applies both or neither.
type SupportOwnerUpdate struct {
	ID              string
	Status          *string
	Response        *string
	ReplaceResponse bool
}

type SupportOwnerStore interface {
	// OwnerTickets lists tickets of all accounts, oldest first, optionally
	// narrowed to one ID or status.
	OwnerTickets(ctx context.Context, id, status *string, limit int) ([]entity.SupportTicketOwnerView, error)
	// OwnerUpdate applies the update under a row lock and returns the result.
	OwnerUpdate(ctx context.Context, in SupportOwnerUpdate) (entity.SupportTicketOwnerView, error)
}

type SupportOwnerService struct{ store SupportOwnerStore }

func NewSupportOwnerService(store SupportOwnerStore) *SupportOwnerService {
	return &SupportOwnerService{store: store}
}

func (s *SupportOwnerService) List(ctx context.Context, id, status string, limit int) ([]entity.SupportTicketOwnerView, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidHistoryQuery
	}
	var idp, statusp *string
	if id != "" {
		if !partnerID.MatchString(id) {
			return nil, ErrSupportTicketNotFound
		}
		id = strings.ToLower(id)
		idp = &id
	}
	if status != "" {
		if !validSupportStatus(status) {
			return nil, ErrInvalidSupportStatus
		}
		statusp = &status
	}
	return s.store.OwnerTickets(ctx, idp, statusp, limit)
}

// Update validates input. status and response are both optional but at least
// one is required; a blank response string is rejected, never ignored.
func (s *SupportOwnerService) Update(ctx context.Context, id string, status, response *string, replace bool) (entity.SupportTicketOwnerView, error) {
	if !partnerID.MatchString(id) {
		return entity.SupportTicketOwnerView{}, ErrSupportTicketNotFound
	}
	in := SupportOwnerUpdate{ID: strings.ToLower(id), ReplaceResponse: replace}
	if status == nil && response == nil {
		return entity.SupportTicketOwnerView{}, ErrSupportNothingToChange
	}
	if status != nil {
		if !validSupportStatus(*status) {
			return entity.SupportTicketOwnerView{}, ErrInvalidSupportStatus
		}
		in.Status = status
	}
	if response != nil {
		text, ok := cleanSupportText(*response, supportTextMax, true)
		if !ok {
			return entity.SupportTicketOwnerView{}, ErrInvalidSupportReply
		}
		in.Response = &text
	}
	return s.store.OwnerUpdate(ctx, in)
}
