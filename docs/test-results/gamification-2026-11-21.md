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

Belum menerapkan migration 00010 di Supabase development, belum menjalankan endpoint gamifikasi dengan token Firebase nyata, belum mengintegrasikan/menguji UI Android. Smoke test 36 pemeriksaan yang dilaporkan pemilik sebelumnya tidak mencakup gamifikasi. Schema DBML lama bukan representasi schema runtime; migrasi SQL adalah acuan.
