-- 0007: tokens/s verdict per request (#116). Own table, so trace_perf (#87)
-- stays unchanged. Numbers and fixed identifiers only.
CREATE TABLE trace_tps (
    trace_id           TEXT PRIMARY KEY REFERENCES traces (id) ON DELETE CASCADE,
    class              TEXT NOT NULL,
    samples            INTEGER NOT NULL CHECK (samples >= 0),
    tps_milli          INTEGER NOT NULL CHECK (tps_milli >= 0),
    p10_milli          INTEGER,
    median_milli       INTEGER,
    p90_milli          INTEGER,
    deviation_permille INTEGER,
    verdict            TEXT NOT NULL CHECK (verdict IN ('schneller', 'üblich', 'langsamer', 'unbekannt'))
) STRICT;
