# Design Specification: Full-Fledged Docker Compose Stack Management

- **Status**: Approved
- **Author**: Antigravity & Faiq
- **Date**: 2026-11-23
- **Sub-Project**: Advanced Container Orchestration (Fase 1: Docker Compose Stack)

---

## 1. Background & Problem Statement

Saat ini di `get-labuh`, dukungan aplikasi multi-container Docker Compose (`internal/modules/compose/`) memiliki keterbatasan:
1. `compose_applications` hanya menyimpan `ComposeFilePath` (file lokal di server). Pengguna tidak dapat menulis, mengedit, atau meninjau YAML langsung di antarmuka web dashboard.
2. Belum ada parser Go AST untuk mengurai sub-service yang didefinisikan dalam YAML (`web`, `db`, `redis`, `worker`).
3. Endpoint lifecycle (`Deploy`, `Start`, `Stop`) di `internal/modules/compose/handler.go` hanya memperbarui status di database SQLite tanpa menjalankan proses `docker compose` nyata ke Docker engine.
4. Belum ada integrasi dengan `EnvResolver` kita untuk menyuntikkan variabel lingkungan bersama (Project & Team) ke dalam file `.env` compose stack.
5. Belum ada inspeksi status container individu per sub-service via Docker SDK (misal: apakah service `redis` sedang running atau crash saat service `web` booting).

Spesifikasi ini menetapkan desain menyeluruh untuk mentransformasikan manajemen Docker Compose di `get-labuh` menjadi setara dengan fitur multi-service stack Coolify, mempertahankan arsitektur single-binary Go dan standar zero-CGO.

---

## 2. Architecture & Design Principles

```
┌────────────────────────────────────────────────────────────────────────┐
│                   Web UI (Templ + HTMX + Alpine.js)                    │
│   - YAML Code Editor (Ace/Monospace, Syntax Validation, Save & Deploy) │
│   - Service Decomposition Grid (Card per Sub-Service & Status)         │
│   - Sub-Service Actions (Restart, Stop, Logs per Service)              │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ HTTP / HTMX
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   Module `internal/modules/compose/`                   │
│   - Handler: YAML editor API, Sub-service Action, Live Stream Logs     │
│   - Repo: CRUD `compose_applications` dengan `raw_yaml`                │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │                                │
                    ▼                                ▼
┌──────────────────────────────┐  ┌──────────────────────────────────────┐
│  Go Compose AST Parser       │  │   EnvResolver (Cascading Priority)   │
│  (`internal/compose/parser`) │  │   - Hierarki: Compose > Project >    │
│  - Ekstrak: services, image, │  │     Team                             │
│    ports, volumes, depends_on│  │   - Write `.env` otomatis di runtime │
└──────────────┬───────────────┘  └──────────────────┬───────────────────┘
               │                                     │
               └──────────────────┬──────────────────┘
                                  ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   Docker Compose Orchestrator Engine                   │
│                       (`internal/compose/engine`)                      │
│   - Direktori kerja: `/var/lib/labuh/compose/{id}/`                    │
│     ├── docker-compose.yml (diregenerasi dari DB)                      │
│     └── .env (diregenerasi dari EnvResolver)                           │
│   - Eksekusi: CLI Wrapper `docker compose -p {slug} up -d --build`     │
│   - Inspeksi: Docker SDK Client (`client.ContainerList` filter label)  │
│     `com.docker.compose.project == {slug}`                             │
│     `com.docker.compose.service == {service_name}`                     │
└────────────────────────────────────────────────────────────────────────┘
```

### Prinsip Utama
1. **Opsi A (CLI Execution + Docker SDK Inspection)**:
   - Eksekusi lifecycle (`up`, `down`, `restart`) menggunakan CLI `docker compose` melalui `exec.CommandContext` dengan logging terisolasi.
   - Query runtime status, resource inspection, dan log streaming per-service menggunakan native Docker SDK Client (`docker.Client`) untuk performa tinggi tanpa spawning subprocess berlebih.
2. **Database Single-Source of Truth**:
   - YAML disimpan di kolom `raw_yaml` pada database SQLite. File di disk server (`/var/lib/labuh/compose/{id}/docker-compose.yml`) diregenerasi secara deterministik saat deployment.
3. **Cascading Environment Variable Injection**:
   - Sebelum `docker compose up` dijalankan, `EnvResolver` dipanggil untuk menghasilkan daftar key-value final (Project & Team variables). Nilai tersebut ditulis ke file `.env` di direktori kerja stack sehingga Docker Compose secara otomatis menginterpolasi `${VAR_NAME}`.
4. **Resilient Status & Service Correlation**:
   - Status sub-service dikorelasikan antara definisi statis (hasil parse YAML) dengan container runtime Docker via label `com.docker.compose.service`. Jika suatu service belum dibuat atau crash, UI menampilkannya dengan jelas.

---

## 3. Data Model & Database Migration

### Migration `033_compose_raw_yaml.sql`
```sql
ALTER TABLE compose_applications ADD COLUMN raw_yaml TEXT NOT NULL DEFAULT '';
ALTER TABLE compose_applications ADD COLUMN parsed_services TEXT NOT NULL DEFAULT '[]';
ALTER TABLE compose_applications ADD COLUMN last_deployed_at DATETIME;
ALTER TABLE compose_applications ADD COLUMN error_message TEXT;
```

### Struct `models.ComposeApplication` (`internal/models/compose.go`)
```go
type ComposeApplication struct {
	ID                 string
	EnvironmentID      string
	Name               string
	Slug               string
	ComposeFilePath    string
	ComposeProjectName *string
	CustomDomain       *string
	RawYAML            string
	ParsedServices     []ComposeServiceDef
	LastDeployedAt     *time.Time
	ErrorMessage       *string
	Status             ComposeStatus
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ComposeServiceDef struct {
	Name        string            `json:"name"`
	Image       string            `json:"image,omitempty"`
	BuildPath   string            `json:"build_path,omitempty"`
	Ports       []string          `json:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
}

