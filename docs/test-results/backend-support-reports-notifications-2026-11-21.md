# Verifikasi backend: laporan, bantuan, dan notifikasi

Lingkungan: Windows, Go 1.27; PostgreSQL portable lokal disposable untuk data sintetis. Tidak memakai DATABASE_URL Supabase atau credential Firebase nyata. Batch ini tidak melakukan push.

## Build dan CLI

- `GOMAXPROCS=2 GOMEMLIMIT=128MiB go vet -p 1 -mod=readonly ./...`: exit 0.
- `go build -p 1 -mod=readonly ./...` dengan batas resource yang sama: exit 0.
- `gofmt -l` pada file Go modul baru, wiring, dan fixture migration: kosong.
- `env -u DATABASE_URL go run -p 1 -mod=readonly ./cmd/support-tickets list`: menampilkan `DATABASE_URL wajib diisi`, exit nonzero seperti yang diharapkan. Tidak membuka koneksi ke database lain.

## PDF nyata

Throwaway program memanggil `service.RenderActivityReportPDF` dengan DTO sintetis melalui implementasi asli. PyMuPDF (`fitz`) dan `pypdf` membaca hasilnya; halaman juga dirasterisasi menjadi contact sheet yang diperiksa visual. Program dan script pemeriksaan sementara dihapus.

- Rekap user: 14 halaman A4, 365 baris harian, satu baris jumlah dan 40 kategori. Header tabel berulang pada halaman sambungan; nama panjang terbungkus. Seluruh halaman mempunyai footer `Halaman N dari 14` yang terisi, bukan `{nb}`.
- Font tertanam: aksen Latin, Yunani dan Sirilik tetap terbaca (`Ünïcödé`, `Ωmega`, `Привет`, `Šumarni`, `Ñ`). Tidak ada blok teks melewati tepi halaman.
- Rekap mitra: satu halaman A4, lokasi terlihat dan tidak ada kolom poin dipakai. Dua karakter CJK yang tidak tersedia pada font diganti U+FFFD secara eksplisit, disertai catatan jumlah dua karakter; batas font tidak disembunyikan.
- Contact sheet user halaman pertama, halaman kategori, dan mitra diperiksa: tabel serta footer tidak terpotong. Ini pembuktian renderer menggunakan data sintetis, bukan bukti transaksi Supabase.

## PostgreSQL, HTTP, dan service

Run lengkap pertama yang benar-benar menjalankan Go menghasilkan 65 tes top-level PASS, 3 FAIL dan 0 SKIP. HTTP, service/PDF dan tiket bantuan termasuk lifecycle CLI sudah lulus. Tiga kegagalan: TestActivityReportIntegration, TestGamificationIntegration dan TestNotificationIntegration.

- Fixture laporan memakai UPDATE pada claimed_at milik deposit_claims yang immutable (SQLSTATE 23001). Fixture diperbaiki dengan INSERT klaim historis dan pencatatan ledger/wallet/token/status dalam satu transaksi sejak awal. Trigger immutable tidak dinonaktifkan dan migration 00010 tidak diubah; assertion laporan tetap.
- Trigger notifikasi misi pada migration baru 00011 memakai `AT TIME ZONE 'Asia/Jakarta'`, tidak tersedia pada distribusi PostgreSQL portable minimal ini (SQLSTATE 22023). Perhitungan tanggal sumber notifikasi diubah ke `AT TIME ZONE 'UTC' + interval '7 hours'` (WIB tetap UTC+7), mengikuti pola laporan. Kegagalan lanjutan gamifikasi berasal dari INSERT hadiah yang ikut rollback saat trigger gagal; pengaman immutable tidak dilonggarkan.

Run akhir `./scripts/test-postgres-portable.ps1 -PostgresBin <portable-pg-bin>` setelah perbaikan: **exit 0, 68 tes top-level + 120 subtest PASS, 0 FAIL, 0 SKIP**. Perintah Go di runner: `go test -p 1 -mod=readonly -count=1 -v ./...`. PostgreSQL berhenti normal dan cluster run dibersihkan melalui `finally`.

| Jalur | Bukti run akhir |
|---|---|
| Laporan | TestActivityReportIntegration: total/harian/kategori, batas mikrodetik WIB, periode default, isolasi akun dan lokasi mitra, sumber tidak berubah, error akun, PDF dari data PostgreSQL. TestActivityReportHTTP dan tes service/PDF juga PASS. |
| Bantuan | TestSupportIntegration: 12 retry paralel menghasilkan satu tiket, 30 percobaan paralel menerima tepat 20 tiket belum selesai, retry tetap aman di batas, isolasi, akun nonaktif, data asli immutable, RLS, CLI balasan/status terlihat oleh pengguna. Tes HTTP, service dan CLI juga PASS. |
| Inbox | TestNotificationIntegration: notifikasi klaim user/mitra sekali, preferensi, rollback, voucher/XP, ownership, akun nonaktif, read idempoten, cursor/unread, missing settings gagal tertutup, RLS. TestNotificationHTTP PASS. |
| Regresi | Settings, QR/claim, wallet/ledger, reward/stok, gamifikasi, akun, mitra/favorit, artikel, seed dan CLI verifikasi mitra semuanya PASS. Log penolakan yang memang diharapkan di tes CLI bukan kegagalan suite. |
| Migration | SQL Up/Down/reapply 00001–00012 PASS lewat loader integration. Ini tidak menguji version tracking Goose atau penerapan remote. |

Log lokal: `%TEMP%/ecosnap-backend-verify-final.log` (run gagal pertama), `%TEMP%/ecosnap-backend-verify-repaired.log` (run akhir PASS). Sampel PDF dan contact sheet di `%TEMP%/EcoSnapActivityPDFProof`; sampel Go test dan sampel renderer visual memakai fixture sintetis terpisah. Tidak ada program/script proof sementara yang tersisa di repo.

Percobaan sebelumnya tidak memberikan hasil tes Go: proses sempat dihentikan atas permintaan pengguna, dan wrapper PowerShell berikutnya tertahan setelah startup PostgreSQL ketika seluruh pemanggilan script dialihkan memakai `*>` di dalam PowerShell. Saat diperiksa tidak ada proses Go aktif. Redireksi file di shell luar, sama seperti pola verifikasi sebelumnya, berhasil menjalankan suite. Dua direktori cluster sintetis run yang dibatalkan dibersihkan setelah dipastikan PID-nya tidak berjalan.

## Smoke pengaturan akun

Parser PowerShell serta harness offline AST dari `scripts/smoke-test.ps1` sebelumnya lulus untuk dua role, pemulihan preferensi saat kegagalan transport, dan penolakan bentuk respons/wallet gagal. Harness dihapus. Bukti offline tidak menggantikan mode `-SettingsOnly` dengan login nyata.

## Batas bukti

Migration 00011–00012 belum diterapkan ke Supabase oleh batch ini. Tidak menguji token Firebase nyata atau UI Android untuk laporan, bantuan, inbox, maupun settings. Laporan historis 57 PASS memakai cakupan sebelum fitur baru. Push/email, upload, AI, game dan hosting tidak ditambahkan. Belum membuktikan operasi produksi atau performa beban besar.
