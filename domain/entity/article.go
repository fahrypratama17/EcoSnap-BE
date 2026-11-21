package entity

import "time"

// ArticleSummary is the list card; the detail adds the body.
type ArticleSummary struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Category       string    `json:"category"`
	ReadingMinutes int       `json:"reading_minutes"`
	IsFeatured     bool      `json:"is_featured"`
	IsDemo         bool      `json:"is_demo"`
	PublishedAt    time.Time `json:"published_at"`
}

type ArticleSection struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Article struct {
	ArticleSummary
	Introduction string           `json:"introduction"`
	Sections     []ArticleSection `json:"sections"`
	Tip          string           `json:"tip"`
}
