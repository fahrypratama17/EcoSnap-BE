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

**Berhenti** dan laporkan ke AI jika:
- `00006` atau `00007` tercatat Applied tetapi skema remote tidak sesuai laporan kondisi terakhir. Isi kedua file sudah diubah setelah dibuat, jadi versi di database bisa berbeda.
- Tabel `goose_db_version` tidak ada padahal tabel `accounts` ada (00001 dulu diterapkan manual). Perlu baseline dulu, jangan langsung `up`.

## 2. Terapkan migration

```powershell
goose -dir database/migrations postgres $env:DATABASE_URL up
goose -dir database/migrations postgres $env:DATABASE_URL status
```

Semua `00001`–`00010` harus Applied. Setiap file berjalan dalam transaksinya sendiri; jika satu gagal, file itu tidak setengah jadi, catat pesan error-nya.

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
