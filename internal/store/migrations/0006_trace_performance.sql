-- 0006: performance measurements (#87). Numbers and fixed identifiers only.
ALTER TABLE traces ADD COLUMN ttft_us INTEGER;             -- NULL = unknown
ALTER TABLE traces ADD COLUMN return_us INTEGER;           -- NULL = no return path
ALTER TABLE traces ADD COLUMN return_replacements INTEGER; -- NULL = no return path

-- Comparison query: same model, time window before the request.
CREATE INDEX traces_model_started_idx ON traces (model, started_at DESC, id DESC);

-- Verdict per request and metric, computed once when the request finishes.
CREATE TABLE trace_perf (
    trace_id   TEXT NOT NULL REFERENCES traces (id) ON DELETE CASCADE,
    metric     TEXT NOT NULL CHECK (metric IN ('ttft', 'duration')),
    class      TEXT NOT NULL,
    samples    INTEGER NOT NULL CHECK (samples >= 0),
    median_us  INTEGER,
    p90_us     INTEGER,
    value_us   INTEGER NOT NULL CHECK (value_us >= 0),
    verdict    TEXT NOT NULL CHECK (verdict IN ('normal', 'langsam', 'unbekannt')),
    PRIMARY KEY (trace_id, metric)
) STRICT;

-- Reserved internal client for the speedtest. 32 zero bytes have no known
-- preimage and the blocked status fails every key check anyway.
INSERT INTO clients (id, name, key_hash, key_hint, status, created_at)
VALUES ('speedtest', 'tollgate:speedtest', zeroblob(32), '', 'gesperrt', 0);
