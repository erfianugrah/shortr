-- +goose Up
-- +goose StatementBegin
CREATE TABLE links (
    slug          TEXT    NOT NULL PRIMARY KEY,
    target_url    TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,                    -- unix seconds
    expires_at    INTEGER,                             -- nullable; null = never
    password_hash TEXT,                                 -- nullable bcrypt
    max_clicks    INTEGER,                             -- nullable; null = unlimited
    click_count   INTEGER NOT NULL DEFAULT 0,
    note          TEXT    NOT NULL DEFAULT '',
    created_by    TEXT    NOT NULL DEFAULT 'admin'
) STRICT;

CREATE INDEX idx_links_created_at ON links(created_at DESC);
CREATE INDEX idx_links_expires    ON links(expires_at) WHERE expires_at IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE clicks (
    id          INTEGER PRIMARY KEY,
    slug        TEXT    NOT NULL REFERENCES links(slug) ON DELETE CASCADE,
    ts          INTEGER NOT NULL,
    country     TEXT    NOT NULL DEFAULT '',
    user_agent  TEXT    NOT NULL DEFAULT '',
    referrer    TEXT    NOT NULL DEFAULT '',
    ip_hash     TEXT    NOT NULL DEFAULT '',
    fly_region  TEXT    NOT NULL DEFAULT ''
) STRICT;

CREATE INDEX idx_clicks_slug_ts ON clicks(slug, ts DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS clicks;
DROP TABLE IF EXISTS links;
-- +goose StatementEnd
