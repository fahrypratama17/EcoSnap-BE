# Verifikasi fondasi server — 22 November 2026

## Cakupan

Hardening backend tanpa menambahkan aturan akun/finansial atau mengaktifkan AI, game, hosting, Android, Docker, maupun layanan cloud. Endpoint tetap 46. Tidak mengubah migration SQL, kredensial, atau `.env.example` milik pengguna.

Perubahan: lifecycle HTTP dengan graceful drain; anggaran pool PostgreSQL per proses; request ID, deadline kooperatif, error routing JSON, logging terbatas, dan pemulihan panic. Kontrak operasional ada di [index API](../api/README.md#10-operasi-server-dan-batas-koneksi).

## Perintah dan hasil

PostgreSQL disposable di loopback Windows, bukan `DATABASE_URL` atau Supabase. Runner memakai `GOMAXPROCS=2`, `GOMEMLIMIT=128MiB`, shared buffers PostgreSQL 32 MB dan maksimum 30 koneksi. Semua migration 00001–00012 diterapkan pada database pengujian yang terisolasi. Cluster dihentikan dan direktori datanya dihapus setelah pengujian.

```text
go test -p 1 -mod=readonly -count=1 -v ./...
go vet -p 1 -mod=readonly ./...
go build -p 1 -mod=readonly ./...
```

| Pemeriksaan | Hasil |
|---|---|
| Tes top-level | 90 PASS |
| Subtes | 134 PASS |
| Gagal / dilewati | 0 / 0 |
| go vet | PASS |
| go build | PASS |
| Runtime HTTP + PostgreSQL | PASS |

Hitungan diekstrak dari log menggunakan Python. Baseline sebelumnya 68 top-level + 120 subtes tetap tercakup.

## Skenario regresi yang dibuktikan

- TCP sungguhan: shutdown menunggu request aktif, menolak koneksi baru, menutup paksa setelah grace period dan membatalkan context handler, mengembalikan error listener, serta menerima context yang sudah dibatalkan. Jalur ini memakai pembatalan context terkontrol, bukan injeksi Ctrl+C/SIGTERM ke proses `cmd/app`.
- Pool satu koneksi: acquire kedua melewati deadline saat koneksi ditahan; setelah dilepas, acquire/query kembali berhasil. `pg_sleep(10)` dibatalkan dan query berikutnya berhasil tanpa koneksi acquired tersisa.
- Opsi pool URI/keyword tetap dihormati. Konfigurasi negatif, health-check nol, serta error konfigurasi tidak membocorkan URL/rahasia. Mode eksekusi pooler port 6543 tetap dipertahankan.
- Built-in 404/405 menjadi envelope JSON; `Allow`, HEAD, redirect native mux, business 404, dan unduhan biner tetap dipertahankan.
- Request ID baru dibuat server; request logging tidak menyertakan raw URL/query/body/token. Deadline autentikasi menjadi 504, bukan token-invalid 401.
- Panic sebelum commit menghasilkan 500 generik. Panic setelah body/header/flush tidak menambahkan JSON ke unduhan: koneksi dibatalkan. Flush yang tidak didukung tidak dianggap sudah melakukan commit.

Temuan selama verifikasi: import tes yang kurang diperbaiki; pengujian redirect diubah untuk membandingkan perilaku mux asli pada Go yang dipakai, bukan mengunci status 301; regresi flush dan nilai durasi pool yang semula diterima ditutup dengan tes yang sebelumnya gagal. Hasil tabel adalah eksekusi final sesudah perbaikan, bukan percobaan awal yang gagal.

## Observasi beban lokal

Skrip sementara menjalankan `bootstrap.NewServer` + `bootstrap.Serve` pada listener TCP loopback, PostgreSQL nyata dengan 12 migration, serta verifier token sintetis. Sebanyak 10 worker melakukan 200 request `GET /api/v1/me/wallet` milik akun fixture. Body respons juga didekode dan diperiksa; saldo fixture tetap nol.

| Metrik | Hasil |
|---|---:|
| Request HTTP 200 | 200 / 200 |
| Request ID valid dan unik | 200 |
| Error klien/decode | 0 |
| Rata-rata sampai header | 2,202 ms |
| Median sampai header | 1,089 ms |
| p95 sampai header | 2,987 ms |
| Maksimum sampai header | 51,683 ms |
| Koneksi pool maksimum teramati | 4 |
| Batas pool yang dikonfigurasi | 4 |
| Koneksi acquired setelah beban | 0 |
| Saldo setelah beban | 0 |
| Error shutdown HTTP | 0 |

Latensi diukur sampai `http.Client.Do` menerima header, bukan sampai seluruh body selesai dibaca. Statistik dihitung dengan Python; p95 memakai `statistics.quantiles(n=100, method="inclusive")`. Ini observasi satu endpoint lokal, bukan SLA, uji durasi panjang, kapasitas Supabase, maupun bukti autentikasi Firebase asli.

## Batas bukti

- Tidak menjalankan Firebase asli, Supabase remote, Android, Docker atau hosting; tidak mengklaim migration 00011–00012 sudah Applied di remote.
- Tidak menginjeksi sinyal OS ke executable CLI. Grace HTTP 10 detik bukan batas total proses: penutupan pool masih dapat menunggu pemegang koneksi.
- Deadline request bersifat kooperatif; bukan buffering/kill goroutine dan bukan jaminan transaksi belum commit ketika klien menerima timeout.
- Error parser protokol HTTP sebelum handler tidak dijanjikan memiliki JSON/request ID.
- RLS, retensi, role admin, quota cloud, dan penghapusan akun produksi bukan hasil audit dari batch ini; lihat [model data runtime](../data-model.md).
- CI baseline `a97c05b` teramati sukses: [run 44928399202](https://github.com/fahrypratama17/EcoSnap-BE/actions/runs/44928399202). CI untuk commit batch ini harus dilihat setelah push; hasil lokal tidak menggantikannya.

Skrip proof sementara dihapus setelah dipakai. Log dan data ukur lokal: `%TEMP%/ecosnap-foundation-final.log` dan `%TEMP%/ecosnap-foundation-runtime.json`; tidak berisi kredensial cloud.
