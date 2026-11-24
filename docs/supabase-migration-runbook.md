# Runbook: migration dan data demo ke Supabase development

Dijalankan oleh pemilik repo. AI tidak membaca credential dan tidak mengeksekusi ke Supabase. Laporan pemilik 2026-11-24 pukul 12:40: seluruh migration `00001`–`00012` Applied; `goose up` berikutnya menyatakan tidak ada migration lagi.

## 0. Sebelum mulai

- Pastikan project yang dipakai adalah **development**, bukan production.
- Ambil connection string **direct** (port 5432) atau **session pooler** dari Supabase → Project Settings → Database. Jangan pakai transaction pooler (port 6543) untuk migration.
- Untuk server aplikasi juga pakai 5432/session pooler. Jika terpaksa 6543, backend otomatis beralih ke mode query tanpa prepared statement (sedikit lebih lambat); migration tetap wajib lewat 5432.
- Untuk server Go dan Goose, gunakan `.env` lokal di root repo sesuai [konfigurasi lokal](api/README.md#konfigurasi-lokal-tanpa-set-environment-manual). Isi `DATABASE_URL`, path `GOOGLE_APPLICATION_CREDENTIALS`, serta `GOOSE_DRIVER=postgres`, `GOOSE_DBSTRING=${DATABASE_URL}` dan `GOOSE_MIGRATION_DIR=database/migrations`. Simpan UTF-8 tanpa BOM. Server cukup `go run ./cmd/app`; `.env` tidak di-commit.
- Buka terminal baru jika environment lama masih terpasang: environment proses selalu diprioritaskan dibanding `.env`. Restart server setelah mengubah file.
- **Khusus CLI seed/verifikasi/tiket pada bagian 3–4**, `DATABASE_URL` tetap harus ada di environment shell; loader baru hanya untuk startup server, sedangkan Goose memuat file sendiri. Jangan menyimpan nilai asli di file yang di-commit:

```powershell
$env:DATABASE_URL = "postgresql://postgres:<password>@<host>:5432/postgres?sslmode=require"
```

- Pull branch `dev` terbaru dan pastikan tes lokal PASS (`./scripts/test-postgres-portable.ps1 …` atau CI hijau).

## 1. Cek status Goose

```powershell
goose -env .env status
```

Kondisi terakhir yang dilaporkan: `00001`–`00012` **Applied**. `00010_create_gamification.sql` menambah tabel `mission_claims` dan trigger immutable pada `mission_claims` dan `deposit_claims`. Jangan menjalankan ulang SQL migration secara manual; gunakan Goose.

`00011_create_notifications.sql` (inbox dan trigger peristiwa) serta `00012_create_support_tickets.sql` (antrean bantuan dan penjaga data asli) dilaporkan sudah diterapkan pada 2026-11-24. Jangan mengedit migration yang sudah Applied. Perbaikan latensi Home/misi tidak memerlukan migration atau seed ulang.

**Berhenti** dan laporkan ke AI jika:
- `00006` atau `00007` tercatat Applied tetapi skema remote tidak sesuai laporan kondisi terakhir. Isi kedua file sudah diubah setelah dibuat, jadi versi di database bisa berbeda.
- Tabel `goose_db_version` tidak ada padahal tabel `accounts` ada (00001 dulu diterapkan manual). Perlu baseline dulu, jangan langsung `up`.

## 2. Terapkan migration

```powershell
goose -env .env up
goose -env .env status
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

- Jika sebelumnya menyetel `$env:DATABASE_URL` untuk CLI, hapus dari shell setelah selesai; file `.env` lokal tetap disimpan untuk startup server berikutnya.
- Catat di `BUILD_PROCESS.md` (DATA-02): tanggal, output `goose status`, dan seed yang dijalankan.
- Mulai sekarang migration yang sudah Applied **tidak boleh diedit**; perubahan schema selalu lewat file migration baru.

## 6. Smoke test dengan Firebase nyata

Prasyarat: server berjalan (`go run ./cmd/app` dari root dengan `DATABASE_URL` dan `GOOGLE_APPLICATION_CREDENTIALS` di `.env` lokal atau environment proses), seed sudah dijalankan, satu akun mitra (lokasi verified) dan satu akun user sudah didaftarkan lewat `POST /api/v1/accounts`.

```powershell
./scripts/smoke-test.ps1 -ApiKey <Firebase Web API Key>
```

Script meminta password kedua akun (tidak ditampilkan), lalu menjalankan estimasi → setoran → QR → klaim → penukaran reward → voucher, serta misi/XP, badge, leaderboard, dan pengaturan privasi. Restart backend sesudah memperbarui kode; migration 00010 saja tidak menambahkan route pada proses server lama.

Tiap run membuat 2 setoran (total 6,5 kg), memakai 1 stok reward 500 poin, dan mengklaim hadiah misi yang belum diklaim pada minggu berjalan. Hadiah yang sudah diklaim diuji sebagai replay, bukan diharapkan memberi XP lagi. Tidak ada seed XP atau badge yang diperlukan. Saldo dibandingkan relatif terhadap kondisi awal; XP tidak boleh mengubah EcoPoints.

Gunakan akun demo khusus tanpa aktivitas bersamaan dari perangkat atau script lain. Reset Senin 00.00 WIB di tengah run membuat baseline mingguan tidak berlaku: ulangi tes pada periode yang stabil. Script memulihkan visibilitas leaderboard ke nilai awal setelah pengujian privasi; periksa pesan kegagalan pemulihan jika server/jaringan terputus. Exit code 1 bila ada pemeriksaan gagal.

Laporan 36 PASS sebelumnya adalah versi tanpa gamifikasi. Jangan menyebut smoke versi baru lulus sebelum hasil terminalnya tersedia.

## 7. Smoke pengaturan akun saja

Restart backend dari root repo (`Ctrl+C`, lalu `go run ./cmd/app`); konfigurasi dibaca ulang dari `.env` lokal atau environment proses yang lebih diprioritaskan. Endpoint settings tidak memerlukan migration tambahan; server lama tetap akan mengembalikan 404 sampai direstart.

```powershell
./scripts/smoke-test.ps1 -ApiKey <Firebase Web API Key> -SettingsOnly
```

Masukkan password kedua akun melalui prompt tersembunyi. Mode ini menguji pengaturan user/mitra, PATCH parsial/retry, nilai boolean, isolasi akun, konsistensi dengan profil dan wallet, serta pemulihan nilai semula. Tidak membuat setoran, mengklaim XP, atau memakai stok reward. Gunakan akun demo tanpa aktivitas bersamaan. Jika pemulihan gagal akibat jaringan/server, periksa nilai lewat GET /api/v1/me/settings dan kembalikan sesuai baseline yang dicatat sebelum tes.

Jangan kirim password/token ke chat. Kirim hanya baris PASS/FAIL dan ringkasan hasil. Aturan lama menahan push sampai smoke nyata diubah pemilik saat meminta publikasi progress; kode settings kini sudah dipublikasikan, tetapi laporan 57 PASS sebelumnya tetap tidak mencakup endpoint settings.

## 8. Laporan, bantuan, dan notifikasi

Setelah memeriksa status Goose dan menerapkan 00011–00012, restart server. Kontrak: [statistik/PDF](api/activity-reports.md), [FAQ/tiket](api/support.md), [inbox](api/notifications.md). Laporan hanya menghitung setoran terklaim dan tidak mengubah saldo. FAQ memakai teks demo tanpa seed.

Tiket perlu ditindaklanjuti pemilik melalui `go run ./cmd/support-tickets list --status open`; CLI `reply`/`status` memerlukan `--confirm`. Kredensial database tetap di environment, bukan argumen atau chat. Tidak ada email/push; pengguna membuka tiket untuk melihat balasan.

Inbox dibuat atomik bersama klaim setoran, penukaran dan klaim XP yang baru, mengikuti `notification_enabled`. Backend harus memakai role pemilik migration seperti konfigurasi saat ini; role runtime berbeda membutuhkan grant tabel/fungsi dan kebijakan akses yang sesuai. Akun `anon`/`authenticated` tetap tidak diberi akses langsung.

Laporan pemilik 2026-11-24: `-SettingsOnly` **47 PASS, 0 FAIL** dengan Firebase/Supabase nyata. `-FlowsOnly` berikutnya **66 PASS, 3 FAIL**: Home user, Home mitra dan misi terkena `504 request_timeout`; laporan JSON/PDF kedua peran, daftar inbox, FAQ serta daftar tiket berhasil dibaca. Preflight menghentikan tes sebelum transaksi, klaim XP atau tiket baru. Ini belum membuktikan full smoke, event inbox/read baru maupun tiket-balasan CLI.

Sesudah mengambil perbaikan Home/misi, restart backend dari terminal yang masih memiliki `DATABASE_URL` dan `GOOGLE_APPLICATION_CREDENTIALS`, lalu jalankan `./scripts/smoke-test.ps1 -ApiKey $key -FlowsOnly` dari terminal smoke. Jangan langsung mengulang full smoke atau migration. Kirim ringkasan PASS/FAIL dan log `request_id`/`duration` bila masih 504, tanpa credential/token.

