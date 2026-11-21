// seed-rewards inserts demo rewards without overwriting existing rows. Stock
// is set only on first insert because redemptions change it afterwards.
// Run only against the intended development database.
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
	"github.com/jackc/pgx/v5/pgxpool"
)

type demoReward struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Terms       string `json:"terms"`
	CostPoints  int64  `json:"cost_points"`
	Stock       int64  `json:"stock"`
}

func main() {
	demo := flag.Bool("demo", false, "konfirmasi reward simulasi pada development")
	file := flag.String("file", "data/seeders/dev/rewards.json", "file reward demo")
	flag.Parse()
	if !*demo || flag.NArg() != 0 {
		log.Fatal("gunakan --demo [--file <JSON>]; DATABASE_URL harus menunjuk development")
	}
	rewards, err := readRewards(*file)
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
	err = seed(ctx, pool, rewards)
	pool.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Seed reward demo selesai: %d reward; reward lama tidak ditimpa.", len(rewards))
}

func readRewards(path string) ([]demoReward, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 64*1024 {
		return nil, errors.New("file maksimal 64 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var rewards []demoReward
	if err = decoder.Decode(&rewards); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("file harus berisi satu array JSON")
	}
	if len(rewards) < 1 || len(rewards) > 100 {
		return nil, errors.New("isi 1-100 reward")
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i, r := range rewards {
		name := strings.TrimSpace(r.Name)
		if !uuid.MatchString(r.ID) || seen[r.ID] || name == "" || len(name) > 120 || r.CostPoints < 1 || r.CostPoints > 100000000 || r.Stock < 0 || r.Stock > 1000000 {
			return nil, fmt.Errorf("reward ke-%d invalid/duplikat", i+1)
		}
		seen[r.ID] = true
	}
	return rewards, nil
}

func seed(ctx context.Context, pool *pgxpool.Pool, rewards []demoReward) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	for _, r := range rewards {
		if _, err = tx.Exec(ctx, `INSERT INTO public.rewards(id,name,description,terms,cost_points,stock,is_demo) VALUES($1::uuid,$2,$3,$4,$5,$6,true) ON CONFLICT(id) DO NOTHING`, r.ID, r.Name, r.Description, r.Terms, r.CostPoints, r.Stock); err != nil {
			return err
		}
		var same bool
		if err = tx.QueryRow(ctx, `SELECT name=$2 AND description=$3 AND terms=$4 AND cost_points=$5 AND is_demo FROM public.rewards WHERE id=$1::uuid`, r.ID, r.Name, r.Description, r.Terms, r.CostPoints).Scan(&same); err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("reward %s berbeda dari database; gunakan ID baru untuk reward baru", r.ID)
		}
	}
	return tx.Commit(ctx)
}
