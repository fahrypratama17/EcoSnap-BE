# Hasil uji gamifikasi

Lingkungan: Windows, Go 1.27.1, PostgreSQL lokal disposable melalui binary portable. Tidak menggunakan DATABASE_URL/Supabase atau kredensial Firebase nyata.

## Hasil akhir

Perintah tes pada cluster loopback sementara:

```text
go test -p 1 -mod=readonly -count=1 -v ./internal/repository/postgres ./internal/delivery/http -run 'TestGamification|TestWeekPeriod'
```

Exit 0, tanpa SKIP:
- TestWeekPeriodWIBBoundaries: Minggu detik terakhir dan Senin tepat 00.00 WIB.
- TestGamificationIntegration: target belum tercapai, klaim/replay, delapan klaim paralel hanya satu award, saldo EcoPoints tidak berubah, klaim minggu lalu dan setoran belum diklaim tidak masuk progres, batas 2999/3000 gram dari setoran multi-item, token periode lama ditolak, badge all-time, ranking seri, sembunyikan/tampilkan profil, pengecualian akun suspended/mitra, dan penolakan perubahan/penghapusan histori XP/klaim.
- TestGamificationHTTP: autentikasi/role, status 201/200/404/410, token periode, JSON ketat, query asing/duplikat/kosong/encoding rusak, dan pengaturan visibilitas.

Migration 00010 diterapkan pada database tes. Runner sementara menghentikan PostgreSQL dan menghapus cluster setelah tes; file runner tidak dipertahankan.

## Riwayat verifikasi

Runner full suite juga dijalankan. Tes lama lulus; suite tersebut gagal pada fixture gamifikasi baru karena parameter SQL ambigu. Fixture diperbaiki dengan cast bigint/timestamptz. Run terarah berikutnya menemukan timestamp fixture badge berada di masa depan dan asumsi visibilitas default yang salah; keduanya diperbaiki. Hasil akhir terarah di atas lulus. Full suite tidak dijalankan ulang setelah perbaikan fixture terakhir; jangan menyebut full suite final hijau.

## Batas bukti

LAPORAN pemilik: migration 00001–00010 Applied di Supabase development; smoke terbaru dengan token Firebase nyata **57 lulus, 0 gagal**. Integrasi/UI Android belum diuji. Schema DBML lama bukan representasi schema runtime; migrasi SQL adalah acuan.

## Perluasan smoke test

`scripts/smoke-test.ps1` kini memeriksa baseline progres mingguan dan badge, setoran belum diklaim tidak memberi progres, hadiah 50/100 XP, replay tanpa XP ganda, penolakan periode lama dan role mitra, wallet EcoPoints tidak berubah karena XP, serta hide/show leaderboard dengan pemulihan visibilitas awal dalam `finally`. Baseline nonzero dan hadiah yang sudah diklaim didukung; gunakan akun demo tanpa aktivitas bersamaan.

Verifikasi yang dijalankan:
- Parser Windows PowerShell 5.1.26100.9818: parse OK.
- Harness sementara mengeksekusi blok gamifikasi/privasi dari AST script sebenarnya dengan respons simulasi: klaim baru, klaim pertama sudah pernah dilakukan, visibilitas awal false/true, pemulihan setelah kegagalan leaderboard untuk kedua nilai awal, dan respons hadiah salah 999 XP yang harus terdeteksi. Semua skenario harness lulus.
- Harness tidak memakai jaringan, database, atau credential dan sudah dihapus. Pemeriksaan setoran awal berada di luar blok harness; bagian itu hanya tercakup parser dan smoke versi lama.

LAPORAN pemilik setelah restart backend: smoke versi baru **57 lulus, 0 gagal**, termasuk progres setoran belum/sudah diklaim, hadiah kedua misi dan replay, pemisahan XP/EcoPoints, badge/timestamp permanen, penolakan periode lama/mitra, leaderboard tanpa UID/email, serta hide/show dan pemulihan visibilitas awal. Saldo awal 140 dan saldo akhir awal +140; total_spent naik 500. Bukti ini berasal dari output terminal pemilik, tidak dijalankan ulang oleh agen.
