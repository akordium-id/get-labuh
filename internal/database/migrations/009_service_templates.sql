CREATE TABLE service_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    source_type TEXT NOT NULL DEFAULT 'git',
    repository_url TEXT,
    docker_image TEXT,
    compose_yaml TEXT,
    icon_url TEXT,
    category TEXT NOT NULL DEFAULT 'other',
    is_official BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE template_variables (
    id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL REFERENCES service_templates(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    required BOOLEAN DEFAULT true,
    default_value TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
