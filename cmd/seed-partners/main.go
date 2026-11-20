// seed-partners adds one explicitly requested development location. It does not
// create Firebase users, change account roles, or overwrite existing locations.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	uid := flag.String("mitra-uid", "", "UID akun mitra development yang sudah terdaftar")
	demo := flag.Bool("demo", false, "konfirmasi seed lokasi simulasi pada database development")
	flag.Parse()
	if !*demo || *uid == "" {
		log.Fatal("gunakan --demo --mitra-uid <UID>; DATABASE_URL harus menunjuk development")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL wajib diisi")
	}
	pool, err := postgres.NewPool(ctx, url)
	if err != nil {
		log.Fatal("koneksi development gagal")
	}
	err = seed(ctx, pool, *uid)
	pool.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Seed development selesai: empat kategori dan lokasi simulasi Bank Sampah Tunas. Tidak ada password dibuat.")
}

func seed(ctx context.Context, pool *pgxpool.Pool, uid string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var role, status, registration string
	err = tx.QueryRow(ctx, `SELECT a.role::text,a.status::text,r.status FROM public.accounts a JOIN public.partner_registrations r ON r.user_uid=a.firebase_uid WHERE a.firebase_uid=$1 FOR UPDATE OF a,r`, uid).Scan(&role, &status, &registration)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("daftarkan akun mitra development melalui endpoint akun terlebih dahulu")
	}
	if err != nil {
		return err
	}
	if role != "mitra" || status != "active" || (registration != "pending" && registration != "approved") {
		return errors.New("seed memerlukan akun mitra aktif dengan pendaftaran pending/approved")
	}
	categories := []struct{ slug, name string }{{"plastic", "Plastik"}, {"paper", "Kertas"}, {"metal", "Logam"}, {"glass", "Kaca"}}
	for i, c := range categories {
		id := fmt.Sprintf("10000000-0000-4000-8000-%012d", i+1)
		if _, err = tx.Exec(ctx, `INSERT INTO public.waste_categories(id,name,slug) VALUES($1,$2,$3) ON CONFLICT(slug) DO NOTHING`, id, c.name, c.slug); err != nil {
			return err
		}
	}
	const partnerID = "20000000-0000-4000-8000-000000000001"
	_, err = tx.Exec(ctx, `INSERT INTO public.partners(id,mitra_uid,business_name,business_category,description,area,address,latitude,longitude,phone,opening_hours,status)
 VALUES($1,$2,'Bank Sampah Tunas','Bank sampah','Lokasi simulasi development','Kawasan Malang','Alamat simulasi, Malang',-7.9775,112.6304,'','Senin-Sabtu, 08.00-16.00','verified') ON CONFLICT DO NOTHING`, partnerID, uid)
	if err != nil {
		return err
	}
	var owner, locationStatus string
	err = tx.QueryRow(ctx, `SELECT mitra_uid,status::text FROM public.partners WHERE id=$1 FOR UPDATE`, partnerID).Scan(&owner, &locationStatus)
	if err != nil || owner != uid || locationStatus != "verified" {
		return errors.New("lokasi demo atau akun sudah dipakai; seed tidak menimpa lokasi/status yang ada")
	}
	var ownedID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.partners WHERE mitra_uid=$1`, uid).Scan(&ownedID); err != nil {
		return err
	}
	if ownedID != partnerID {
		return errors.New("akun sudah memiliki tempat lain")
	}
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("30000000-0000-4000-8000-%012d", i+1)
		var opens, closes any
		if i != 0 {
			opens = "08:00"
			closes = "16:00"
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.partner_hours(id,partner_id,day_of_week,opens_at,closes_at,is_closed) VALUES($1,$2,$3,$4::text::time,$5::text::time,$6) ON CONFLICT(partner_id,day_of_week) DO NOTHING`, id, partnerID, i, opens, closes, i == 0); err != nil {
			return err
		}
	}
	for i, slug := range []string{"plastic", "paper", "metal"} {
		id := fmt.Sprintf("40000000-0000-4000-8000-%012d", i+1)
		if _, err = tx.Exec(ctx, `INSERT INTO public.partner_accepted_waste(id,partner_id,category_id) SELECT $1::uuid,$2::uuid,id FROM public.waste_categories WHERE slug=$3 ON CONFLICT(partner_id,category_id) DO NOTHING`, id, partnerID, slug); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public.partner_registrations SET status='approved' WHERE user_uid=$1 AND status='pending'`, uid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
