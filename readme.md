# Inventra

Inventra adalah sistem manajemen inventori produk berbasis Go dengan alur CRUD **asinkron**: setiap perubahan data (create/update/delete) tidak langsung menyentuh database dari request HTTP, melainkan dicatat lebih dulu lalu diproses di belakang layar melalui message broker. Pendekatan ini dipilih untuk mensimulasikan kondisi dunia nyata di mana operasi tulis perlu tahan terhadap lonjakan trafik dan kegagalan sebagian komponen, tanpa risiko kehilangan data.

Proyek ini dibuat sebagai bahan uji coba/demo — mencakup backend API, worker pemroses, relay outbox, serta antarmuka web sederhana untuk mencoba alur CRUD-nya secara langsung.

## Arsitektur Singkat

```
Browser (webui)
   │  HTTP
   ▼
API Server (cmd/api) ──► PostgreSQL (products, operations, outbox_messages, users, sessions)
   │                              │
   │  tulis "outbox_messages"     │  dibaca oleh
   ▼                              ▼
Outbox Relay (cmd/relay) ──► RabbitMQ ──► Worker (cmd/worker) ──► PostgreSQL + Redis (cache)
```

- **API server** menerima request, menyimpan produk/perubahan sekaligus baris `operations` + `outbox_messages` dalam satu transaksi (pola *transactional outbox*), lalu langsung mengembalikan `operation_id` ke client.
- **Outbox relay** membaca `outbox_messages` yang belum terkirim dan mempublikasikannya ke RabbitMQ, menjamin pesan tidak hilang meski API atau broker sempat down.
- **Worker** mengonsumsi pesan dari RabbitMQ, mengeksekusi perubahan sebenarnya ke database, menginvalidasi cache Redis, lalu memperbarui status operasi menjadi `succeeded`/`failed`.
- **Web UI** (`internal/webui/static`) melakukan polling ke `/api/operations/{id}` untuk menampilkan status operasi secara real-time, lalu memuat ulang daftar produk begitu operasi selesai.

## Tech Stack

| Komponen        | Teknologi                                      |
|------------------|-------------------------------------------------|
| Bahasa           | Go 1.27                                          |
| Database         | PostgreSQL (via `pgx/v5`)                        |
| Cache            | Redis (via `go-redis/v9`)                        |
| Message broker   | RabbitMQ (via `amqp091-go`)                      |
| Auth             | Session cookie + hashing password Argon2id       |
| Frontend         | HTML/CSS/JS statis (tanpa framework), disajikan langsung dari binary Go |

## Struktur Proyek

```
├── cmd
│   ├── api          → HTTP API + serve web UI statis
│   ├── worker        → consumer RabbitMQ, eksekusi CRUD produk
│   ├── relay          → outbox relay (DB → RabbitMQ)
│   ├── createadmin  → CLI untuk membuat akun admin pertama
│   ├── cachecheck  → utilitas cek konektivitas Redis
│   └── mqdemo        → utilitas cek konektivitas RabbitMQ
├── internal
│   ├── auth           → hashing & verifikasi password (Argon2id)
│   ├── cache          → wrapper Redis
│   ├── config         → pembacaan environment variable
│   ├── database       → koneksi PostgreSQL (pgx pool)
│   ├── messaging      → deklarasi exchange/queue RabbitMQ
│   ├── operation      → status operasi asinkron
│   ├── product        → domain produk (create/update/delete/cache/repository)
│   ├── session        → login/logout/cek sesi
│   ├── user           → model & repository user
│   └── webui          → aset statis (HTML/CSS/JS)
├── migrations         → skema database (golang-migrate)
└── postman            → koleksi Postman untuk pengujian API
```

## Prasyarat

Pastikan sudah terpasang dan berjalan secara lokal:

