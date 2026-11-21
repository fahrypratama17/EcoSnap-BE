# Runbook: migration dan data demo ke Supabase development

Dijalankan oleh pemilik repo. AI tidak membaca credential dan tidak mengeksekusi ke Supabase. Kondisi awal (laporan pemilik 2026-11-21): hanya `00001_create_accounts.sql` yang sudah diterapkan.

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

Yang diharapkan: `00001` **Applied**, `00002`–`00008` **Pending**.

**Berhenti** dan laporkan ke AI jika:
- `00006` atau `00007` tercatat Applied. Isi kedua file sudah diubah setelah dibuat, jadi versi di database bisa berbeda.
- Tabel `goose_db_version` tidak ada padahal tabel `accounts` ada (00001 dulu diterapkan manual). Perlu baseline dulu, jangan langsung `up`.

## 2. Terapkan migration

```powershell
goose -dir database/migrations postgres $env:DATABASE_URL up
goose -dir database/migrations postgres $env:DATABASE_URL status
```

Semua `00001`–`00008` harus Applied. Setiap file berjalan dalam transaksinya sendiri; jika satu gagal, file itu tidak setengah jadi, catat pesan error-nya.

Pemeriksaan cepat di SQL editor Supabase:

```sql
SELECT tablename, rowsecurity FROM pg_tables WHERE schemaname='public' ORDER BY 1;
```

Semua tabel aplikasi harus `rowsecurity = true` (akses hanya lewat backend Go).

## 3. Data demo (urutan penting)

```powershell
# Kategori + satu lokasi demo untuk akun mitra yang sudah register lewat aplikasi
go run ./cmd/seed-partners --demo --mitra-uid <UID_MITRA_DEMO>
go run ./cmd/seed-point-rates --demo
go run ./cmd/seed-rewards --demo
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
