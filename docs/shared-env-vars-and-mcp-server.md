# Shared Environment Variables & Native Model Context Protocol (MCP) Server

Dokumentasi ini menjelaskan arsitektur pewarisan variabel lingkungan bersama (*cascading shared environment variables*), proteksi enkripsi AES-256 GCM, serta panduan integrasi **Native Go MCP Server** untuk mengontrol **get-labuh** melalui agen AI (Claude Desktop, Cursor, Antigravity, dsb).

---

## 1. Cascading Shared Environment Variables

Fitur Shared Environment Variables memungkinkan pengelolaan konfigurasi bersama di level **Project** maupun **Team** tanpa perlu menduplikasi variabel pada setiap aplikasi individual.

### Hirarki & Urutan Prioritas Resolusi

Resolusi variabel lingkungan dieksekusi oleh mesin `EnvResolver` (`internal/deploy/env_resolver.go`) dengan urutan prioritas deterministik:

```
[ Team Scope Variables ]  (Paling Umum: mis. ORGANISATION_NAME, SENTRY_DSN)
           │
           ▼ (Ditimpa oleh)
[ Project Scope Variables ]  (Spesifik Project: mis. DATABASE_URL, REDIS_HOST)
           │
           ▼ (Ditimpa oleh)
[ Application Scope Variables ]  (Spesifik App: mis. PORT, APP_NAME, CACHE_PREFIX)
           │
           ▼
[ Final Environment Variables ] ──▶ Diinjeksikan ke Docker Container saat Build & Deploy
```

1. **Team Scope**: Diterapkan ke seluruh project di bawah tim yang sama.
2. **Project Scope**: Diterapkan ke seluruh aplikasi di dalam project terkait (menimpa key yang sama dari Team).
3. **Application Scope**: Variabel spesifik aplikasi (menimpa key yang sama dari Project maupun Team).

### Keamanan & Enkripsi (AES-256 GCM)
- Nilai variabel yang ditandai sebagai `is_secret = true` dienkripsi secara otomatis menggunakan **AES-256 GCM** (`internal/encryption`) sebelum disimpan ke SQLite (`shared_env_vars`).
- Tersimpan dalam format bertanda `ENC:...`.
- Bersifat *fail-safe*: Jika kunci enkripsi tidak cocok atau dekripsi gagal, sistem mengembalikan error eksplisit dan membatalkan deployment untuk mencegah bocornya ciphertext mentah ke dalam runtime container.

### Manajemen via Web UI & Bulk Import
- Akses melalui panel project: `/projects/{id}/env`.
- Mendukung penambahan satu per satu atau **Bulk Import** dengan format teks `.env` standar:
  ```env
  # Komentar diabaikan
  DB_HOST=postgres.internal
  DB_PORT=5432
  API_SECRET=rahasia-besar
  ```
- Variabel rahasia dimaskir di antarmuka web (`••••••••••••••••`) untuk menghindari bahaya bahu mengintip (*shoulder surfing*).

---

## 2. Native Go MCP Server (`mark3labs/mcp-go`)

**get-labuh** menyertakan server bawaan berspesifikasi **Model Context Protocol (MCP)** v0.12.0 yang memungkinkan asisten AI mengelola lifecycle infrastruktur secara mandiri.

### Arsitektur Dual Transport

Server MCP get-labuh mendukung dua mode transportasi:

```
┌────────────────────────────────────────────────────────┐
│                   AI Assistant Client                  │
│       (Claude Desktop / Cursor / Antigravity CLI)      │
└───────────────┬────────────────────────┬───────────────┘
                │                        │
       [Transport 1: Stdio]      [Transport 2: HTTP SSE]
                │                        │
                ▼                        ▼
       `labuh mcp` CLI          POST /mcp/message  (Tool call)
       (Local process pipe)     GET  /mcp/sse      (Event stream)
                │                        │
                │             [RequireAPIKey Middleware]
                │             [CSRF Bypass Protection]
                │                        │
                └───────────┬────────────┘
                            ▼
              ┌───────────────────────────┐
              │    Labuh MCP Controller   │
              │  (16 Core Ops Tools)      │
              └─────────────┬─────────────┘
                            ▼
        ┌───────────────────┴───────────────────┐
        │ Docker Client / SQLite / DeployWorker │
        └───────────────────────────────────────┘
```

1. **Stdio JSON-RPC (`labuh mcp`)**:
   Dijalankan sebagai subproses lokal di workstation pengembang atau server. Sangat cocok untuk koneksi langsung CLI dan tool desktop.
2. **HTTP Server-Sent Events (SSE) (`/mcp`)**:
   Terpasang di router Chi HTTP. Memungkinkan agen jarak jauh terhubung melalui endpoint web yang aman:
   - Autentikasi: Wajib header `Authorization: Bearer <API_KEY>`.
   - CSRF Bypass: Rute `/mcp` dikecualikan dari `CSRFProtectionMiddleware` agar request POST JSON-RPC agen tidak ditolak.

