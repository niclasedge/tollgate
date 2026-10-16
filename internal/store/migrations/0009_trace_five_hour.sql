-- 0009: 5-hour window utilization per request (#134), read from the
-- anthropic-ratelimit-unified-5h-* response headers. Numbers only.
ALTER TABLE traces ADD COLUMN five_hour_permille INTEGER CHECK (five_hour_permille >= 0); -- NULL = unknown
ALTER TABLE traces ADD COLUMN five_hour_reset INTEGER;                                     -- unix seconds, NULL = unknown
