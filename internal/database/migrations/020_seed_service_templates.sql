-- Migration 020: Seed One-Click Service Catalog Templates and Variables
INSERT OR IGNORE INTO service_templates (id, name, description, source_type, docker_image, category, is_official, created_at, updated_at)
VALUES
('tpl-redis', 'Redis', 'In-memory data structure store used as a database, cache, and message broker.', 'docker_image', 'redis:7-alpine', 'database', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-minio', 'MinIO', 'High-performance, S3-compatible object storage server with web console.', 'docker_image', 'minio/minio:latest', 'storage', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-vault', 'HashiCorp Vault', 'Identity-based secrets management and data encryption service.', 'docker_image', 'hashicorp/vault:latest', 'security', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-meilisearch', 'Meilisearch', 'Lightning fast, highly relevant and typo-tolerant search engine.', 'docker_image', 'getmeili/meilisearch:v1.6', 'search', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-n8n', 'n8n', 'Fair-code workflow automation tool to connect any apps and services.', 'docker_image', 'n8nio/n8n:latest', 'automation', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-uptime-kuma', 'Uptime Kuma', 'Easy-to-use self-hosted monitoring tool with status pages and alerts.', 'docker_image', 'louislam/uptime-kuma:1', 'monitoring', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-postgres', 'PostgreSQL', 'Powerful, open-source object-relational database system.', 'docker_image', 'postgres:16-alpine', 'database', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('tpl-mysql', 'MySQL', 'Widely used open-source relational database management system.', 'docker_image', 'mysql:8.0', 'database', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Template Variables
-- MinIO
INSERT OR IGNORE INTO template_variables (id, template_id, key, display_name, description, required, default_value, created_at)
VALUES
('var-minio-1', 'tpl-minio', 'MINIO_ROOT_USER', 'Root Username', 'Admin access key for MinIO', 1, 'minioadmin', CURRENT_TIMESTAMP),
('var-minio-2', 'tpl-minio', 'MINIO_ROOT_PASSWORD', 'Root Password', 'Admin secret password for MinIO (min 8 chars)', 1, 'minioadmin123', CURRENT_TIMESTAMP);

-- Meilisearch
INSERT OR IGNORE INTO template_variables (id, template_id, key, display_name, description, required, default_value, created_at)
VALUES
('var-meili-1', 'tpl-meilisearch', 'MEILI_MASTER_KEY', 'Master Key', 'Security key to protect API endpoints (min 16 bytes)', 1, 'labuh_meili_secret_master_key_123', CURRENT_TIMESTAMP),
('var-meili-2', 'tpl-meilisearch', 'MEILI_ENV', 'Environment', 'Meilisearch environment mode (production/development)', 0, 'production', CURRENT_TIMESTAMP);

-- n8n
INSERT OR IGNORE INTO template_variables (id, template_id, key, display_name, description, required, default_value, created_at)
VALUES
('var-n8n-1', 'tpl-n8n', 'N8N_PORT', 'N8N Web Port', 'Port where n8n listens', 0, '5678', CURRENT_TIMESTAMP),
('var-n8n-2', 'tpl-n8n', 'GENERIC_TIMEZONE', 'Timezone', 'Default timezone for workflow cron triggers', 0, 'Asia/Jakarta', CURRENT_TIMESTAMP);

-- PostgreSQL
INSERT OR IGNORE INTO template_variables (id, template_id, key, display_name, description, required, default_value, created_at)
VALUES
('var-pg-1', 'tpl-postgres', 'POSTGRES_USER', 'Username', 'Default PostgreSQL superuser', 1, 'postgres', CURRENT_TIMESTAMP),
('var-pg-2', 'tpl-postgres', 'POSTGRES_PASSWORD', 'Password', 'Default superuser password', 1, 'labuh_postgres_pass_123', CURRENT_TIMESTAMP),
('var-pg-3', 'tpl-postgres', 'POSTGRES_DB', 'Database Name', 'Initial database to create', 1, 'labuh_db', CURRENT_TIMESTAMP);

-- MySQL
INSERT OR IGNORE INTO template_variables (id, template_id, key, display_name, description, required, default_value, created_at)
VALUES
('var-my-1', 'tpl-mysql', 'MYSQL_ROOT_PASSWORD', 'Root Password', 'Root user password for MySQL', 1, 'labuh_mysql_root_123', CURRENT_TIMESTAMP),
('var-my-2', 'tpl-mysql', 'MYSQL_DATABASE', 'Database Name', 'Initial database to create', 1, 'labuh_db', CURRENT_TIMESTAMP),
('var-my-3', 'tpl-mysql', 'MYSQL_USER', 'App User', 'Dedicated application username', 0, 'app_user', CURRENT_TIMESTAMP),
('var-my-4', 'tpl-mysql', 'MYSQL_PASSWORD', 'App Password', 'Password for dedicated application user', 0, 'app_pass_123', CURRENT_TIMESTAMP);
