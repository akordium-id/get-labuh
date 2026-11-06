-- Migration 027: Tambahkan kolom lifecycle commands (pre/post deploy) ke tabel applications
ALTER TABLE applications ADD COLUMN pre_deployment_command TEXT;
ALTER TABLE applications ADD COLUMN post_deployment_command TEXT;
