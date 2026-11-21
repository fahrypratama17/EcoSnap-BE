package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var ErrArticleNotFound = errors.New("artikel tidak ditemukan")

var ArticleCategories = []string{"GUIDE", "SORTING", "RECYCLING", "SPECIAL_WASTE", "NEWS"}

// ArticleFilter: Search matches title or summary, case-insensitive.
type ArticleFilter struct {
	Search   *string
	Category *string
	Featured bool
}

type ArticleStore interface {
	Articles(ctx context.Context, f ArticleFilter, limit int, after *Cursor) ([]entity.ArticleSummary, error)
	Article(ctx context.Context, id string) (entity.Article, error)
}

type ArticleService struct{ store ArticleStore }

func NewArticleService(store ArticleStore) *ArticleService { return &ArticleService{store: store} }

type ArticleQuery struct {
	Limit            int
	Cursor           string
	Search, Category string
	Featured         bool
}

func (s *ArticleService) Articles(ctx context.Context, q ArticleQuery) (Page[entity.ArticleSummary], error) {
	limit, after, err := pageArgs(HistoryQuery{Limit: q.Limit, Cursor: q.Cursor})
	if err != nil {
		return Page[entity.ArticleSummary]{}, err
	}
	f := ArticleFilter{Featured: q.Featured}
	if search := strings.TrimSpace(q.Search); search != "" {
		if utf8.RuneCountInString(search) > 100 {
			return Page[entity.ArticleSummary]{}, ErrInvalidHistoryQuery
		}
		f.Search = &search
	}
	if q.Category != "" {
		valid := false
		for _, c := range ArticleCategories {
			valid = valid || c == q.Category
		}
		if !valid {
			return Page[entity.ArticleSummary]{}, ErrInvalidHistoryQuery
		}
		f.Category = &q.Category
	}
	items, err := s.store.Articles(ctx, f, limit+1, after)
	if err != nil {
		return Page[entity.ArticleSummary]{}, err
	}
	return trim(items, limit, func(a entity.ArticleSummary) (time.Time, string) { return a.PublishedAt, a.ID }), nil
}

func (s *ArticleService) Article(ctx context.Context, id string) (entity.Article, error) {
	if !partnerID.MatchString(id) {
		return entity.Article{}, ErrArticleNotFound
	}
	return s.store.Article(ctx, strings.ToLower(id))
}
