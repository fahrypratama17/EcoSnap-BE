# Runbook: migration dan data demo ke Supabase development

Dijalankan oleh pemilik repo. AI tidak membaca credential dan tidak mengeksekusi ke Supabase. Laporan pemilik 2026-11-21 pukul 17:37: migration `00010_create_gamification.sql` berhasil dan `goose status` menunjukkan seluruh migration `00001`–`00010` Applied.

## 0. Sebelum mulai

- Pastikan project yang dipakai adalah **development**, bukan production.
- Ambil connection string **direct** (port 5432) atau **session pooler** dari Supabase → Project Settings → Database. Jangan pakai transaction pooler (port 6543) untuk migration.
- Untuk server aplikasi juga pakai 5432/session pooler. Jika terpaksa 6543, backend otomatis beralih ke mode query tanpa prepared statement (sedikit lebih lambat); migration tetap wajib lewat 5432.
- Simpan di environment shell saja, jangan di file yang di-commit:

```powershell
$env:DATABASE_URL = "postgresql://postgres:<password>@<host>:5432/postgres?sslmode=require"
```

- Pull branch `dev` terbaru dan pastikan tes lokal PASS (`./scripts/test-postgres-portable.ps1 …` atau CI hijau).

## 1. Cek status Goose

```powershell
goose -dir database/migrations postgres $env:DATABASE_URL status
```

Kondisi terakhir yang dilaporkan: `00001`–`00010` **Applied**. `00010_create_gamification.sql` menambah tabel `mission_claims` dan trigger immutable pada `mission_claims` dan `deposit_claims`. Jangan menjalankan ulang SQL migration secara manual; gunakan Goose.

Tambahan lokal yang belum diterapkan/dibuktikan di Supabase: `00011_create_notifications.sql` (inbox dan trigger peristiwa) dan `00012_create_support_tickets.sql` (antrean bantuan dan penjaga data asli). Jangan mengedit 00001–00010 yang sudah Applied.

**Berhenti** dan laporkan ke AI jika:
- `00006` atau `00007` tercatat Applied tetapi skema remote tidak sesuai laporan kondisi terakhir. Isi kedua file sudah diubah setelah dibuat, jadi versi di database bisa berbeda.
- Tabel `goose_db_version` tidak ada padahal tabel `accounts` ada (00001 dulu diterapkan manual). Perlu baseline dulu, jangan langsung `up`.

## 2. Terapkan migration

```powershell
goose -dir database/migrations postgres $env:DATABASE_URL up
goose -dir database/migrations postgres $env:DATABASE_URL status
```

Setelah memperbarui backend, target terbaru adalah `00001`–`00012` Applied. Setiap file berjalan dalam transaksinya sendiri; jika satu gagal, file itu tidak setengah jadi, catat pesan error-nya. Migration baru tidak memasukkan notifikasi dari transaksi lama dan tidak memerlukan seed baru.

Pemeriksaan cepat di SQL editor Supabase:

```sql
SELECT tablename, rowsecurity FROM pg_tables WHERE schemaname='public' ORDER BY 1;
```

Semua tabel aplikasi harus `rowsecurity = true` (akses hanya lewat backend Go). `goose_db_version` dibuat Goose tanpa RLS; karena Supabase mengekspos schema `public` lewat REST API, kunci juga tabel ini (Goose memakai role `postgres` sehingga tidak terpengaruh):

```sql
ALTER TABLE public.goose_db_version ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.goose_db_version FROM anon, authenticated;
```

## 3. Data demo (urutan penting)

```powershell
# Kategori + satu lokasi demo untuk akun mitra yang sudah register lewat aplikasi
go run ./cmd/seed-partners --demo --mitra-uid <UID_MITRA_DEMO>
go run ./cmd/seed-point-rates --demo
go run ./cmd/seed-rewards --demo
go run ./cmd/seed-articles --demo
```

Semua seed aman diulang dan tidak menimpa data yang sudah ada.

## 4. Verifikasi mitra baru (D-21)

Mitra yang mendaftar lewat aplikasi berstatus pending dan belum bisa membuat setoran. Salin `data/seeders/dev/partner_location.example.json`, isi lokasi mitra, lalu:

```powershell
go run ./cmd/verify-partner approve --confirm --mitra-uid <UID> --location lokasi.json
go run ./cmd/verify-partner set-status --confirm --mitra-uid <UID> --status suspended   # atau verified / rejected
```

## 5. Setelah selesai

- Hapus `$env:DATABASE_URL` dari shell.
- Catat di `BUILD_PROCESS.md` (DATA-02): tanggal, output `goose status`, dan seed yang dijalankan.
- Mulai sekarang migration yang sudah Applied **tidak boleh diedit**; perubahan schema selalu lewat file migration baru.

