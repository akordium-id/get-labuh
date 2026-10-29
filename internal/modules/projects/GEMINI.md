# Module: Projects, Deploy Keys & Teams

Package `internal/modules/projects` — domain agregat untuk pengelolaan Project, Environment, SSH Deploy Keys, dan Team Access Management.

## Tanggung Jawab

- **`repo.go`**:
  - `ProjectRepo`: CRUD Project dan relasi Environment (`environments`). Menyediakan query pagination dan search.
  - `DeployKeyRepo`: Simpan dan ambil public/private SSH deploy key (terenkripsi AES-CFB) per project.
  - `TeamRepo`: Organisasi multi-user, team membership, dan role-based hierarchy (`Owner > Admin > Developer > Viewer`).
- **`handler.go`**:
  - View & action controller untuk `/projects`, `/projects/create`, `/projects/{id}`.
  - Handler SSH deploy key generation & listing per project.
  - Handler tim `/settings/teams`, detail anggota, dan manajemen role.
- **`module.go`**:
  - Inisialisasi dependency `New(db, userProvider)`.
  - Registrasi rute `RegisterRoutes(r chi.Router)`.

## Rules untuk AI Agent

1. Selalu sertakan `context.Context` untuk semua query ke database (gunakan `*WithContext`).
2. Private key SSH pada `DeployKeyRepo` WAJIB selalu disimpan dalam bentuk terenkripsi (`PrivateKeyEncrypted`).
3. Komunikasi lintas modul (seperti user list untuk halaman team) harus melalui abstraction interface (`UserProvider`).
