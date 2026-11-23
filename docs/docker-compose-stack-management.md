# Docker Compose Multi-Service Stack Management

Dokumentasi ini menjelaskan arsitektur, siklus hidup, dan panduan penggunaan fitur **Docker Compose Multi-Service Stack Management** di **get-labuh**.

---

## 1. Arsitektur & Prinsip Kerja

Fitur Docker Compose di get-labuh memungkinkan pengelolaan aplikasi multi-container (misal: stack WordPress, LEMP/LAMP, aplikasi web dengan background worker, database, dan cache) dalam satu kesatuan lifecycle.

```
┌────────────────────────────────────────────────────────────────────────┐
│                   Web UI (Templ + HTMX + Alpine.js)                    │
│   - YAML Code Editor (Input, Syntax Validation, Save & Deploy)         │
│   - Service Breakdown Grid (Card per Sub-Service, Status, Ports)       │
│   - Per-service Actions (Restart Sub-service, Live Logs Modal)         │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ HTTP / HTMX
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                  Module `internal/modules/compose/`                    │
│   - Handler: YAML Validator, Deploy/Up, Stop/Down, Service Logs        │
│   - Repo: Simpan `raw_yaml` & metadata stack di database SQLite        │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │                                │
                    ▼                                ▼
┌──────────────────────────────┐  ┌──────────────────────────────────────┐
│  Go Compose AST Parser       │  │   EnvResolver (Cascading Priority)   │
│  (`internal/compose/parser`) │  │   - Hierarki: Project > Team         │
│  - Ekstrak: services, image, │  │   - Tulis `.env` ter-escape di       │
│    ports, volumes, depends_on│  │     direktori kerja stack            │
└──────────────┬───────────────┘  └──────────────────┬───────────────────┘
               │                                     │
               └──────────────────┬──────────────────┘
                                  ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   Docker Compose Orchestrator Engine                   │
│                       (`internal/compose/engine`)                      │
│   - Direktori kerja: `/var/lib/labuh/compose/{id}/`                    │
│     ├── docker-compose.yml (diregenerasi dari DB `raw_yaml`)           │
│     └── .env (diregenerasi dari EnvResolver)                           │
│   - Eksekusi Lifecycle:                                                │
│     `docker compose -p {slug} -f docker-compose.yml up -d --build`     │
│     `docker compose -p {slug} -f docker-compose.yml down`              │
│     `docker compose -p {slug} -f docker-compose.yml restart -- {svc}`  │
│   - Runtime Inspection via Docker SDK Client:                          │
│     Filter: `com.docker.compose.project={slug}`                        │
│     Filter: `com.docker.compose.service={service_name}`                │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Fitur Utama

### A. In-Dashboard YAML Editor & AST Parser
- **Single Source of Truth**: Definisi YAML disimpan langsung di kolom `raw_yaml` pada database SQLite (`compose_applications`).
- **Validasi Sintaks Instan**: Endpoint `/compose-apps/{id}/validate` memeriksa sintaks YAML via AST parser Go (`gopkg.in/yaml.v3`) sebelum disimpan, lengkap dengan sanitasi output HTML (bebas celah XSS).
- **Service Decomposition**: Parser secara otomatis membedah struktur:
  - Container Image / Build Context
  - Port Mappings (format string `80:80` atau map `published:target`)
  - Environment Variables (format list `KEY=VAL` atau map `KEY: VAL`)
  - Volume Bindings & Named Volumes
  - Dependency Graph (`depends_on`)

### B. Cascading Environment Variables Injection
Stack Compose otomatis mewarisi variabel lingkungan bersama tingkat **Project** dan **Team** melalui `EnvResolver`:
1. Sistem menghimpun seluruh variabel aktif dengan prioritas **`Project > Team`**.
2. Variabel ditulis ke file `.env` di direktori kerja stack (`/var/lib/labuh/compose/{id}/.env`).
3. Semua karakter sensitif diescape secara aman:
   - Tanda petik ganda (`"`) dan backslash (`\`) diescape.
   - Simbol dollar (`$`) diescape menjadi `$$` agar tidak dianggap interpolasi variabel kosong oleh Docker Compose.
4. Docker Compose menginterpolasi sintaks `${VAR_NAME}` di dalam `docker-compose.yml` secara otomatis.

