// verify-partner is the manual mitra approval tool while there is no admin
// role (D-21). Only the repo owner runs it, against the intended database.
//
//	approve    --mitra-uid U --location loc.json  create the verified location
//	set-status --mitra-uid U --status S            verified|suspended|rejected
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type location struct {
	BusinessName  string   `json:"business_name"`
	Description   string   `json:"description"`
	Area          string   `json:"area"`
	Address       string   `json:"address"`
	Latitude      float64  `json:"latitude"`
	Longitude     float64  `json:"longitude"`
	Phone         string   `json:"phone"`
	OpeningHours  string   `json:"opening_hours"`
	AcceptedWaste []string `json:"accepted_waste"`
}

var slug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("gunakan: verify-partner approve|set-status --confirm ...; DATABASE_URL wajib")
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	uid := fs.String("mitra-uid", "", "UID akun mitra")
	file := fs.String("location", "", "JSON lokasi (approve)")
	status := fs.String("status", "", "verified|suspended|rejected (set-status)")
	confirm := fs.Bool("confirm", false, "konfirmasi perubahan status mitra")
	_ = fs.Parse(os.Args[2:])
	if !*confirm || *uid == "" || fs.NArg() != 0 {
		log.Fatal("--confirm dan --mitra-uid wajib")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL wajib diisi")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var run func(context.Context, pgx.Tx) (string, error)
	switch os.Args[1] {
	case "approve":
		loc, err := readLocation(*file)
		if err != nil {
			log.Fatal(err)
		}
		run = func(ctx context.Context, tx pgx.Tx) (string, error) { return approve(ctx, tx, *uid, loc) }
	case "set-status":
		if *status != "verified" && *status != "suspended" && *status != "rejected" {
			log.Fatal("--status harus verified, suspended, atau rejected")
		}
		run = func(ctx context.Context, tx pgx.Tx) (string, error) { return setStatus(ctx, tx, *uid, *status) }
	default:
		log.Fatal("perintah harus approve atau set-status")
	}
	pool, err := postgres.NewPool(ctx, url)
	if err != nil {
		log.Fatal("koneksi database gagal")
	}
	msg, err := inTx(ctx, pool, run)
	pool.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(msg)
}

func inTx(ctx context.Context, pool *pgxpool.Pool, run func(context.Context, pgx.Tx) (string, error)) (string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	msg, err := run(ctx, tx)
	if err != nil {
		return "", err
	}
	return msg, tx.Commit(ctx)
}

func readLocation(path string) (location, error) {
	var loc location
	if path == "" {
		return loc, errors.New("--location wajib untuk approve")
	}
	f, err := os.Open(path)
	if err != nil {
		return loc, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 64*1024))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&loc); err != nil {
		return loc, fmt.Errorf("JSON lokasi tidak valid: %w", err)
	}
	loc.Area, loc.Address = strings.TrimSpace(loc.Area), strings.TrimSpace(loc.Address)
	if loc.Area == "" || len(loc.Area) > 150 || loc.Address == "" || loc.Latitude < -90 || loc.Latitude > 90 || loc.Longitude < -180 || loc.Longitude > 180 || len(loc.Phone) > 32 || len(loc.OpeningHours) > 200 || len(loc.BusinessName) > 150 {
		return loc, errors.New("area, address, latitude, longitude wajib dan dalam batas")
	}
	if len(loc.AcceptedWaste) < 1 || len(loc.AcceptedWaste) > 50 {
		return loc, errors.New("accepted_waste wajib berisi 1-50 slug kategori")
	}
	for _, s := range loc.AcceptedWaste {
		if !slug.MatchString(s) {
			return loc, fmt.Errorf("slug kategori tidak valid: %q", s)
		}
	}
	return loc, nil
}

// lockMitra locks the account and registration so status checks hold until
// commit.
func lockMitra(ctx context.Context, tx pgx.Tx, uid string) (registration, businessName, category string, err error) {
	var role, status string
	err = tx.QueryRow(ctx, `SELECT a.role::text,a.status::text,r.status,r.business_name,r.business_category FROM public.accounts a JOIN public.partner_registrations r ON r.user_uid=a.firebase_uid WHERE a.firebase_uid=$1 FOR UPDATE OF a,r`, uid).
		Scan(&role, &status, &registration, &businessName, &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", errors.New("akun belum mendaftar sebagai mitra")
	}
	if err == nil && (role != "mitra" || status != "active") {
		err = errors.New("akun harus role mitra dan active")
	}
	return registration, businessName, category, err
}

// approve creates the single location of the account (D-14) as verified. It
// never overwrites an existing location; use set-status for those.
func approve(ctx context.Context, tx pgx.Tx, uid string, loc location) (string, error) {
	registration, name, category, err := lockMitra(ctx, tx, uid)
	if err != nil {
		return "", err
	}
	if registration == "rejected" {
		return "", errors.New("pendaftaran sudah ditolak")
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.partners WHERE mitra_uid=$1)`, uid).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("akun sudah memiliki lokasi; gunakan set-status")
	}
	if loc.BusinessName != "" {
		name = loc.BusinessName
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO public.partners(id,mitra_uid,business_name,business_category,description,area,address,latitude,longitude,phone,opening_hours,status)
 VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'verified') RETURNING id::text`,
		uid, name, category, loc.Description, loc.Area, loc.Address, loc.Latitude, loc.Longitude, loc.Phone, loc.OpeningHours).Scan(&id)
	if err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO public.partner_accepted_waste(id,partner_id,category_id)
 SELECT gen_random_uuid(),$1::uuid,c.id FROM public.waste_categories c WHERE c.slug=ANY($2::text[])`, id, loc.AcceptedWaste)
	if err != nil {
		return "", err
	}
	if int(tag.RowsAffected()) != len(loc.AcceptedWaste) {
		return "", errors.New("ada slug kategori yang belum ada atau ganda; jalankan seed kategori dahulu")
	}
	if _, err = tx.Exec(ctx, `UPDATE public.partner_registrations SET status='approved' WHERE user_uid=$1`, uid); err != nil {
		return "", err
	}
	return fmt.Sprintf("Mitra %s disetujui; lokasi %s verified.", uid, id), nil
}

func setStatus(ctx context.Context, tx pgx.Tx, uid, status string) (string, error) {
	if _, _, _, err := lockMitra(ctx, tx, uid); err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, `UPDATE public.partners SET status=$2::public.partner_status WHERE mitra_uid=$1`, uid, status)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 && status != "rejected" {
		return "", errors.New("akun belum memiliki lokasi; gunakan approve")
	}
	registration := map[string]string{"verified": "approved", "suspended": "approved", "rejected": "rejected"}[status]
	if _, err = tx.Exec(ctx, `UPDATE public.partner_registrations SET status=$2 WHERE user_uid=$1`, uid, registration); err != nil {
		return "", err
	}
	return fmt.Sprintf("Status mitra %s: %s.", uid, status), nil
}
