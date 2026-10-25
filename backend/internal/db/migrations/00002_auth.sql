-- +goose Up

CREATE TABLE refresh_tokens (
    jti         uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_info jsonb NOT NULL,
    is_revoked  boolean NOT NULL DEFAULT false,
    expires_at  timestamp(0) with time zone NOT NULL,
    created_at  timestamp(0) with time zone NOT NULL DEFAULT now(),
    updated_at  timestamp(0) with time zone NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_expires_at_index ON refresh_tokens (expires_at);
CREATE INDEX refresh_tokens_user_id_index    ON refresh_tokens (user_id);

-- +goose Down

DROP TABLE refresh_tokens;
