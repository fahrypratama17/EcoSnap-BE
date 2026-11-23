package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ArticleRepository struct{ pool *pgxpool.Pool }

func NewArticleRepository(pool *pgxpool.Pool) *ArticleRepository {
	return &ArticleRepository{pool: pool}
}

const articleSummaryColumns = `id::text,title,summary,category::text,reading_minutes,is_featured,is_demo,published_at`

// Only published articles whose publish time has passed are visible, so a
// scheduled article stays hidden until then. now() is the enclosing
// transaction's start time (the snapshot's visibility instant), not an exact
// wall-clock sample.
const articleVisible = `is_published AND published_at<=now()`

// likePattern escapes LIKE wildcards so the search is a literal substring.
func likePattern(s string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%"
}

func (r *ArticleRepository) Articles(ctx context.Context, f service.ArticleFilter, limit int, after *service.Cursor) ([]entity.ArticleSummary, error) {
	return articlesTx(ctx, r.pool, f, limit, after)
}

// articlesTx lists visible articles newest first; shared by the article list
// and the home card.
func articlesTx(ctx context.Context, q rowQuerier, f service.ArticleFilter, limit int, after *service.Cursor) ([]entity.ArticleSummary, error) {
	var pattern *string
	if f.Search != nil {
		p := likePattern(*f.Search)
		pattern = &p
	}
	at, id := cursorArgs(after)
	rows, err := q.Query(ctx, `SELECT `+articleSummaryColumns+` FROM public.articles
 WHERE `+articleVisible+`
 AND ($1::text IS NULL OR title ILIKE $1 OR summary ILIKE $1)
 AND ($2::text IS NULL OR category::text=$2)
 AND (NOT $3 OR is_featured)
 AND ($4::timestamptz IS NULL OR (published_at,id)<($4,$5::uuid))
 ORDER BY published_at DESC,id DESC LIMIT $6`, pattern, f.Category, f.Featured, at, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.ArticleSummary{}
	for rows.Next() {
		var a entity.ArticleSummary
		if err = rows.Scan(&a.ID, &a.Title, &a.Summary, &a.Category, &a.ReadingMinutes, &a.IsFeatured, &a.IsDemo, &a.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ArticleRepository) Article(ctx context.Context, id string) (entity.Article, error) {
	var a entity.Article
	err := r.pool.QueryRow(ctx, `SELECT `+articleSummaryColumns+`,introduction,sections,tip FROM public.articles WHERE id=$1::uuid AND `+articleVisible, id).
		Scan(&a.ID, &a.Title, &a.Summary, &a.Category, &a.ReadingMinutes, &a.IsFeatured, &a.IsDemo, &a.PublishedAt, &a.Introduction, &a.Sections, &a.Tip)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, service.ErrArticleNotFound
	}
	if a.Sections == nil {
		a.Sections = []entity.ArticleSection{}
	}
	return a, err
}
