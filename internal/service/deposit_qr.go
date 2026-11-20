package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

// QRTTL is D-05/D-06: ten minutes from (re)issuance, judged by server time.
const QRTTL = 10 * time.Minute

// QRBearerWarning is shown by Android whenever a QR is displayed (D-18).
const QRBearerWarning = "QR ini dapat diklaim oleh siapa pun yang memindainya pertama kali. Tunjukkan hanya kepada penyetor dan jangan bagikan foto atau tangkapan layarnya."

var (
	ErrQRInvalid             = errors.New("QR tidak dikenal")
	ErrQRRevoked             = errors.New("QR sudah diganti dengan QR baru")
	ErrQRExpired             = errors.New("QR sudah kedaluwarsa; minta mitra menerbitkan ulang")
	ErrDepositAlreadyClaimed = errors.New("setoran sudah diklaim")
	ErrClaimForbidden        = errors.New("hanya akun pengguna yang dapat mengklaim setoran")
)

// 32 random bytes in unpadded base64url.
var qrToken = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

type QRRecord struct{ IssuedAt, ExpiresAt time.Time }
type ClaimPreviewRecord struct {
	Deposit     DepositRecord
	PartnerName string
	ExpiresAt   time.Time
}
type ClaimRecord struct {
	DepositID, PartnerID string
	Points, Balance      int64
	ClaimedAt            time.Time
}
type DepositQRStore interface {
	IssueQR(ctx context.Context, uid, depositID, tokenHash string, ttl time.Duration) (QRRecord, error)
	PreviewQR(ctx context.Context, uid, tokenHash string) (ClaimPreviewRecord, error)
	// ClaimQR returns created=false when uid already claimed this deposit.
	ClaimQR(ctx context.Context, uid, tokenHash string) (ClaimRecord, bool, error)
}

type DepositQRService struct{ store DepositQRStore }

func NewDepositQRService(store DepositQRStore) *DepositQRService {
	return &DepositQRService{store: store}
}

func hashQRToken(token string) (string, error) {
	if !qrToken.MatchString(token) {
		return "", ErrQRInvalid
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), nil
}

// Issue creates or replaces the deposit's QR. Previous tokens stop working.
func (s *DepositQRService) Issue(ctx context.Context, uid, depositID string) (entity.DepositQR, error) {
	if !partnerID.MatchString(depositID) {
		return entity.DepositQR{}, ErrDepositNotFound
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return entity.DepositQR{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash, _ := hashQRToken(token)
	id := strings.ToLower(depositID)
	rec, err := s.store.IssueQR(ctx, uid, id, hash, QRTTL)
	if err != nil {
		return entity.DepositQR{}, err
	}
	return entity.DepositQR{DepositID: id, Token: token, IssuedAt: rec.IssuedAt, ExpiresAt: rec.ExpiresAt, Warning: QRBearerWarning}, nil
}

func (s *DepositQRService) Preview(ctx context.Context, uid, token string) (entity.DepositClaimPreview, error) {
	hash, err := hashQRToken(token)
	if err != nil {
		return entity.DepositClaimPreview{}, err
	}
	rec, err := s.store.PreviewQR(ctx, uid, hash)
	if err != nil {
		return entity.DepositClaimPreview{}, err
	}
	d := depositView(rec.Deposit)
	return entity.DepositClaimPreview{DepositID: d.ID, PartnerID: d.PartnerID, PartnerName: rec.PartnerName, TotalWeightKG: d.TotalWeightKG, TotalPoints: d.TotalPoints, UsesDemoRates: d.UsesDemoRates, ExpiresAt: rec.ExpiresAt, Items: d.Items}, nil
}

func (s *DepositQRService) Claim(ctx context.Context, uid, token string) (entity.DepositClaim, bool, error) {
	hash, err := hashQRToken(token)
	if err != nil {
		return entity.DepositClaim{}, false, err
	}
	rec, created, err := s.store.ClaimQR(ctx, uid, hash)
	if err != nil {
		return entity.DepositClaim{}, false, err
	}
	return entity.DepositClaim{DepositID: rec.DepositID, PartnerID: rec.PartnerID, Points: rec.Points, ClaimedAt: rec.ClaimedAt, WalletBalance: rec.Balance}, created, nil
}
