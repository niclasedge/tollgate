-- 0010: 7-day window utilization per request (#136), read from the
-- anthropic-ratelimit-unified-7d-* response headers. Numbers only.
ALTER TABLE traces ADD COLUMN seven_day_permille INTEGER CHECK (seven_day_permille >= 0); -- NULL = unknown
ALTER TABLE traces ADD COLUMN seven_day_reset INTEGER;                                     -- unix seconds, NULL = unknown
