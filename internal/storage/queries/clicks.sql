-- queries for the analytics bounded context.

-- name: InsertClick :exec
INSERT INTO clicks (slug, ts, country, user_agent, referrer, ip_hash, fly_region)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListClicksForSlug :many
SELECT id, slug, ts, country, user_agent, referrer, ip_hash, fly_region
FROM   clicks
WHERE  slug = ?
  AND  ts < ?
ORDER  BY ts DESC
LIMIT  ?;

-- name: CountClicksForSlug :one
SELECT COUNT(*) FROM clicks WHERE slug = ?;

-- name: ClicksByDay :many
-- Aggregated daily counts for the dashboard's per-link sparkline.
SELECT (ts / 86400) * 86400 AS day_start,
       COUNT(*)              AS hits
FROM   clicks
WHERE  slug = ?
  AND  ts >= ?
GROUP  BY day_start
ORDER  BY day_start ASC;
