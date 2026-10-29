# Module: Authentication & Sessions

Package `internal/modules/auth` — autentikasi pengguna, manajemen sesi login cookie, penguncian akun (brute-force protection), dan API Key management.

## Tanggung Jawab

- **`repo.go`**: CRUD dan query untuk tabel `users`, `sessions`, dan `api_keys` (dengan context passing). Token generation dan hashing SHA-256.
- **`middleware.go`**: Middleware `RequireAuth` untuk memvalidasi cookie sesi dan menginjeksi model `*models.User` ke request context (`ContextWithUser`).
- **`handler.go`**: Halaman login Templ, aksi form login/logout, serta CRUD token API Key pengguna.
- **`module.go`**: Factory `New(db, secure)` dan registrasi rute `RegisterPublicRoutes` dan `RegisterProtectedRoutes`.

## Konstanta Keamanan

- `MaxFailedLogins = 5` (Kunci akun selama 15 menit jika gagal 5x berturut-turut).
- `LockoutDuration = 15 * time.Minute`.
- `MaxConcurrentSessions = 3` (Auto-evict sesi terlama jika melebihi batas).

## Rules untuk AI Agent

1. Selalu gunakan `r.Context()` pada pemanggilan repo untuk memastikan tracing/cancellation berjalan.
2. Jangan pernah menyimpan password mentah — selalu hash menggunakan bcrypt.
3. Cookie sesi wajib berlabel `HttpOnly` dan `SameSiteLaxMode`.
