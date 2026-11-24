# Home dan misi melewati deadline — 24 November 2026

## Bukti dari layanan asli

Pemilik melaporkan migration Supabase 00001–00012 Applied, smoke Firebase/Supabase `-SettingsOnly` 47 PASS/0 FAIL, kemudian `-FlowsOnly` 66 PASS/3 FAIL. Tiga kegagalan ialah `/me/home` user dan mitra serta `/me/missions`, semuanya HTTP 504. Preflight berhenti sebelum setoran, redeem, klaim XP atau tiket baru.

Log server yang dikirim pemilik memperlihatkan Home gagal pada 5.0010537s dan 5.0003769s. Misi sempat 200 pada 3.2058159s, kemudian 504 pada 5.0106071s. Wallet, badge, laporan JSON/PDF, daftar inbox, FAQ dan daftar tiket tetap berhasil. Ini bukan bukti migration hilang atau login Firebase gagal. Log tidak mengungkap durasi eksekusi tiap query maupun wait event PostgreSQL, jadi penyebab remote tidak dinyatakan sudah diprofilkan secara lengkap.

## Reproduksi yang dijalankan sebelum fix

`TestReadPathsUnderNetworkLatency` menjalankan handler HTTP produksi dan PostgreSQL disposable, dengan verifier identitas sintetis. Dialer pool khusus tes membungkus protokol PostgreSQL plaintext loopback dan menunda setiap frame `ReadyForQuery` 450 ms. Tidak mengubah data/query produksi, tidak menghitung jumlah query sebagai assertion, dan tidak memakai Supabase remote.

Pool hanya satu koneksi. Ping memanaskan koneksi, bukan cache statement SQL. Sebelum perbaikan, ketiga endpoint yang dilaporkan pemilik benar-benar menghasilkan 504 `request_timeout`; wallet dan badge tetap 200. Ini menjadi loop merah yang dapat dijalankan tanpa credential asli. Cluster dihentikan dan dihapus walaupun baseline memang gagal.

## Perubahan

- `UserHome`: dua batch berurutan di dalam transaksi `REPEATABLE READ` yang sama. Batch pertama mengunci akun `FOR SHARE`, membaca status/role/nama, lalu mengambil clock setelah lock. Batch kedua membaca kartu-kartu Home; tidak membuka koneksi tambahan atau goroutine.
- History, partner dan artikel memakai query builder serta scanner yang sama pada endpoint biasa dan batch Home. Filter, cursor, urutan, `[]`, kartu nullable dan decoding detail setoran/redeem dipertahankan. Unread count memakai konstanta SQL bersama.
- Snapshot misi: tiga statement serial digabung menjadi satu statement aggregate. Progress setoran, claimed-map serta XP memiliki filter waktu yang sama seperti sebelumnya; JSONB claimed-map di-scan ke `map[string]time.Time`. Jalur claim/replay juga memakai helper yang sama.
- Error query, scan, rows, penutupan batch dan commit tetap diteruskan; cleanup transaksi tetap berjalan. `PartnerHome` tidak diubah.

**Deadline handler tetap 5 detik.** Tidak menambah cache, retry, indeks/migration, koneksi pool atau field API. Tidak membaca `.env.example`, credential Firebase, ataupun mengeksekusi query ke database remote.

## Hasil akhir lokal

Angka berikut diambil dari request HTTP yang dibaca test, dengan latensi buatan yang sama. Sebelum fix berasal dari baseline merah; sesudah fix berasal dari run seluruh package repository PostgreSQL.

| Endpoint / akun | Sebelum HTTP | Sebelum (s) | Sesudah HTTP | Sesudah (s) |
|---|---:|---:|---:|---:|
| Home user | 504 | 5.005 | 200 | 2.778 |
| Home mitra | 504 | 5.002 | 200 | 1.808 |
| Misi user | 504 | 5.001 | 200 | 2.707 |

Wallet dan badge tetap 200. Regression memeriksa isi yang terlihat konsumen: saldo 225, setoran terklaim 1, berat 1.500 kg, ID aktivitas, misi mingguan, role mitra dan badge pertama. Ekspektasi bulanan/mingguan mengikuti timestamp klaim fixture dan batas periode respons, sehingga tidak mengasumsikan test selalu selesai sebelum pergantian bulan/minggu. Katalog badge boleh bertambah. Test juga memeriksa koneksi tidak tertahan.

Verifikasi yang dijalankan:

- Baseline merah: tiga subtes target FAIL 504, dua kontrol PASS; parent test ikut FAIL sesuai harapan.
- Run hijau awal: kelima subtes PASS pada reproduksi yang sama.
- Seluruh `internal/repository/postgres`: **30 tes top-level + 115 subtes PASS, 0 FAIL/SKIP**. Termasuk pagination/filter/ownership, Home terisi dan kosong, satu koneksi, gamifikasi/claim/replay, notifikasi, reward, artikel, serta test latensi final.
- `go vet -p 1 -mod=readonly ./internal/repository/postgres`: exit 0.
- File Go terdampak diformat dengan gofmt. Cluster baseline, hijau dan regresi seluruhnya terkonfirmasi dihentikan/dihapus.

Runner PostgreSQL sementara dibersihkan dan tidak ikut commit. Tidak menjalankan full suite Go lintas package secara lokal; CI menjalankan job normal dan Newman full flow setelah publikasi.

## Menjalankan ulang

Dengan `ECOSNAP_TEST_ADMIN_URL` menunjuk PostgreSQL disposable loopback sesuai guard fixture:

```text
go test -p 1 -mod=readonly -count=1 -v ./internal/repository/postgres -run '^TestReadPathsUnderNetworkLatency$'
```

Untuk pemilik yang sudah menjalankan layanan asli: hentikan backend lama dengan Ctrl+C, lalu `go run ./cmd/app` dari terminal yang masih menyimpan environment database dan Firebase. Di terminal smoke jalankan:

```powershell
./scripts/smoke-test.ps1 -ApiKey $key -FlowsOnly
```

**Smoke nyata setelah fix belum tersedia.** Kirim ringkasan PASS/FAIL dan log request ID/durasi bila masih timeout. Jangan langsung mengulang full smoke, migration atau seed. Perbaikan menghilangkan kegagalan akibat round trip serial pada reproduksi terkontrol; bukan SLA jaringan dan bukan bukti bahwa semua kemungkinan lock/query lambat di Supabase telah disingkirkan. Android tetap belum diuji.
