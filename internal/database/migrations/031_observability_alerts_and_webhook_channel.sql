-- Migration 031: Menambahkan dukungan channel Webhook dan event Server Resource Alert
ALTER TABLE notification_channels ADD COLUMN webhook_url TEXT;
ALTER TABLE notification_channels ADD COLUMN webhook_secret TEXT;
ALTER TABLE notification_channels ADD COLUMN on_server_alert BOOLEAN NOT NULL DEFAULT 1;
