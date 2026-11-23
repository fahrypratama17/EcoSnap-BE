# Newman full flow CI — 24 November 2026

## Otomatisasi dan isolasi

Job `newman-full-flow` ditambahkan ke workflow `ci` untuk push ke `main`/`dev` dan pull request menuju kedua branch tersebut. Job lama tidak diganti. Setiap job Newman menggunakan PostgreSQL 17 sementara, database acak yang dibuat fixture terjaga, semua migration, dan listener HTTP loopback dengan handler `bootstrap.NewServer` asli.

Test Go memakai build tag `newman`; `go test ./...` biasa tidak memperoleh dependency Node atau test baru yang di-skip. Saat tag diaktifkan, ketiadaan Node/`ECOSNAP_TEST_ADMIN_URL` menjadi kegagalan. Fixture tidak memakai `DATABASE_URL` development. Database/listener dibersihkan saat test selesai atau gagal.

Verifier Firebase hanya ada di `_test.go`, menerima empat token sintetis yang dipetakan ke UID fixture dan menolak nilai lain. Metode verifikasi biasa maupun fresh-auth memakai allowlist tersebut. Tidak ada flag auth sintetis pada server produksi, password, token Firebase asli atau credential database remote.

## Flow yang dijalankan

Newman menjalankan salinan in-memory dari koleksi asli, tanpa mengganti assertion atau file kontraknya:

1. Registrasi user dan mitra baru dengan identitas khusus registrasi.
2. Putaran utama dengan user dan mitra fixture verified: profil/settings, katalog, favorit, estimasi, setoran/QR/klaim/replay, saldo/riwayat/Home, reward/voucher, misi/badge/leaderboard, laporan JSON/PDF, inbox dan tiket bantuan.
3. Putaran utama baru dengan pagination. Dua putaran menyediakan dua setoran, voucher dan tiket; katalog memuat lima artikel. Key idempotency baru hanya untuk intent baru di salinan kedua, bukan retry. Request replay dalam setiap putaran tetap memakai key yang sama.

Folder helper manual dikecualikan; registrasi tidak dijalankan ulang sebagai akun utama. Cakupan/jumlah request dihitung dari koleksi, bukan angka tetap pada runner.

## Bukti HTTP dan PostgreSQL lokal

`TestNewmanFullFlow` terakhir PASS setelah gate per-execution ditambahkan. Handler, middleware, service dan repository produksi memakai PostgreSQL portable disposable dengan migration 00001–00012; hanya verifier identitas yang diganti.

| Fase | Request | Assertion |
|---|---:|---:|
| Registrasi | 2 | 11 |
| Putaran utama | 92 | 312 |
| Putaran baru + pagination | 104 | 354 |
| Total | 198 | 677 |

Log handler mencatat tepat 198 request; semua **46 operasi** mempunyai respons sukses, termasuk dua pendaftaran 201. Request negatif serta replay juga dijalankan. Ringkasan runner: `NEWMAN_FULL_FLOW_PASS phases=3 requests=198 assertions=677`.

Marker `NEWMAN_POSTGRES_PROOF_PASS` dan `NEWMAN_POSTGRES_CLUSTER_REMOVED` teramati; cluster dihentikan dan dihapus. Script proof portable sementara tidak masuk commit. Tidak menjalankan ulang full suite Go lokal karena kode produksi tidak berubah; job Go lama tetap dijalankan GitHub.


## Bukti runner menolak hijau palsu

`node --test scripts/newman/test-runner.cjs` terakhir: **7 tes PASS, 0 FAIL/SKIP**, memakai Newman nyata dan server HTTP loopback:

- Respons dengan assertion yang sesuai diterima.
- Respons tanpa assertion ditolak (`unchecked`).
- HTTP 200 ketika assertion mengharapkan 201 ditolak (`assertion`), tepat satu request diterima server.
- `pm.execution.skipRequest()` pada pre-request ditolak sebagai run tidak lengkap; server menerima nol request.
- Error script ditolak tanpa menyalin isi error privat.
- Target non-loopback ditolak sebelum koneksi.
- CLI dijalankan sebagai proses anak terhadap respons HTTP 418 nyata; assertion registrasi gagal, **exit code 1**, tepat satu request, tanpa membocorkan body respons.

Runner memeriksa seluruh request terencana, respons, error transport/script dan assertion yang tidak di-skip pada setiap execution. Pesan gagal hanya memuat kategori/fase terkontrol; tidak menyalin body, kode voucher atau token QR. Request timeout dan script timeout dibatasi; proses Go membatasi seluruh flow hingga lima menit.

## Tooling dan batas

Newman **6.2.3** dipasang dengan `npm ci --prefix scripts/newman --ignore-scripts` dari lockfile. Install lokal menambahkan 148 package dan menampilkan warning deprecation upstream (`har-validator`, `uuid`, `faker`); dependency ini khusus tooling CI, bukan server Go. Node lokal 24.14.0; workflow menetapkan Node 22. Format file Go baru dan sintaks YAML workflow diperiksa lokal.

CI ini bukan auto-deploy, auto-merge atau penerapan migration ke Supabase development. Branch protection tidak diubah. Firebase/Supabase nyata, Android, generated Kotlin client dan pencocokan seluruh field response terhadap OpenAPI tetap di luar bukti ini. Kegagalan HTTP yang memang diharapkan dalam request negatif tidak membuat job gagal selama assertion sesuai kontrak.

## Menjalankan ulang

Sediakan PostgreSQL disposable pada loopback melalui `ECOSNAP_TEST_ADMIN_URL` sesuai guard fixture, lalu:

```text
npm ci --prefix scripts/newman --ignore-scripts
node --test scripts/newman/test-runner.cjs
go test -p 1 -mod=readonly -tags newman -count=1 -v ./internal/repository/postgres -run "^TestNewmanFullFlow$"
```

Panduan: [indeks API](../api/README.md#gate-ci-kontrak-dan-smoke-offline) dan [Postman](../api/postman.md#full-flow-otomatis-di-ci). Status run GitHub diperiksa setelah publikasi; hasil lokal tidak disebut sebagai hasil GitHub.
