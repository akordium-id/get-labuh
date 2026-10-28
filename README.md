# get-labuh

[![License](https://img.shields.io/badge/License-Apache2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://www.docker.com/)

> [!WARNING]
> **Proyek ini sedang dalam pengembangan aktif.** Antarmuka pengguna, fungsionalitas, dan struktur data dapat berubah secara signifikan di masa mendatang.

**Get-Labuh** adalah platform self-hosted PaaS sumber terbuka yang dirancang sebagai alternatif ultra-ringan (single Go binary) untuk menyederhanakan proses deployment aplikasi dan database ke server sendiri. 

Proyek ini terinspirasi oleh platform hebat [Coolify](https://coolify.io) dengan fokus pada performa tinggi, jejak memori yang sangat kecil (~30-50MB), dan kesederhanaan operasional.

## 🚀 Fitur Utama

- **Deployment Otomatis**: Siapkan server Anda sekali dan deployment dapat dilakukan hanya dengan beberapa klik.
- **Manajemen Server**: Tambahkan, kelola, dan amankan server Anda melalui dasbor terpusat.
- **Portal Layanan**: Hubungkan ke layanan yang sudah ada atau deploy layanan baru dengan mudah.
- **Integrasi Docker**: Pemanfaatan penuh teknologi kontainer untuk konsistensi dan skalabilitas.

## 🔧 Persiapan

### Prasyarat
Pastikan Anda telah menginstal perangkat lunak berikut:
- **Docker Desktop** (Versi 4.37 atau lebih baru)
- **Go** (Versi 1.26 atau lebih baru)

### Instalasi dan Setup
1. **Clone Repositori**
   ```bash
   git clone <repository-url>
   cd get-labuh
   ```

2. **Setup Database**
   Proyek ini menggunakan SQLite. Anda dapat mengatur database dengan menjalankan perintah berikut:
   ```bash
   go run cmd/getlabuh/main.go initdb
   ```

3. **Jalankan Aplikasi**
   ```bash
   go run cmd/getlabuh/main.go run
   ```
   Aplikasi akan berjalan pada `http://localhost:8080`.

### Menjalankan via Docker
Anda juga dapat menjalankan aplikasi menggunakan Docker:
```bash
docker run --rm -p 8080:8080 -v $(pwd):/app getlabuh:latest
```

## 📖 Penggunaan

Setelah aplikasi berjalan, akses dasbor admin di:
```
http://localhost:8080/admin
```

### Konfigurasi Awal
- **Email**: [EMAIL_ADDRESS]`
- **Password**: `admin`

Anda akan diarahkan untuk mengubah kata sandi default saat login pertama kali.

### Menambah Server
1. Navigasi ke **Settings → Servers**.
2. Klik **Add Server**.
3. Masukkan detail server, termasuk Host, Port, dan User SSH.

### Layanan
1. Navigasi ke **Services**.
2. Anda dapat memilih **Create Service** untuk memulai instance baru atau mengelola layanan yang ada.

## 🛡️ Keamanan
- Semua koneksi harus menggunakan kredensial yang valid (Email/Password).
- Server Docker di-deploy dengan user non-root (`ubuntu`) untuk keamanan tambahan.

## 🤝 Berkontribusi

Kontribusi sangat diterima! Silakan ikuti langkah-langkah berikut:
1. **Fork** repositori.
2. Buat **Branch Fitur** (`git checkout -b feature/AmazingFeature`).
3. **Commit** perubahan Anda (`git commit -m 'Add some AmazingFeature'`).
4. **Push** ke branch (`git push origin feature/AmazingFeature`).
5. Buka **Pull Request**.

## 🙏 Apresiasi & Atribusi

Proyek ini terinspirasi secara mendalam oleh [Coolify](https://coolify.io) karya Andras Bacsai dan komunitas open source. Labuh dibangun sebagai eksplorasi arsitektur alternatif yang ultra-ringan (single Go binary) dengan filosofi kesederhanaan dan kemudahan self-hosting yang dirintis oleh Coolify.

## 📝 Lisensi

Proyek ini dirilis di bawah Lisensi MIT. Lihat file [LICENSE](LICENSE) untuk detail lebih lanjut.

## 📞 Hubungi

- **Email**: [EMAIL_ADDRESS]
- **GitHub**: [Link Repositori]
