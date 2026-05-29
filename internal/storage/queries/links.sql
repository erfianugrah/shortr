-- queries for the shortener bounded context.

-- name: InsertLink :exec
INSERT INTO links (
    slug, target_url, created_at, expires_at, password_hash,
    max_clicks, note, created_by
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLink :one
SELECT slug, target_url, created_at, expires_at, password_hash,
       max_clicks, click_count, note, created_by
FROM   links
WHERE  slug = ?
LIMIT  1;

-- name: GetActiveLink :one
-- Hot path: the only query that runs on the redirect critical path.
-- Returns the link only if it isn't expired and hasn't exhausted max_clicks.
SELECT slug, target_url, created_at, expires_at, password_hash,
       max_clicks, click_count, note, created_by
FROM   links
WHERE  slug = ?
  AND  (expires_at IS NULL OR expires_at > ?)
  AND  (max_clicks IS NULL OR click_count < max_clicks)
LIMIT  1;

-- name: ListLinks :many
SELECT slug, target_url, created_at, expires_at, password_hash,
       max_clicks, click_count, note, created_by
FROM   links
WHERE  created_at < ?
ORDER  BY created_at DESC
LIMIT  ?;

-- name: UpdateLink :exec
UPDATE links
SET    target_url    = ?,
       expires_at    = ?,
       password_hash = ?,
       max_clicks    = ?,
       note          = ?
WHERE  slug = ?;

-- name: DeleteLink :exec
DELETE FROM links WHERE slug = ?;

-- name: IncrementClickCount :exec
UPDATE links SET click_count = click_count + 1 WHERE slug = ?;
