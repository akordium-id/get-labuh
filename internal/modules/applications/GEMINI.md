# Modul Applications

Modul `applications` mengelola siklus hidup aplikasi (Application lifecycle), environment variables, webhook integrasi Git, magic dropzone upload, dan metrik monitoring container di get-labuh.

## Struktur Modul

- `repo.go`: Data access layer untuk tabel `applications` dan `app_env_vars` dengan context awareness.
- `interfaces.go`: Kontrak abstraksi dependency (`EnvironmentProvider`, `ProjectEnvironmentProvider`, `DeploymentCreator`, `SettingProvider`, `DeployEnqueuer`).
- `handler.go`: HTTP Handler untuk CRUD, start, stop, restart, deploy, env var, domain, dan limits aplikasi.
- `webhooks.go`: Handler webhook publik (GitHub, GitLab, Generic Git).
- `dropzone.go`: Magic Dropzone handler untuk instant upload `.zip` & deteksi stack runtime.
- `monitoring.go`: Handler SSE/JSON metrik CPU & RAM aplikasi.
- `module.go`: Module container & route registration.
- `applications_test.go`: Unit test untuk seluruh alur repository dan handler.

## Rute yang Didaftarkan

### Terproteksi (`RegisterRoutes`)
- `POST /lite/deploy/upload` - Upload file ZIP dropzone
- `GET /projects/{project_id}/environments/{env_id}/applications` - List aplikasi
- `POST /projects/{project_id}/environments/{env_id}/applications` - Buat aplikasi
- `GET /applications/{id}` - Detail aplikasi
- `POST /applications/{id}/start` - Menjalankan kontainer
- `POST /applications/{id}/stop` - Menghentikan kontainer
- `POST /applications/{id}/restart` - Restart kontainer
- `POST /applications/{id}/deploy` - Enqueue deployment baru
- `POST /applications/{id}/env-vars` - Tambah variabel lingkungan
- `POST /applications/{id}/env-vars/{var_id}/delete` - Hapus variabel lingkungan
- `POST /applications/{id}/domain` - Atur custom domain
- `POST /applications/{id}/domain/remove` - Hapus custom domain
- `POST /applications/{id}/resources` - Update CPU & Memory limit
- `POST /applications/{id}/webhook/regenerate` - Regenerasi secret webhook
- `GET /applications/{id}/status` - Polling status kontainer
- `GET /applications/{id}/stats` - Metrik utilisasi kontainer

### Publik (`RegisterPublicRoutes`)
- `POST /webhooks/deploy/{application_id}` - Universal webhook listener
- `POST /webhooks/github/{application_id}` - GitHub push webhook
- `POST /webhooks/gitlab/{application_id}` - GitLab push webhook
