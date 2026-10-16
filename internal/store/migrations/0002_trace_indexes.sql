-- 0002: make the ListTraces keyset order index-only (issue #46).
-- ORDER BY started_at DESC, id DESC cannot use (started_at DESC, id ASC) from
-- 0001 without a temp B-tree, so both trace indexes are rebuilt with id DESC.

DROP INDEX traces_started_at;
DROP INDEX traces_client;
CREATE INDEX traces_started_at ON traces (started_at DESC, id DESC);
CREATE INDEX traces_client ON traces (client_id, started_at DESC, id DESC);
