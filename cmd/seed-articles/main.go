// seed-articles upserts demo articles from JSON. While there is no admin
// (Q-09), the JSON file is the editing tool: re-running applies edits, and
// "is_published": false hides an article. Articles not in the file are left
// untouched. Run only against the intended development database.
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
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type section struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type demoArticle struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Category       string    `json:"category"`
	ReadingMinutes int       `json:"reading_minutes"`
	Introduction   string    `json:"introduction"`
	Sections       []section `json:"sections"`
	Tip            string    `json:"tip"`
	IsFeatured     bool      `json:"is_featured"`
	IsPublished    bool      `json:"is_published"`
	PublishedAt    time.Time `json:"published_at"`
}

var categories = []string{"GUIDE", "SORTING", "RECYCLING", "SPECIAL_WASTE", "NEWS"}

func main() {
	demo := flag.Bool("demo", false, "konfirmasi artikel simulasi pada development")
	file := flag.String("file", "data/seeders/dev/articles.json", "file artikel demo")
	flag.Parse()
	if !*demo || flag.NArg() != 0 {
		log.Fatal("gunakan --demo [--file <JSON>]; DATABASE_URL harus menunjuk development")
	}
	articles, err := readArticles(*file)
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
	err = seed(ctx, pool, articles)
	pool.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Seed artikel demo selesai: %d artikel diperbarui.", len(articles))
}

func readArticles(path string) ([]demoArticle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1024*1024 {
		return nil, errors.New("file maksimal 1 MiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var articles []demoArticle
	if err = decoder.Decode(&articles); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("file harus berisi satu array JSON")
	}
	if len(articles) < 1 || len(articles) > 200 {
		return nil, errors.New("isi 1-200 artikel")
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i, a := range articles {
		ok := uuid.MatchString(a.ID) && !seen[a.ID] &&
			strings.TrimSpace(a.Title) != "" && utf8.RuneCountInString(a.Title) <= 200 &&
			utf8.RuneCountInString(a.Summary) <= 500 && slices.Contains(categories, a.Category) &&
			a.ReadingMinutes >= 1 && a.ReadingMinutes <= 120 && len(a.Sections) <= 50 &&
			(!a.IsPublished || !a.PublishedAt.IsZero())
		for _, s := range a.Sections {
			ok = ok && strings.TrimSpace(s.Title) != "" && strings.TrimSpace(s.Body) != ""
		}
		if !ok {
			return nil, fmt.Errorf("artikel ke-%d invalid/duplikat", i+1)
		}
		seen[a.ID] = true
	}
	return articles, nil
}

func seed(ctx context.Context, pool *pgxpool.Pool, articles []demoArticle) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	for _, a := range articles {
		var publishedAt *time.Time
		if !a.PublishedAt.IsZero() {
			publishedAt = &a.PublishedAt
		}
		sections := a.Sections
		if sections == nil {
			sections = []section{}
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.articles(id,title,summary,category,reading_minutes,introduction,sections,tip,is_featured,is_published,published_at,is_demo)
 VALUES($1::uuid,$2,$3,$4::public.article_category,$5,$6,$7,$8,$9,$10,$11,true)
 ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,summary=EXCLUDED.summary,category=EXCLUDED.category,reading_minutes=EXCLUDED.reading_minutes,
 introduction=EXCLUDED.introduction,sections=EXCLUDED.sections,tip=EXCLUDED.tip,is_featured=EXCLUDED.is_featured,
 is_published=EXCLUDED.is_published,published_at=EXCLUDED.published_at,updated_at=now()
 WHERE public.articles.is_demo`,
			a.ID, a.Title, a.Summary, a.Category, a.ReadingMinutes, a.Introduction, sections, a.Tip, a.IsFeatured, a.IsPublished, publishedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
