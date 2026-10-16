-- 0001: initial schema (ADR-0006, ADR-0008, ADR-0009).
-- Only redacted data is stored: hashes instead of keys, no request or response bodies.
-- All timestamps are Unix milliseconds.

CREATE TABLE clients (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    key_hash    BLOB NOT NULL UNIQUE CHECK (length(key_hash) = 32),
    key_hint    TEXT NOT NULL CHECK (length(key_hint) <= 16),
    status      TEXT NOT NULL CHECK (status IN ('aktiv', 'gesperrt')),
    created_at  INTEGER NOT NULL,
    rotated_at  INTEGER
) STRICT;

-- Config versions are append-only. words_enc is an opaque AES-GCM blob (ADR-0009).
CREATE TABLE config_versions (
    id          INTEGER PRIMARY KEY,
    created_at  INTEGER NOT NULL,
    author      TEXT NOT NULL,
    comment     TEXT NOT NULL,
    content     BLOB NOT NULL,
    words_enc   BLOB NOT NULL,
    diff        TEXT NOT NULL
) STRICT;

CREATE TRIGGER config_versions_no_update BEFORE UPDATE ON config_versions
BEGIN
    SELECT RAISE(ABORT, 'config_versions is append-only');
END;

-- INSERT OR REPLACE would delete the old row without firing the DELETE trigger
-- (recursive_triggers is off), so an insert onto an existing id is refused.
CREATE TRIGGER config_versions_no_replace BEFORE INSERT ON config_versions
WHEN EXISTS (SELECT 1 FROM config_versions WHERE id = NEW.id)
BEGIN
    SELECT RAISE(ABORT, 'config_versions is append-only');
END;

CREATE TRIGGER config_versions_no_delete BEFORE DELETE ON config_versions
BEGIN
    SELECT RAISE(ABORT, 'config_versions is append-only');
END;

-- Single row pointing at the active version.
CREATE TABLE config_active (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    version_id  INTEGER NOT NULL REFERENCES config_versions (id)
) STRICT;

CREATE TABLE sessions (
    key_hash          BLOB PRIMARY KEY CHECK (length(key_hash) = 32),
    client_id         TEXT NOT NULL REFERENCES clients (id),
    config_version_id INTEGER NOT NULL REFERENCES config_versions (id),
    created_at        INTEGER NOT NULL,
    last_seen_at      INTEGER NOT NULL
) STRICT;

CREATE INDEX sessions_last_seen ON sessions (last_seen_at);

CREATE TABLE traces (
    id                      TEXT PRIMARY KEY,
    client_id               TEXT NOT NULL REFERENCES clients (id),
    session_key_hash        BLOB,
    config_version_id       INTEGER REFERENCES config_versions (id),
    started_at              INTEGER NOT NULL,
    endpoint                TEXT NOT NULL,
    model                   TEXT NOT NULL,
    stream                  INTEGER NOT NULL CHECK (stream IN (0, 1)),
    estimated_input_tokens  INTEGER NOT NULL,
    status                  INTEGER,
    duration_us             INTEGER,
    usage_input             INTEGER,
    usage_output            INTEGER,
    usage_cache_read        INTEGER,
    usage_cache_creation    INTEGER,
    error_kind              TEXT NOT NULL DEFAULT ''
) STRICT;

CREATE INDEX traces_started_at ON traces (started_at DESC, id);
CREATE INDEX traces_client ON traces (client_id, started_at DESC);

CREATE TABLE trace_stages (
    trace_id            TEXT NOT NULL REFERENCES traces (id) ON DELETE CASCADE,
    seq                 INTEGER NOT NULL,
    name                TEXT NOT NULL,
    duration_us         INTEGER NOT NULL,
    bytes_in            INTEGER NOT NULL,
    bytes_out           INTEGER NOT NULL,
    tokens_in           INTEGER NOT NULL,
    tokens_out          INTEGER NOT NULL,
    hash_in             TEXT NOT NULL CHECK (length(hash_in) = 64),
    hash_out            TEXT NOT NULL CHECK (length(hash_out) = 64),
    config_version_id   INTEGER REFERENCES config_versions (id),
    diff                TEXT NOT NULL,
    matches             TEXT NOT NULL,
    PRIMARY KEY (trace_id, seq)
) STRICT;
