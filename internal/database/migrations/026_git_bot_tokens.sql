-- Migration 026: Tambahkan kolom git_token ke tabel applications untuk override bot comment token
ALTER TABLE applications ADD COLUMN git_token TEXT;