### C. Services Breakdown Grid & Status Inspection
Setiap sub-service dalam stack ditampilkan dalam bentuk kartu interaktif:
- **Status Lampu**:
  - `● running`: Container berjalan aktif.
  - `● healthy`: Container memiliki Docker healthcheck dan berstatus sehat.
  - `▲ restarting`: Container sedang dalam proses restart/booting.
  - `✕ exited`: Container berhenti atau mengalami error.
  - `○ not created`: Service didefinisikan di YAML namun belum dibuat oleh Docker.
- **Port Badges**: Port publik dan port internal yang terpetakan.
- **Aksi Per-Service**:
  - **Restart**: Merestart hanya satu sub-service (misal hanya `web` atau hanya `worker`) tanpa menghentikan service database/redis di stack yang sama.
  - **Logs Modal**: Membaca streaming log stdout/stderr langsung dari container service yang dipilih.

---

## 3. Struktur Database (Migration `033_compose_raw_yaml.sql`)

```sql
ALTER TABLE compose_applications ADD COLUMN raw_yaml TEXT NOT NULL DEFAULT '';
ALTER TABLE compose_applications ADD COLUMN parsed_services TEXT NOT NULL DEFAULT '[]';
ALTER TABLE compose_applications ADD COLUMN last_deployed_at DATETIME;
ALTER TABLE compose_applications ADD COLUMN error_message TEXT;
```

---

## 4. Referensi Rute & Endpoint HTTP

| Method | Rute | Deskripsi |
|---|---|---|
| `GET` | `/projects/{project_id}/environments/{env_id}/compose-apps` | Daftar aplikasi Docker Compose per environment |
| `GET` | `/projects/{project_id}/environments/{env_id}/compose-apps/new` | Halaman form pembuatan compose app baru |
| `POST` | `/projects/{project_id}/environments/{env_id}/compose-apps` | Membuat stack compose baru |
| `GET` | `/compose-apps/{id}` | Halaman detail stack (Services Grid, YAML Editor, Domain) |
| `POST` | `/compose-apps/{id}/yaml` | Menyimpan perubahan `raw_yaml` dan parsing services |
| `POST` | `/compose-apps/{id}/validate` | Validasi sintaks YAML (HTMX response badge) |
| `POST` | `/compose-apps/{id}/deploy` | Eksekusi `docker compose up -d --build` |
| `POST` | `/compose-apps/{id}/stop` | Eksekusi `docker compose down` |
| `POST` | `/compose-apps/{id}/services/{service}/restart` | Restart satu sub-service spesifik |
| `GET` | `/compose-apps/{id}/services/{service}/logs` | Mengambil log runtime container sub-service |
| `POST` | `/compose-apps/{id}/domain` | Mendaftarkan custom domain dengan Caddy auto-HTTPS |
| `POST` | `/compose-apps/{id}/domain/remove` | Menghapus custom domain |

---

## 5. Contoh Definisi Docker Compose

Berikut contoh Docker Compose YAML multi-service yang memanfaatkan cascading environment variables:

```yaml
version: '3.8'

services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    environment:
      - APP_ENV=production
      - DATABASE_URL=${DATABASE_URL}
      - ORG_DOMAIN=${ORG_DOMAIN}
    volumes:
      - ./html:/usr/share/nginx/html
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    restart: unless-stopped

  worker:
    image: php:8.3-cli-alpine
    environment:
      - DATABASE_URL=${DATABASE_URL}
    depends_on:
      - redis
```

---

## 6. Standar Verifikasi & Keamanan

Fitur ini memenuhi standar kualitas dan keamanan get-labuh:
1. **Command Injection Prevention**: Semua perintah CLI dieksekusi melalui `exec.CommandContext("docker", args...)` dengan argumen terpisah dan pemisah flag `--` sebelum nama service.
2. **XSS Sanitization**: Validasi error dan nama service di-escape via `html.EscapeString`.
3. **Data Race Free**: Teruji 100% lulus melalui pengujian `go test -race ./...`.
4. **Zero-CGO**: Dikompilasi murni tanpa ketergantungan CGO (`CGO_ENABLED=0 go build ./cmd/labuh`).
