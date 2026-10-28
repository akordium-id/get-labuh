-- Migration 019: Container Resource Limits (CPU & Memory)
ALTER TABLE applications ADD COLUMN cpu_limit REAL DEFAULT NULL;
ALTER TABLE applications ADD COLUMN memory_limit INTEGER DEFAULT NULL;
ALTER TABLE applications ADD COLUMN memory_reservation INTEGER DEFAULT NULL;

ALTER TABLE database_services ADD COLUMN cpu_limit REAL DEFAULT NULL;
ALTER TABLE database_services ADD COLUMN memory_limit INTEGER DEFAULT NULL;
ALTER TABLE database_services ADD COLUMN memory_reservation INTEGER DEFAULT NULL;
