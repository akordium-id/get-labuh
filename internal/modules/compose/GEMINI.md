# Module: Compose Applications

Package `internal/modules/compose` — manajemen aplikasi multi-container berbasis Docker Compose.

## Tanggung Jawab

- **`repo.go`**:
  - `Repo`: CRUD aplikasi compose (`compose_applications`) per environment.
  - Tracking status aplikasi: `idle`, `building`, `running`, `stopped`.
  - Custom domain routing configuration.
- **`handler.go`**:
  - Halaman list compose apps per environment `/projects/{project_id}/environments/{env_id}/compose-apps`.
  - Halaman create & form submit compose apps.
  - Detail status compose app `/compose-apps/{id}`.
  - Lifecycle controller: deploy, start, stop.
  - Custom domain management (Caddy integration).
- **`module.go`**:
  - Inisialisasi modul `New(db, caddyClient, settingProvider)`.
  - Registrasi rute `RegisterRoutes(r chi.Router)`.

## Rules untuk AI Agent

1. Selalu sertakan `context.Context` untuk semua query ke database (`*WithContext`).
2. Jangan mengimpor modul handler legacy ke dalam modul compose.
3. Akses ke pengaturan reverse proxy Caddy dilakukan via interface `SettingProvider`.
