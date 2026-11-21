-- +goose Up
-- Structured article content for FE-19 (introduction, sections, tip). Plain
-- text only: the app renders it as text, so no HTML/Markdown is executed.
CREATE TYPE public.article_category AS ENUM ('GUIDE','SORTING','RECYCLING','SPECIAL_WASTE','NEWS');
CREATE TABLE public.articles (
 id UUID PRIMARY KEY,
 title VARCHAR(200) NOT NULL CHECK(length(btrim(title))>0),
 summary VARCHAR(500) NOT NULL DEFAULT '',
 category public.article_category NOT NULL,
 reading_minutes SMALLINT NOT NULL CHECK(reading_minutes BETWEEN 1 AND 120),
 introduction TEXT NOT NULL DEFAULT '',
 -- [{"title": "...", "body": "..."}]; shape validated by the seed CLI.
 sections JSONB NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(sections)='array'),
 tip TEXT NOT NULL DEFAULT '',
 is_featured BOOLEAN NOT NULL DEFAULT false,
 is_published BOOLEAN NOT NULL DEFAULT false,
 published_at TIMESTAMPTZ,
 is_demo BOOLEAN NOT NULL DEFAULT false,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(NOT is_published OR published_at IS NOT NULL)
);
CREATE INDEX articles_published_idx ON public.articles(published_at DESC, id DESC) WHERE is_published;
ALTER TABLE public.articles ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.articles FROM anon, authenticated;
-- +goose Down
DROP TABLE public.articles;
DROP TYPE public.article_category;