## 6. Smoke test dengan Firebase nyata

Prasyarat: server berjalan (`go run ./cmd/app` dengan `DATABASE_URL` dan `GOOGLE_APPLICATION_CREDENTIALS`), seed sudah dijalankan, satu akun mitra (lokasi verified) dan satu akun user sudah didaftarkan lewat `POST /api/v1/accounts`.

```powershell
./scripts/smoke-test.ps1 -ApiKey <Firebase Web API Key>
```

Script meminta password kedua akun (tidak ditampilkan), lalu menjalankan estimasi → setoran → QR → klaim → penukaran reward → voucher, serta misi/XP, badge, leaderboard, dan pengaturan privasi. Restart backend sesudah memperbarui kode; migration 00010 saja tidak menambahkan route pada proses server lama.

Tiap run membuat 2 setoran (total 6,5 kg), memakai 1 stok reward 500 poin, dan mengklaim hadiah misi yang belum diklaim pada minggu berjalan. Hadiah yang sudah diklaim diuji sebagai replay, bukan diharapkan memberi XP lagi. Tidak ada seed XP atau badge yang diperlukan. Saldo dibandingkan relatif terhadap kondisi awal; XP tidak boleh mengubah EcoPoints.

Gunakan akun demo khusus tanpa aktivitas bersamaan dari perangkat atau script lain. Reset Senin 00.00 WIB di tengah run membuat baseline mingguan tidak berlaku: ulangi tes pada periode yang stabil. Script memulihkan visibilitas leaderboard ke nilai awal setelah pengujian privasi; periksa pesan kegagalan pemulihan jika server/jaringan terputus. Exit code 1 bila ada pemeriksaan gagal.

Laporan 36 PASS sebelumnya adalah versi tanpa gamifikasi. Jangan menyebut smoke versi baru lulus sebelum hasil terminalnya tersedia.

## 7. Smoke pengaturan akun saja

Restart backend dari terminal yang sudah memiliki `DATABASE_URL` dan `GOOGLE_APPLICATION_CREDENTIALS` (`Ctrl+C`, lalu `go run ./cmd/app`). Endpoint settings tidak memerlukan migration tambahan; server lama tetap akan mengembalikan 404 sampai direstart.

```powershell
./scripts/smoke-test.ps1 -ApiKey <Firebase Web API Key> -SettingsOnly
```

Masukkan password kedua akun melalui prompt tersembunyi. Mode ini menguji pengaturan user/mitra, PATCH parsial/retry, nilai boolean, isolasi akun, konsistensi dengan profil dan wallet, serta pemulihan nilai semula. Tidak membuat setoran, mengklaim XP, atau memakai stok reward. Gunakan akun demo tanpa aktivitas bersamaan. Jika pemulihan gagal akibat jaringan/server, periksa nilai lewat GET /api/v1/me/settings dan kembalikan sesuai baseline yang dicatat sebelum tes.

Jangan kirim password/token ke chat. Kirim hanya baris PASS/FAIL dan ringkasan hasil. Aturan lama menahan push sampai smoke nyata diubah pemilik saat meminta publikasi progress; kode settings kini sudah dipublikasikan, tetapi laporan 57 PASS sebelumnya tetap tidak mencakup endpoint settings.

## 8. Laporan, bantuan, dan notifikasi

Setelah memeriksa status Goose dan menerapkan 00011–00012, restart server. Kontrak: [statistik/PDF](api/activity-reports.md), [FAQ/tiket](api/support.md), [inbox](api/notifications.md). Laporan hanya menghitung setoran terklaim dan tidak mengubah saldo. FAQ memakai teks demo tanpa seed.

Tiket perlu ditindaklanjuti pemilik melalui `go run ./cmd/support-tickets list --status open`; CLI `reply`/`status` memerlukan `--confirm`. Kredensial database tetap di environment, bukan argumen atau chat. Tidak ada email/push; pengguna membuka tiket untuk melihat balasan.

Inbox dibuat atomik bersama klaim setoran, penukaran dan klaim XP yang baru, mengikuti `notification_enabled`. Backend harus memakai role pemilik migration seperti konfigurasi saat ini; role runtime berbeda membutuhkan grant tabel/fungsi dan kebijakan akses yang sesuai. Akun `anon`/`authenticated` tetap tidak diberi akses langsung.

Smoke 57 PASS dan mode SettingsOnly belum membuktikan tiga fitur baru ini. Verifikasi nyata selanjutnya perlu memeriksa angka laporan/unduhan PDF, tiket-balasan CLI, dan inbox/read melalui token akun masing-masing; jangan menyebutnya lulus sebelum hasil ada.

