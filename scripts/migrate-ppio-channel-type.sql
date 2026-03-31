-- Migration: PPIO ChannelType 53 → 54
-- Reason: Upstream labring/aiproxy introduced Fake adaptor (type=53),
--         conflicting with our PPIO adaptor. PPIO moved to type=54.
-- Date: 2026-03-31
-- Related commit: merge-upstream branch (labring/aiproxy #492–#503)
--
-- IMPORTANT: Run this BEFORE deploying the new binary.
-- If type=53 PPIO channels are not migrated, they will be routed
-- to the Fake adaptor instead of PPIO, breaking all PPIO requests.
--
-- Safe to run multiple times (idempotent if no Fake channels exist yet).
-- If you have intentionally created Fake (type=53) channels, do NOT run blindly —
-- filter by base_url instead:
--   UPDATE channels SET type = 54 WHERE type = 53 AND (base_url LIKE '%ppio%' OR base_url LIKE '%ppinfra%');

-- Standard migration (all type=53 → 54, safe if no Fake channels exist):
UPDATE channels SET type = 54 WHERE type = 53;

-- Verify:
SELECT id, name, type, base_url FROM channels WHERE type IN (53, 54);
