# KP-WIFI-MANAGEMENT

## Aplikasi Manajemen Akses Wi-Fi Member untuk Coworking Space

Aplikasi berbasis web untuk membantu pengelolaan akses Wi-Fi pada lingkungan coworking space, meliputi autentikasi pengguna, hak akses, durasi penggunaan, sesi koneksi, Guest Wi-Fi, pencatatan penggunaan, audit log, dan monitoring perangkat jaringan.

## Tujuan

Sistem dibuat untuk membantu administrator mengelola pengguna dan akses Wi-Fi secara terstruktur serta menyediakan dashboard berbeda sesuai peran pengguna.

## Role Pengguna

### Admin
Admin memiliki akses untuk:
- Login dan autentikasi sistem
- Mengelola data member
- Mengatur hak akses member
- Mengatur periode akses member
- Mengelola konfigurasi Guest Wi-Fi
- Melihat penggunaan Wi-Fi
- Melihat audit log
- Mengelola akun teknisi
- Mengelola perangkat jaringan

### Member
Member memiliki akses untuk:
- Login
- Melihat profil akun
- Melihat kode member
- Melihat status akun
- Melihat status dan periode hak akses
- Membuat sesi penggunaan
- Melihat sesi aktif
- Mengakhiri sesi
- Melihat riwayat penggunaan

### Guest
Guest menggunakan alur terpisah dari akun member:
- Menggunakan konfigurasi Guest Wi-Fi
- Membuat sesi guest
- Memiliki batas durasi akses
- Sesi yang melewati batas waktu dapat berubah menjadi EXPIRED
- Data sesi guest dapat dicatat oleh sistem

### Teknisi
Teknisi memiliki akses khusus untuk:
- Melihat dashboard perangkat
- Melihat status perangkat
- Menjalankan monitoring perangkat
- Melihat informasi perangkat jaringan
- Mengelola aktivitas teknis sesuai hak akses yang diberikan

Teknisi tidak memiliki hak akses Admin.

## Fitur Sistem

### 1. Autentikasi
Sistem menggunakan:
- Username dan password
- Session berbasis cookie
- Password hashing menggunakan Argon2id
- Role-based access control

### 2. Hak Akses Member
Admin dapat menentukan:
- Waktu mulai akses
- Waktu berakhir akses
- Status akses member

Member dapat melihat status hak akses dari dashboard masing-masing.

### 3. Guest Wi-Fi
Admin dapat mengatur:
- SSID
- Network type
- Durasi akses guest
- Status konfigurasi

Password Guest Wi-Fi disimpan dalam bentuk terenkripsi pada sistem.

Sistem juga memiliki mekanisme otomatis untuk mengubah sesi guest yang sudah melewati batas waktu menjadi EXPIRED.

### 4. Session Management
Sistem mencatat sesi penggunaan Wi-Fi, termasuk:
- User atau guest
- Waktu mulai
- Waktu selesai
- Status sesi
- IP client jika tersedia
- MAC client jika tersedia
- Perangkat yang terkait jika tersedia

### 5. Usage Log
Sistem menyediakan struktur pencatatan:
- Upload bytes
- Download bytes
- Total bytes
- Waktu pencatatan
- Session ID

Saat ini API usage sudah tersedia untuk menerima data penggunaan. Pengambilan traffic jaringan secara langsung dari perangkat jaringan masih menjadi tahap integrasi berikutnya.

### 6. Audit Log
Aktivitas administrasi penting dicatat dalam audit log, antara lain:
- Pembuatan akun teknisi
- Penghapusan akun teknisi
- Perubahan hak akses member
- Perubahan konfigurasi Guest Wi-Fi

Audit log menyimpan informasi aktivitas, target, deskripsi, user, alamat IP, dan waktu aktivitas.

### 7. Monitoring Perangkat
Teknisi dapat melihat:
- Total perangkat
- Status ONLINE
- Status OFFLINE
- Status UNKNOWN
- Informasi perangkat

Sistem memiliki proses monitoring berkala terhadap perangkat yang terdaftar.

Integrasi monitoring dengan perangkat jaringan fisik seperti MikroTik masih menjadi tahap pengembangan/integrasi lanjutan.

## Arsitektur

```text
Browser
   |
   v
Frontend HTML / CSS / JavaScript
   |
   v
Go HTTP Backend
   |
   +---- Authentication & RBAC
   |
   +---- Member Management
   |
   +---- Guest Wi-Fi
   |
   +---- Session Management
   |
   +---- Usage Log
   |
   +---- Audit Log
   |
   +---- Device Monitoring
   |
   v
SQLite Database


## Teknologi

- Go
- HTML
- CSS
- JavaScript
- SQLite
- Argon2id
- AES-GCM
- REST API
- Docker
- Docker Compose

## Struktur Project

```text
KP-WIFI-MANAGEMENT/
├── backend/
│   ├── cmd/
│   ├── database/
│   ├── handlers/
│   ├── middleware/
│   ├── services/
│   ├── go.mod
│   ├── go.sum
│   └── main.go
├── frontend/
│   ├── admin/
│   ├── guest/
│   ├── member/
│   ├── technician/
│   └── index.html
├── deployment/
│   └── docker-compose.yml
├── database.sql
├── Dockerfile
├── README.md
└── .gitignore

## Menjalankan Secara Lokal

Masuk ke folder backend:

```bash
cd backend
```

Jalankan aplikasi:

```bash
go run .
```


Aplikasi berjalan pada http://localhost:8080.

Health check tersedia pada http://localhost:8080/api/health.

## Docker

Project menyediakan Dockerfile dan Docker Compose untuk kebutuhan deployment.

## Keamanan

Sistem menerapkan hashing Argon2id untuk password user, enkripsi AES-GCM untuk password Guest Wi-Fi, session cookie untuk autentikasi, pembatasan akses berdasarkan role, dan Audit Log untuk aktivitas penting Admin.

## Status Implementasi

### Sudah tersedia

- [x] Login dan logout
- [x] Role ADMIN
- [x] Role MEMBER
- [x] Role TECHNICIAN
- [x] Manajemen member
- [x] Hak akses member
- [x] Session member
- [x] Guest Wi-Fi configuration
- [x] Guest session dan expiry
- [x] Audit Log
- [x] Monitoring perangkat dasar
- [x] Riwayat penggunaan Wi-Fi
- [x] Dockerfile
- [x] Docker Compose

### Pengembangan lanjutan

- [ ] Integrasi langsung dengan MikroTik/gateway nyata
- [ ] Pengambilan traffic upload/download otomatis dari gateway
- [ ] Enforcement akses Wi-Fi melalui gateway/RADIUS
- [ ] Monitoring perangkat menggunakan API/SNMP/ICMP sesuai perangkat
- [ ] Deployment ke server/cloud production

Fitur yang belum terhubung dengan perangkat jaringan nyata tidak dianggap sebagai data monitoring nyata.

## Catatan

Project ini dikembangkan sebagai aplikasi Kerja Praktik untuk manajemen akses Wi-Fi pada lingkungan coworking space.

Database yang digunakan selama pengembangan adalah SQLite.

Integrasi perangkat jaringan seperti MikroTik memerlukan konfigurasi perangkat, alamat IP, kredensial/API, serta metode autentikasi yang sesuai.

## Repository

Branch utama: main

Repository: KP-WIFI-MANAGEMENT
