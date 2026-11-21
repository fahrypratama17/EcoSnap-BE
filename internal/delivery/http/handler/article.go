package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type ArticleHandler struct{ articles *service.ArticleService }

func NewArticleHandler(articles *service.ArticleService) *ArticleHandler {
	return &ArticleHandler{articles: articles}
}

// List accepts limit, cursor, q (search), category, and featured=true|false.
func (h *ArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	q := service.ArticleQuery{Limit: 20}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		articleError(w, service.ErrInvalidHistoryQuery)
		return
	}
	for key, v := range values {
		if len(v) != 1 {
			articleError(w, service.ErrInvalidHistoryQuery)
			return
		}
		switch key {
		case "limit":
			q.Limit, err = strconv.Atoi(v[0])
		case "cursor":
			q.Cursor = v[0]
		case "q":
			q.Search = v[0]
		case "category":
			q.Category = v[0]
		case "featured":
			q.Featured, err = strconv.ParseBool(v[0])
		default:
			err = service.ErrInvalidHistoryQuery
		}
		if err != nil {
			articleError(w, service.ErrInvalidHistoryQuery)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.articles.Articles(ctx, q)
	if err != nil {
		articleError(w, err)
		return
	}
	pageJSON(w, page)
}

func (h *ArticleHandler) Detail(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		articleError(w, service.ErrInvalidHistoryQuery)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	a, err := h.articles.Article(ctx, r.PathValue("articleID"))
	if err != nil {
		articleError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": a})
}

func articleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidHistoryQuery):
		response.Error(w, 400, "invalid_query", "Parameter artikel tidak valid.")
	case errors.Is(err, service.ErrArticleNotFound):
		response.Error(w, 404, "article_not_found", err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui.")
	default:
		log.Printf("artikel gagal: %v", err)
		response.Error(w, 500, "internal_error", "Artikel belum dapat dimuat.")
	}
}
