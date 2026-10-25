-- +goose Up

-- Matches schema.sql. ids are client-generated (offline-first), so there is no
-- DEFAULT; real password hashing lands in Phase 2.
CREATE TABLE users (
    id            uuid PRIMARY KEY,
    email         text NOT NULL,
    password_hash text NOT NULL,
    created_at    timestamp(0) with time zone NOT NULL DEFAULT now(),
    updated_at    timestamp(0) with time zone NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

-- +goose Down

DROP TABLE users;
