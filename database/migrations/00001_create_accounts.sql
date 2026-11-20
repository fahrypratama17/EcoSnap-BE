-- +goose Up
CREATE TYPE public.account_role AS ENUM (
    'user',
    'mitra'
);

CREATE TYPE public.account_status AS ENUM (
    'active',
    'suspended',
    'deleted'
);

CREATE TABLE public.accounts (
    firebase_uid VARCHAR(128) PRIMARY KEY,
    role public.account_role NOT NULL DEFAULT 'user',
    full_name VARCHAR(150) NOT NULL,
    display_name VARCHAR(80) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(32),
    photo_path TEXT,
    status public.account_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE public.user_settings (
    firebase_uid VARCHAR(128) PRIMARY KEY
        REFERENCES public.accounts(firebase_uid),
    notification_enabled BOOLEAN NOT NULL DEFAULT true,
    show_on_leaderboard BOOLEAN NOT NULL DEFAULT true,
    onboarding_completed BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE public.wallets (
    user_uid VARCHAR(128) PRIMARY KEY
        REFERENCES public.accounts(firebase_uid),
    balance INTEGER NOT NULL DEFAULT 0
        CHECK (balance >= 0),
    total_earned INTEGER NOT NULL DEFAULT 0
        CHECK (total_earned >= 0),
    total_spent INTEGER NOT NULL DEFAULT 0
        CHECK (total_spent >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.wallets ENABLE ROW LEVEL SECURITY;

REVOKE ALL PRIVILEGES
ON TABLE public.accounts, public.user_settings, public.wallets
FROM anon, authenticated;

-- +goose Down

DROP TABLE public.wallets;
DROP TABLE public.user_settings;
DROP TABLE public.accounts;

DROP TYPE public.account_status;
DROP TYPE public.account_role;