- Go 1.27+
- PostgreSQL 14+ (database `inventra`, user `inventra_app`)
- Redis 6+
- RabbitMQ 3.x (vhost `inventra`, user sesuai `RABBITMQ_USER`)
- [golang-migrate](https://github.com/golang-migrate/migrate) untuk menjalankan migrasi

## Konfigurasi

Semua konfigurasi dibaca dari environment variable (lihat `internal/config/config.go`):

| Variable            | Wajib | Default                | Keterangan                                  |
|---------------------|:-----:|-------------------------|----------------------------------------------|
| `DB_PASSWORD`       | ✅    | –                        | Password user PostgreSQL                     |
| `RABBITMQ_PASSWORD` | ✅    | –                        | Password user RabbitMQ                       |
| `APP_NAME`          |       | `Inventra`               | Nama aplikasi (dipakai di endpoint `/health`) |
| `HTTP_ADDR`         |       | `127.0.0.1:8081`         | Alamat & port API server                      |
| `RABBITMQ_USER`     |       | `inventra_app`           | Username RabbitMQ                             |
| `RABBITMQ_VHOST`    |       | `inventra`                | Virtual host RabbitMQ                        |
| `SESSION_TTL_HOURS` |       | `24`                      | Masa berlaku sesi login                       |
| `COOKIE_SECURE`     |       | `true`                    | Set `false` jika testing tanpa HTTPS di localhost |
| `COOKIE_DOMAIN`     |       | *(kosong)*                | Domain cookie sesi                            |
| `REDIS_URL`         |       | `redis://127.0.0.1:6379/0` | Koneksi Redis                                |

Koneksi PostgreSQL host/port/dbname/user (`127.0.0.1:5432`, db `inventra`, user `inventra_app`) saat ini di-hardcode di `internal/database/postgres.go`, jadi cukup samakan environment lokal dengan nilai tersebut, atau sesuaikan filenya bila perlu.

## Langkah Setup

1. **Buat database & jalankan migrasi**

   ```bash
   createdb inventra
   migrate -path migrations -database "postgres://inventra_app:<DB_PASSWORD>@127.0.0.1:5432/inventra?sslmode=disable" up
   ```

2. **Set environment variable** (contoh untuk shell lokal)

   ```bash
   export DB_PASSWORD="isi_password_db"
   export RABBITMQ_PASSWORD="isi_password_rabbitmq"
   ```

3. **Buat akun admin pertama**

   Perintah ini interaktif (password diketik di terminal, tidak lewat argumen) untuk menghindari password tersimpan di history shell:

   ```bash
   go run ./cmd/createadmin -name "Fairuz Alda Perkasa" -email "admin@inventra.test"
   ```

   Saat diminta, masukkan password berikut (panjang minimum 15 karakter, sesuai aturan Argon2id di `internal/auth/password.go`):

   ```
   Password admin (15–128 karakter): akumencoba112233
   Ulangi password admin:            akumencoba112233
   ```

4. **Jalankan ketiga proses** (masing-masing di terminal terpisah)

   ```bash
   go run ./cmd/api      # API + web UI → http://127.0.0.1:8081
   go run ./cmd/relay    # outbox relay
   go run ./cmd/worker   # worker CRUD produk
   ```

   `cmd/relay` dan `cmd/worker` wajib jalan agar operasi create/update/delete produk benar-benar selesai — tanpa keduanya, operasi akan tetap berstatus `pending` di web UI.

5. **Buka web UI** di `http://127.0.0.1:8081` dan login dengan akun admin di atas.

## Kredensial Login (Testing)

| Field    | Nilai                     |
|----------|----------------------------|
| Nama     | Fairuz Alda Perkasa        |
| Email    | `admin@inventra.test`      |
| Password | `akumencoba112233`         |
| Role     | admin                       |

> Kredensial ini dibuat lewat `cmd/createadmin` sesuai langkah setup di atas — akun tidak otomatis ada begitu database baru dimigrasikan.

## Ringkasan Endpoint API

| Method | Path                     | Keterangan                                   |
|--------|---------------------------|-----------------------------------------------|
| GET    | `/health`                 | Health check dasar                            |
| GET    | `/ready`                  | Cek kesiapan (koneksi DB)                     |
| POST   | `/api/auth/login`         | Login, set cookie sesi                        |
| POST   | `/api/auth/logout`        | Logout                                        |
| GET    | `/api/auth/me`            | Info user yang sedang login                   |
| GET    | `/api/products`           | List produk (`limit`, `offset`)               |
| GET    | `/api/products/{id}`      | Detail produk (dengan Redis cache)            |
| POST   | `/api/products`           | Buat produk baru → mengembalikan `operation_id` |
| PUT    | `/api/products/{id}`      | Ubah produk (butuh `version` untuk optimistic concurrency) → `operation_id` |
| DELETE | `/api/products/{id}`      | Hapus produk (butuh query `version`) → `operation_id` |
| GET    | `/api/operations/{id}`    | Cek status operasi asinkron (`pending`/`succeeded`/`failed`) |

Semua endpoint `/api/products` dan `/api/operations` mengembalikan payload yang dibungkus dalam key `data`, misalnya `{"data": {...}}` atau `{"data": [...], "pagination": {...}}`.

Koleksi Postman untuk mencoba seluruh endpoint di atas tersedia di folder `postman/`.

## Catatan Pengujian

- Karena alur CRUD bersifat asinkron, perubahan pada tabel produk **tidak instan** — web UI melakukan polling ke `/api/operations/{id}` hingga statusnya `succeeded` atau `failed` sebelum me-refresh tabel. Pastikan `cmd/worker` dan `cmd/relay` berjalan, kalau tidak operasi akan tampak "menggantung" di status `pending`.
- `cmd/cachecheck` dan `cmd/mqdemo` disediakan sebagai utilitas cepat untuk memverifikasi konektivitas Redis dan RabbitMQ secara terpisah dari aplikasi utama, berguna saat proses debugging environment.
- Produk yang sudah dihapus tidak benar-benar hilang dari database (soft delete via kolom `deleted_at`), sehingga histori tetap tercatat di tabel `audit_logs`.