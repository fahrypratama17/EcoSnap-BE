# Pengujian akun dengan PostgreSQL

Pembaruan verifikasi: full suite PostgreSQL lokal portable kini PASS tanpa SKIP, termasuk akun, query mitra, favorit, dan CLI seed. Lihat [laporan PostgreSQL](test-results/postgres-favorites.md). Catatan belum diuji/SKIP di bagian historis berikut menggambarkan pemeriksaan sebelumnya. Supabase, Firebase nyata, dan Android tetap belum diuji end-to-end.


## Status verifikasi

File integration test sudah dapat dikompilasi. Pada lingkungan saat penulisan, Docker Engine belum dapat dihubungi meskipun Docker Desktop telah dicoba dijalankan. Environment database tes juga belum tersedia. Hasil eksekusi `TestAccountRepositoryIntegration` adalah **SKIP**, bukan bukti bahwa operasi PostgreSQL sudah lolos.

Percobaan Go awal sempat gagal karena kehabisan memori. Eksekusi berikutnya dengan GOMAXPROCS=2, GOMEMLIMIT=128MiB, dan `-p 1` berhasil menjalankan binary test hingga pemeriksaan environment. Tidak ada perubahan konfigurasi Go permanen atau penambahan dependency Go.

## Interface dan skenario

Pengujian berada di `internal/repository/postgres/account_registration_integration_test.go`. Perilaku akun diamati melalui method publik `AccountRepository.Register` dan `AccountRepository.Profile`, tanpa verifier Firebase atau memory store. SQL tambahan hanya dipakai untuk menyiapkan migration dan menyuntikkan kegagalan pada database disposable.

Sembilan skenario:

1. Pendaftaran dapat dibaca kembali beserta settings awal dan wallet nol.
2. Retry identik mengembalikan profil lama, termasuk email lama, tanpa membuat akun baru.
3. Pendaftaran ulang dengan nama berbeda menghasilkan konflik tanpa mengubah profil.
4. Pendaftaran usaha mitra dapat dibaca dan statusnya tetap pending.
5. Enam belas request bersamaan untuk UID sama menghasilkan tepat satu hasil `created=true`.
6. Kegagalan penyimpanan wallet membatalkan pendaftaran; retry sesudah kegagalan dihilangkan berhasil sebagai pendaftaran baru.
7. Kegagalan constraint detail usaha membatalkan akun/settings/wallet; retry valid berhasil tanpa sisa data parsial.
8. Profil yang tidak ada menghasilkan error not found.
9. SQL migration Down dapat dijalankan berurutan terbalik lalu Up diterapkan kembali, dan pendaftaran baru berfungsi.

Pengujian ini menjalankan bagian SQL Up/Down dari file migration sebenarnya. Ia tidak memanggil Goose CLI atau membuktikan version tracking/re-run Goose. Tidak menguji RLS melalui role client anon/authenticated, kebijakan Firebase, HTTP end-to-end dengan token nyata, atau migration pada Supabase.

## Cara menjalankan dengan Docker

Aktifkan Docker Desktop menggunakan Linux containers. Pastikan perintah berikut berhasil:

```powershell
docker info
```

Dari root EcoSnap-BE, jalankan:

```powershell
./scripts/test-accounts-postgres.ps1
```

Runner memakai image `postgres:17-alpine`; Docker dapat mengunduh image jika belum tersedia. Runner membuat container dengan nama acak, tanpa volume, dengan port acak yang hanya terikat ke 127.0.0.1. Password contoh hanya untuk container tes lokal. Tidak ada pembacaan DATABASE_URL aplikasi atau credential Firebase.

Runner menunggu PostgreSQL siap, memasang ECOSNAP_TEST_ADMIN_URL pada prosesnya, lalu menjalankan:

```powershell
go test -p 1 -mod=readonly -count=1 ./internal/repository/postgres -run '^TestAccountRepositoryIntegration$' -v
```

Hasil sukses harus memperlihatkan seluruh subtest PASS tanpa SKIP. Runner menghentikan container miliknya pada blok finally; `--rm` menghapus container tersebut. Environment sebelumnya dipulihkan. Jika proses dihentikan paksa sehingga finally tidak berjalan, periksa container dengan nama `ecosnap-account-tests-...` yang dibuat pada run tersebut dan bersihkan container itu saja.

File ini bukan Dockerfile/Compose untuk aplikasi utama. DEV-01 pada tracker tetap terpisah.

## PostgreSQL lokal yang sudah tersedia

Gunakan cluster lokal khusus tes yang penggunanya dapat CREATE DATABASE dan membuat role. Jangan menunjuk instance aplikasi atau tunnel ke database aplikasi. Contoh konfigurasi untuk cluster tes lokal:

```powershell
$env:ECOSNAP_TEST_ADMIN_URL = 'postgresql://TEST_ADMIN:TEST_PASSWORD@127.0.0.1:TEST_PORT/postgres?sslmode=disable'
go test -p 1 -mod=readonly -count=1 ./internal/repository/postgres -run '^TestAccountRepositoryIntegration$' -v
```

Helper hanya menerima database admin bernama postgres pada alamat loopback tanpa fallback hosts. Ia membuat database baru bernama `ecosnap_account_test_<random>` untuk setiap run. Pada cluster lokal itu, role NOLOGIN anon/authenticated dibuat bila belum ada agar SQL REVOKE dari migration Supabase dapat dijalankan. Setelah tes, pool ditutup lalu hanya database hasil run tersebut yang dihapus. Database yang sudah ada tidak di-reset atau dijalankan migration.

Jika environment tidak ada, test sengaja SKIP. Jika environment ada tetapi salah atau PostgreSQL tidak dapat dihubungi, test gagal. Tidak ada fallback otomatis ke DATABASE_URL.

## Setelah suite ini lolos

Catat output aktual pada BUILD_PROCESS.md sebelum menaikkan status task. Verifikasi migration development dengan Goose dan pengujian HTTP menggunakan Firebase ID token nyata masih diperlukan. Migration 00002 pada Supabase belum dijalankan oleh tahap ini.

Fixture SQL integration kemudian diperluas hingga migration 00003, dengan urutan Down dibalik untuk semua migration. Ini belum membuktikan migration ketiga atau query mitra lolos; suite database tetap perlu dijalankan ketika PostgreSQL tes tersedia.

## Guard ruang sebelum startup

Kedua runner sekarang memanggil scripts/assert-test-storage.ps1 sebelum membuat container atau cluster. Volume repository, TEMP, dan GOCACHE diperiksa dengan batas awal konservatif 4 GiB per volume unik. Batas ini mencegah pengulangan kegagalan pada C sekitar 413 MB; bukan ukuran universal yang menjamin semua build/image/VM cukup. Kebutuhan volume penyimpanan Docker dan kebutuhan puncak build tetap harus diperhatikan.

Jalankan uji helper tanpa membuat database/container:

```powershell
./scripts/test-storage-guard.ps1
```

Uji memeriksa jalur cukup dengan minimum satu byte dan jalur kurang dengan minimum yang melebihi kapasitas drive, tanpa membuat file besar. Pada pengecekan runner aktual terbaru, keduanya menolak ruang 0,39 GiB dengan ECOSNAP_TEST_DISK_LOW sebelum startup. Daftar cluster dan environment ECOSNAP_TEST_ADMIN_URL tidak berubah. Ini hasil guard, bukan hasil full suite PostgreSQL.

Baseline db51b4b tetap dipakai. Sediakan ruang, jalankan runner penuh, dan perbarui GIT-03 setelah hasil nyata PASS; tidak perlu mengulang pembuatan baseline Git.
