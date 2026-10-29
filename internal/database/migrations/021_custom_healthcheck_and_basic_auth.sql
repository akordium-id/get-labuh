ALTER TABLE applications ADD COLUMN healthcheck_path TEXT DEFAULT '/';
ALTER TABLE applications ADD COLUMN healthcheck_timeout INTEGER DEFAULT 30;
ALTER TABLE applications ADD COLUMN basic_auth_user TEXT;
ALTER TABLE applications ADD COLUMN basic_auth_password TEXT;
