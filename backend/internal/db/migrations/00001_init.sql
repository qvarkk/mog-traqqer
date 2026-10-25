-- +goose Up

CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email        text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key UNIQUE (email)
);

-- Listing users newest-first is the common access pattern, so index for it.
CREATE INDEX users_created_at_idx ON users (created_at DESC);

-- +goose Down

DROP INDEX users_created_at_idx;
DROP TABLE users;
