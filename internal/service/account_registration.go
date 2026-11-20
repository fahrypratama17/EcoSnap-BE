package service

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidAccount   = errors.New("data pendaftaran tidak valid")
	ErrAccountNotFound  = errors.New("profil akun belum dibuat")
	ErrAccountForbidden = errors.New("akun tidak aktif")
	ErrAccountConflict  = errors.New("data pendaftaran berbeda dari profil yang sudah ada")
)

type AccountRegistration struct {
	Role             string `json:"role"`
	FullName         string `json:"full_name"`
	Phone            string `json:"phone"`
	BusinessName     string `json:"business_name"`
	BusinessCategory string `json:"business_category"`
}

type AccountStore interface {
	UpdateProfile(context.Context, string, AccountProfileUpdate) (entity.AccountProfile, error)
	Register(context.Context, string, string, AccountRegistration) (entity.AccountProfile, bool, error)
	Profile(context.Context, string) (entity.AccountProfile, error)
}
type AccountService struct{ store AccountStore }

func NewAccountService(store AccountStore) *AccountService { return &AccountService{store: store} }

func (s *AccountService) Register(ctx context.Context, uid, email string, in AccountRegistration) (entity.AccountProfile, bool, error) {
	in.Role = strings.TrimSpace(in.Role)
	in.FullName = strings.TrimSpace(in.FullName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.BusinessCategory = strings.TrimSpace(in.BusinessCategory)
	valid := uid != "" && len(uid) <= 128 && len(email) <= 255 && utf8.ValidString(in.FullName) && utf8.RuneCountInString(in.FullName) > 0 && utf8.RuneCountInString(in.FullName) <= 150 && validPhone(in.Phone)
	valid = valid && (in.Role == "user" || in.Role == "mitra")
	if in.Role == "mitra" {
		valid = valid && utf8.ValidString(in.BusinessName) && utf8.ValidString(in.BusinessCategory) && utf8.RuneCountInString(in.BusinessName) > 0 && utf8.RuneCountInString(in.BusinessName) <= 150 && utf8.RuneCountInString(in.BusinessCategory) > 0 && utf8.RuneCountInString(in.BusinessCategory) <= 80
	} else {
		valid = valid && in.BusinessName == "" && in.BusinessCategory == ""
	}
	if !valid {
		return entity.AccountProfile{}, false, ErrInvalidAccount
	}
	profile, created, err := s.store.Register(ctx, uid, email, in)
	if err != nil {
		return entity.AccountProfile{}, false, err
	}
	if profile.Status != "active" {
		return entity.AccountProfile{}, false, ErrAccountForbidden
	}
	return profile, created, nil
}
func validPhone(phone string) bool {
	digits := strings.TrimPrefix(phone, "+")
	if len(digits) < 9 || len(digits) > 15 {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
