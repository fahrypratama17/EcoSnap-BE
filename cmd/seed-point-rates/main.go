// seed-point-rates inserts explicit, versioned demo tariffs without overwriting
// existing rates. Run only against the intended development database.
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
	"strconv"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type demoRate struct {
	ID           string    `json:"id"`
	CategorySlug string    `json:"category_slug"`
	PointsPerKG  string    `json:"points_per_kg"`
	ActiveFrom   time.Time `json:"active_from"`
}

func main() {
	demo := flag.Bool("demo", false, "konfirmasi tarif simulasi pada development")
	file := flag.String("file", "data/seeders/dev/point_rates.json", "file tarif demo")
	flag.Parse()
	if !*demo || flag.NArg() != 0 {
		log.Fatal("gunakan --demo [--file <JSON>]; DATABASE_URL harus menunjuk development")
	}
	rates, err := readRates(*file)
	if err != nil {
		log.Fatal(err)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL wajib diisi")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, url)
	if err != nil {
		log.Fatal("koneksi development gagal")
	}
	err = seed(ctx, pool, rates)
	pool.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Seed tarif demo selesai: %d versi; tarif lama tidak ditimpa.", len(rates))
}

func readRates(path string) ([]demoRate, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 64*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 64*1024 {
		return nil, errors.New("file maksimal 64 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var rates []demoRate
	if err = decoder.Decode(&rates); err != nil {
		return nil, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("file harus berisi satu array JSON")
	}
	if len(rates) < 1 || len(rates) > 50 {
		return nil, errors.New("isi 1-50 tarif")
	}
	uuid := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	decimal := regexp.MustCompile(`^[0-9]{1,7}(\.[0-9]{1,2})?$`)
	slug := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	ids := map[string]bool{}
	versions := map[string]bool{}
	for i := range rates {
		r := &rates[i]
		r.ID = strings.ToLower(r.ID)
		if !uuid.MatchString(r.ID) || ids[r.ID] || !slug.MatchString(r.CategorySlug) || len(r.CategorySlug) > 100 || !decimal.MatchString(r.PointsPerKG) || r.ActiveFrom.IsZero() {
			return nil, fmt.Errorf("tarif ke-%d invalid/duplikat", i+1)
		}
		parts := strings.SplitN(r.PointsPerKG, ".", 2)
		whole, _ := strconv.ParseInt(parts[0], 10, 64)
		fraction := ""
		if len(parts) == 2 {
			fraction = parts[1]
		}
		fraction += strings.Repeat("0", 2-len(fraction))
		cents, _ := strconv.ParseInt(fraction, 10, 64)
		if whole*100+cents < 1 || whole*100+cents > 100000000 {
			return nil, errors.New("tarif di luar batas 0.01-1000000.00")
		}
		key := r.CategorySlug + "/" + r.ActiveFrom.UTC().Format(time.RFC3339Nano)
		if versions[key] {
			return nil, errors.New("versi kategori/waktu ganda")
		}
		ids[r.ID] = true
		versions[key] = true
	}
	return rates, nil
}

func seed(ctx context.Context, pool *pgxpool.Pool, rates []demoRate) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	for _, r := range rates {
		var category string
		if err = tx.QueryRow(ctx, `SELECT id::text FROM public.waste_categories WHERE slug=$1`, r.CategorySlug).Scan(&category); errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("kategori %s belum ada; jalankan seed kategori/mitra dahulu", r.CategorySlug)
		} else if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.point_rates(id,category_id,points_per_kg,active_from,is_demo) VALUES($1::uuid,$2::uuid,$3::numeric,$4,true) ON CONFLICT(id) DO NOTHING`, r.ID, category, r.PointsPerKG, r.ActiveFrom); err != nil {
			return err
		}
		var same bool
		if err = tx.QueryRow(ctx, `SELECT category_id=$2::uuid AND partner_id IS NULL AND points_per_kg=$3::numeric AND active_from=$4 AND active_until IS NULL AND is_demo FROM public.point_rates WHERE id=$1::uuid`, r.ID, category, r.PointsPerKG, r.ActiveFrom).Scan(&same); err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("tarif %s berbeda dari database; buat ID dan active_from baru untuk versi baru", r.ID)
		}
	}
	return tx.Commit(ctx)
}
