-- Partial index for efficient TTL scans: only indexes gates that can still
-- become unresponsive (excludes already-unresponsive and never-seen gates).
-- The TTL worker queries: last_seen_at < NOW()-ttl AND status NOT IN ('unresponsive','unknown')
-- This index makes that scan O(k) where k = number of active gates with a timestamp.
CREATE INDEX IF NOT EXISTS idx_gates_ttl_candidates
    ON gates (last_seen_at)
    WHERE last_seen_at IS NOT NULL
      AND status NOT IN ('unresponsive', 'unknown');