type ComposeServiceRuntime struct {
	Name        string            `json:"name"`
	Definition  ComposeServiceDef `json:"definition"`
	ContainerID string            `json:"container_id,omitempty"`
	Status      string            `json:"status"` // "running", "exited", "restarting", "not_created"
	Health      string            `json:"health,omitempty"` // "healthy", "unhealthy", "starting"
	State       string            `json:"state,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
}
```

---

## 4. Go Compose AST Parser (`internal/compose/parser.go`)

Fungsi parser mengurai YAML menggunakan `gopkg.in/yaml.v3`:
1. `ParseComposeYAML(rawYAML string) ([]models.ComposeServiceDef, error)`:
   - Memeriksa struktur dasar map: wajib memiliki key `services:`.
   - Mengiterasi setiap sub-key di bawah `services:` dan memetakan field `image`, `build`, `ports`, `environment`, `volumes`, `depends_on`.
   - Mengembalikan error format yang jelas (misal: `"missing 'services' section"` atau `"invalid yaml at line X"`).
2. `ValidateComposeYAML(rawYAML string) error`:
   - Validasi cepat untuk endpoint HTTP `/compose-apps/{id}/validate`.

---

## 5. Compose Orchestrator Engine (`internal/compose/engine.go`)

Engine bertanggung jawab mengelola eksekusi dan inspeksi:

```go
type Orchestrator interface {
    PrepareWorkDir(ctx context.Context, app *models.ComposeApplication, envVars []models.AppEnvVar) (string, error)
    Up(ctx context.Context, app *models.ComposeApplication, envVars []models.AppEnvVar) error
    Down(ctx context.Context, app *models.ComposeApplication) error
    RestartService(ctx context.Context, app *models.ComposeApplication, serviceName string) error
    InspectServices(ctx context.Context, app *models.ComposeApplication) ([]models.ComposeServiceRuntime, error)
    GetServiceLogs(ctx context.Context, app *models.ComposeApplication, serviceName string, tail int) (string, error)
}
```

- **Direktori Kerja**: `/var/lib/labuh/compose/{id}/` (fallback ke `/tmp/labuh-compose/{id}/` jika running di test/development environment).
- **Log Deployment**: `/tmp/labuh-logs/compose-{id}.log`.
- **Label Selector Docker SDK**:
  - `label=com.docker.compose.project={project_name}`
  - `label=com.docker.compose.service={service_name}`

---

## 6. Handler & Routing (`internal/modules/compose/`)

Route tambahan di `internal/modules/compose/module.go`:
- `GET /compose-apps/{id}`: Menampilkan halaman detail dengan 3 tab (Services Grid, YAML Editor, Runtime Logs).
- `POST /compose-apps/{id}/yaml`: Menyimpan perubahan `raw_yaml` dan mengurai `parsed_services`.
- `POST /compose-apps/{id}/validate`: Validasi sintaks YAML (HTMX out-of-band badge/alert).
- `POST /compose-apps/{id}/deploy`: Menulis file, resolve cascading env, dan memicu `docker compose up -d --build`.
- `POST /compose-apps/{id}/stop`: Menghentikan seluruh stack (`docker compose down`).
- `POST /compose-apps/{id}/services/{service}/restart`: Merestart satu sub-service tertentu.
- `GET /compose-apps/{id}/services/{service}/logs`: Membaca runtime log dari sub-service tertentu.

---

## 7. Web UI Design (Templ + Alpine.js)

File `internal/web/pages/compose/detail.templ`:
1. **Header**: Nama stack, slug, environment badge, status badge (`idle`, `building`, `running`, `failed`, `stopped`), dan tombol aksi utama (*Deploy*, *Stop*, *Redeploy*).
2. **Tab 1: Services Grid**:
   - Card per sub-service menampilkan:
     - Nama service (misal: `web`, `postgres`, `redis`).
     - Image / Build context.
     - Port bindings (misal: `0.0.0.0:8080->80/tcp`).
     - Status lampu (Hijau: Running/Healthy, Kuning: Restarting, Merah: Exited/Unhealthy, Abu-abu: Not Created).
     - Tombol cepat: *Restart Service*, *View Log Modal*.
3. **Tab 2: YAML Editor**:
   - Textarea dengan font monospace, line numbers, dan status validasi interaktif.
   - Tombol *Validate Syntax* dan *Save & Redeploy Stack*.
4. **Tab 3: Stack Logs**:
   - Console terminal hitam dengan auto-scroll dan filter per sub-service.

---

## 8. Testing & Verification Plan

1. **Unit Tests**:
   - `internal/compose/parser_test.go`: Uji parsing YAML valid (multi-service, port arrays, environment maps, volumes) dan penolakan YAML invalid/malformed.
   - `internal/modules/compose/repo_test.go`: Uji simpan dan baca `raw_yaml`, `last_deployed_at`, dan `error_message`.
2. **Integration Tests**:
   - `internal/compose/engine_test.go`: Uji `PrepareWorkDir`, penulisan `.env`, inspeksi container via mock Docker client, dan eksekusi command compose.
   - `internal/modules/compose/compose_test.go`: Uji endpoint HTTP handler (Save YAML, Validate, Deploy, Service Restart).
3. **Full Suite & Zero-CGO**:
   - `go test -race ./...`: Seluruh suite bebas data race.
   - `CGO_ENABLED=0 go build -v -o /dev/null ./cmd/labuh`: Kompilasi binary Go bersih.