### Kebijakan Keamanan Masking Rahasia

Secara default, respon JSON dari MCP server memaskir seluruh data sensitif:
- Variabel bertipe `is_secret == true` dimaskir menjadi `"••••••••"`.
- Kredensial internal seperti `GitToken`, `WebhookSecret`, `BasicAuthPassword`, dan password database dimaskir menjadi `"••••••••"`.
- Untuk melihat nilai plaintext (hanya jika agen benar-benar membutuhkan untuk debugging langsung), agen dapat mengirim argumen `show_secrets: true` secara eksplisit pada tool env.

---

## 3. Daftar 16 Core MCP Tools

| Kategori | Nama Tool | Deskripsi & Parameter Kunci |
|---|---|---|
| **Applications** | `list_applications` | Menampilkan seluruh aplikasi, slug, status, dan port. |
| | `get_application` | Detail lengkap aplikasi berdasarkan `id` (token rahasia dimaskir). |
| | `deploy_application` | Memicu deployment aplikasi berdasarkan `id` ke antrean worker. |
| | `get_deployment_status` | Memeriksa status log/tahapan deployment berdasarkan `deployment_id`. |
| | `get_application_logs` | Mengambil log runtime container (`id`, `tail` maks 2000 baris). |
| | `rollback_application` | Melakukan instant rollback aplikasi (`id`) ke deployment sukses sebelumnya. |
| | `restart_application` | Merestart container aplikasi yang sedang berjalan (`id`). |
| | `stop_application` | Menghentikan container aplikasi (`id`). |
| **Environment** | `get_application_env` | Melihat variabel aplikasi (`id`, `include_inherited`, `show_secrets`). |
| | `set_application_env` | Menyimpan variabel aplikasi (`id`, `key`, `value`, `is_secret`). |
| | `get_shared_env` | Melihat variabel bersama (`scope`: 'project'/'team', `scope_id`, `show_secrets`). |
| | `set_shared_env` | Menyimpan variabel bersama (`scope`, `scope_id`, `key`, `value`, `is_secret`). |
| **Databases** | `list_databases` | Daftar container database, port, dan status (`environment_id` opsional). |
| | `backup_database` | Memicu proses backup snapshot database on-demand (`id`). |
| **Server & SRE** | `get_server_metrics` | Metrik utilisasi host (CPU, RAM, Disk, Load Average). |
| | `list_system_alerts` | Memeriksa apakah utilisasi sistem melebihi ambang batas darurat. |

---

## 4. Panduan Konfigurasi Klien AI

### A. Claude Desktop

Tambahkan konfigurasi berikut ke file konfigurasi Claude Desktop (`~/.config/Claude/claude_desktop_config.json` di Linux atau `~/Library/Application Support/Claude/claude_desktop_config.json` di macOS):

#### Opsi 1: Stdio Transport (Local Binary)
```json
{
  "mcpServers": {
    "get-labuh": {
      "command": "/usr/local/bin/labuh",
      "args": ["mcp"],
      "env": {
        "LABUH_DB_PATH": "/var/lib/labuh/labuh.db",
        "LABUH_ENCRYPTION_KEY": "your-32-byte-hex-encryption-key"
      }
    }
  }
}
```

#### Opsi 2: HTTP SSE Transport (Remote / Dockerized Labuh)
```json
{
  "mcpServers": {
    "get-labuh": {
      "url": "https://labuh.yourdomain.com/mcp/sse",
      "headers": {
        "Authorization": "Bearer lba_your_api_key_here"
      }
    }
  }
}
```

---

### B. Cursor Editor (`.cursor/mcp.json`)

Di root repository atau direktori kerja Anda:

```json
{
  "mcpServers": {
    "labuh-local": {
      "command": "labuh",
      "args": ["mcp"]
    }
  }
}
```

---

### C. Antigravity CLI / agy

Bisa dipasang sebagai MCP Server di settings `~/.gemini/antigravity-cli/mcp/labuh/` atau dideklarasikan via HTTP SSE:

```json
{
  "mcpServers": {
    "labuh": {
      "url": "http://127.0.0.1:8080/mcp/sse",
      "headers": {
        "Authorization": "Bearer secret-api-key"
      }
    }
  }
}
```

---

## 5. Ringkasan Pengujian & Verifikasi

Fitur ini dijamin dengan standar ketat get-labuh:
- **Zero Data Race**: Terverifikasi bersih melalui `go test -race ./...`.
- **Zero-CGO**: Dikompilasi murni dengan `CGO_ENABLED=0 go build ./cmd/labuh`.
- **E2E Integration**: Diuji otomatis dalam suite `internal/testing/shared_env_and_mcp_e2e_test.go`.
