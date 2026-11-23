# Implementation Plan: Full-Fledged Docker Compose Stack Management

- **Spec**: `docs/superpowers/specs/2026-11-23-docker-compose-stack-management-design.md`
- **Sub-Project**: Advanced Container Orchestration (Fase 1: Docker Compose Stack)
- **Status**: Ready for execution

---

## Task 1: Database Migration `033_compose_raw_yaml.sql` & Domain Models
- **Goal**: Menambahkan kolom `raw_yaml`, `parsed_services`, `last_deployed_at`, dan `error_message` ke tabel `compose_applications`, serta memperbarui domain models.
- **Files**:
  - Create: `internal/database/migrations/033_compose_raw_yaml.sql`
  - Modify: `internal/models/compose.go`
  - Create: `internal/modules/compose/migration_test.go`
- **Verification**: `go test -v ./internal/modules/compose -run TestMigration033`

---

## Task 2: Go Docker Compose AST Parser & Validator
- **Goal**: Mengurai teks YAML Docker Compose menjadi struct `ComposeServiceDef` menggunakan `gopkg.in/yaml.v3` dengan validasi sintaks dan penanganan error yang presisi.
- **Files**:
  - Create: `internal/compose/parser.go`
  - Create: `internal/compose/parser_test.go`
- **Verification**: `go test -v -race ./internal/compose -run TestParseComposeYAML`

---

## Task 3: Compose Repository Persistence & Query Updates
- **Goal**: Memperbarui `Repo` di `internal/modules/compose/` untuk mendukung penyimpanan `raw_yaml`, update parsed services JSON, update status/error message, dan timestamp deployment.
- **Files**:
  - Modify: `internal/modules/compose/repo.go`
  - Modify: `internal/modules/compose/compose_test.go`
- **Verification**: `go test -v -race ./internal/modules/compose -run TestComposeRepo_RawYAML`

---

## Task 4: Docker Compose Orchestrator Engine
- **Goal**: Mengimplementasikan `Orchestrator` di `internal/compose/engine.go` yang mengelola penulisan direktori kerja (`docker-compose.yml` & `.env` dari `EnvResolver`), eksekusi CLI `docker compose` (`up`, `down`, `restart`), dan inspeksi status sub-service via Docker SDK label filter (`com.docker.compose.project` & `com.docker.compose.service`).
- **Files**:
  - Create: `internal/compose/engine.go`
  - Create: `internal/compose/engine_test.go`
- **Verification**: `go test -v -race ./internal/compose -run TestOrchestrator`

---

## Task 5: Compose Module Handlers & Route Integration
- **Goal**: Menambahkan endpoint HTTP di `internal/modules/compose/handler.go` dan `module.go` untuk update YAML, validasi sintaks YAML, eksekusi deploy, stop, restart sub-service, dan fetch log sub-service.
- **Files**:
  - Modify: `internal/modules/compose/handler.go`
  - Modify: `internal/modules/compose/module.go`
  - Modify: `internal/modules/compose/compose_test.go`
- **Verification**: `go test -v -race ./internal/modules/compose -run TestComposeHandler`

---

## Task 6: Rich Web UI Templ Component & Pages
- **Goal**: Membangun halaman detail stack `/compose-apps/{id}` (`detail.templ`) dengan 3 tab: Services Breakdown Grid (lampu status, ports, restart action), YAML Monospace Code Editor (dengan tombol validate & save/deploy), dan Runtime Log Console.
- **Files**:
  - Create: `internal/web/pages/compose/detail.templ`
  - Generate: `internal/web/pages/compose/detail_templ.go` (`templ generate`)
  - Modify: `internal/web/pages/compose/list.templ`
- **Verification**: `templ generate && go test -v ./internal/web/pages/compose/...`

---

## Task 7: End-to-End Integration Suite & Zero-CGO Verification
- **Goal**: Menulis pengujian integrasi E2E menyeluruh untuk alur lifecycle Docker Compose stack (Create -> Save YAML -> Parse Services -> Deploy -> Sub-service Action -> Down) dan verifikasi bebas race condition serta zero-CGO build.
- **Files**:
  - Create: `internal/testing/compose_stack_e2e_test.go`
- **Verification**: `go test -race ./... && CGO_ENABLED=0 go build -v -o /dev/null ./cmd/labuh`
