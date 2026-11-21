package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

func TestArticleIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..", "..")
	cfg := pool.Config().ConnConfig
	dbURL := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable", User: url.UserPassword(cfg.User, cfg.Password)}
	seed := func(t *testing.T, file string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", "run", "-p=1", "-mod=readonly", "./cmd/seed-articles", "--demo", "--file", file)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dbURL.String())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seed: %v %s", err, out)
		}
	}
	demoFile := filepath.Join(root, "data", "seeders", "dev", "articles.json")
	seed(t, demoFile)
	seed(t, demoFile) // idempotent
	// A draft, a scheduled article, and a non-demo article whose content the
	// seed must never overwrite.
	_, err := pool.Exec(ctx, `INSERT INTO public.articles(id,title,summary,category,reading_minutes,is_published,published_at) VALUES
 ('90000000-0000-4000-8000-0000000000d1','Draft rahasia','draft','NEWS',1,false,NULL),
 ('90000000-0000-4000-8000-0000000000d2','Terjadwal','besok','NEWS',1,true,now()+interval '1 day'),
 ('90000000-0000-4000-8000-0000000000d3','Asli 100% plastik','bukan demo','GUIDE',1,true,'2026-11-02T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	articles := service.NewArticleService(postgres.NewArticleRepository(pool))
	list := func(t *testing.T, q service.ArticleQuery) []string {
		t.Helper()
		if q.Limit == 0 {
			q.Limit = 100
		}
		page, err := articles.Articles(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		titles := make([]string, len(page.Items))
		for i, a := range page.Items {
			titles[i] = a.Title
		}
		return titles
	}

	t.Run("only_published_and_due_articles_newest_first", func(t *testing.T) {
		got := list(t, service.ArticleQuery{})
		if len(got) != 6 || got[0] != "EcoSnap Hadir untuk Warga Malang" || got[5] != "Cara Memilah Sampah Rumah Tangga dengan Benar" {
			t.Fatalf("list: %v", got)
		}
		for _, id := range []string{"90000000-0000-4000-8000-0000000000d1", "90000000-0000-4000-8000-0000000000d2"} {
			if _, err := articles.Article(ctx, id); !errors.Is(err, service.ErrArticleNotFound) {
				t.Fatalf("hidden %s: %v", id, err)
			}
		}
	})
	t.Run("search_category_featured_and_paging", func(t *testing.T) {
		if got := list(t, service.ArticleQuery{Search: "PLASTIK"}); len(got) != 2 {
			t.Fatalf("case-insensitive search: %v", got)
		}
		// % is literal, not a wildcard.
		if got := list(t, service.ArticleQuery{Search: "100%"}); len(got) != 1 || got[0] != "Asli 100% plastik" {
			t.Fatalf("literal percent: %v", got)
		}
		if got := list(t, service.ArticleQuery{Search: "_"}); len(got) != 0 {
			t.Fatalf("literal underscore: %v", got)
		}
		if got := list(t, service.ArticleQuery{Category: "SPECIAL_WASTE"}); len(got) != 1 {
			t.Fatalf("category: %v", got)
		}
		if got := list(t, service.ArticleQuery{Featured: true}); len(got) != 2 {
			t.Fatalf("featured: %v", got)
		}
		seen, cursor := map[string]bool{}, ""
		for {
			page, err := articles.Articles(ctx, service.ArticleQuery{Limit: 4, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range page.Items {
				if seen[a.ID] {
					t.Fatalf("duplicate %s", a.ID)
				}
				seen[a.ID] = true
			}
			if cursor = page.NextCursor; cursor == "" {
				break
			}
		}
		if len(seen) != 6 {
			t.Fatalf("paged %d", len(seen))
		}
		if _, err := articles.Articles(ctx, service.ArticleQuery{Limit: 10, Category: "BLOG"}); !errors.Is(err, service.ErrInvalidHistoryQuery) {
			t.Fatalf("bad category: %v", err)
		}
	})
	t.Run("detail_has_structured_body", func(t *testing.T) {
		a, err := articles.Article(ctx, "90000000-0000-4000-8000-000000000003")
		if err != nil || len(a.Sections) != 3 || a.Sections[0].Title != "Simpan terpisah dan kering" || a.Tip == "" || a.Introduction == "" || !a.IsDemo || a.ReadingMinutes != 3 {
			t.Fatalf("detail: %+v %v", a, err)
		}
		a, err = articles.Article(ctx, "90000000-0000-4000-8000-0000000000d3")
		if err != nil || a.Sections == nil || len(a.Sections) != 0 {
			t.Fatalf("empty sections: %+v %v", a, err)
		}
	})
	t.Run("reseed_applies_edits_and_unpublish_but_spares_real_articles", func(t *testing.T) {
		raw, err := os.ReadFile(demoFile)
		if err != nil {
			t.Fatal(err)
		}
		var items []map[string]any
		if err = json.Unmarshal(raw, &items); err != nil {
			t.Fatal(err)
		}
		for _, a := range items {
			switch a["id"] {
			case "90000000-0000-4000-8000-000000000004":
				a["title"] = "Kompos Rumahan"
			case "90000000-0000-4000-8000-000000000005":
				a["is_published"] = false
			case "90000000-0000-4000-8000-000000000002":
				// Same ID as the real article: the seed must not take it over.
				a["id"] = "90000000-0000-4000-8000-0000000000d3"
			}
		}
		edited, err := json.Marshal(items)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "articles.json")
		if err = os.WriteFile(file, edited, 0600); err != nil {
			t.Fatal(err)
		}
		seed(t, file)
		got := strings.Join(list(t, service.ArticleQuery{}), "|")
		if !strings.Contains(got, "Kompos Rumahan") || strings.Contains(got, "EcoSnap Hadir") || !strings.Contains(got, "Asli 100% plastik") {
			t.Fatalf("after reseed: %s", got)
		}
	})
}
