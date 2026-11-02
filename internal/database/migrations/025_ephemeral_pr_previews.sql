-- Migration 025: Ephemeral PR Previews
ALTER TABLE applications ADD COLUMN parent_app_id TEXT REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE applications ADD COLUMN pr_number INTEGER;
ALTER TABLE applications ADD COLUMN is_preview BOOLEAN DEFAULT FALSE;
ALTER TABLE applications ADD COLUMN preview_template_domain TEXT;
ALTER TABLE applications ADD COLUMN pr_previews_enabled BOOLEAN DEFAULT FALSE;

CREATE INDEX idx_applications_parent_pr ON applications(parent_app_id, pr_number);
