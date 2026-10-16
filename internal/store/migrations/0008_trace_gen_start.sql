-- 0008: generation start per stream (#116). Tokens/s is measured from the
-- flush of message_start, not from the first content delta: with hidden
-- thinking that delta only arrives after all thinking tokens. The stored
-- tokens/s verdicts used the old base and are dropped.
ALTER TABLE traces ADD COLUMN gen_start_us INTEGER CHECK (gen_start_us >= 0); -- NULL = unknown
DELETE FROM trace_tps;
