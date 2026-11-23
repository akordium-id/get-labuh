ALTER TABLE compose_applications ADD COLUMN raw_yaml TEXT NOT NULL DEFAULT '';
ALTER TABLE compose_applications ADD COLUMN parsed_services TEXT NOT NULL DEFAULT '[]';
ALTER TABLE compose_applications ADD COLUMN last_deployed_at DATETIME;
ALTER TABLE compose_applications ADD COLUMN error_message TEXT;
