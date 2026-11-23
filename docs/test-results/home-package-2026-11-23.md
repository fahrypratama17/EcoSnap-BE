# Paket beranda dan handoff Android — 23 November 2026

## Perubahan

`GET /api/v1/me/home` kini mengembalikan nama caller, saldo, hitungan setoran bulan WIB, misi, satu saran mitra global, artikel terbaru dan jumlah inbox belum dibaca. Field lama tetap ada dengan arti all-time. Query parameter dan signature publik UserHome tidak berubah; tidak ada route atau migration baru.

Seluruh kartu, termasuk aktivitas terbaru yang sebelumnya diambil dalam transaksi kedua, dibaca dalam satu transaksi PostgreSQL REPEATABLE READ pada satu koneksi. Query riwayat, artikel, mitra dan unread dipakai bersama endpoint asal; pembentukan respons misi memakai builder yang sama dengan `/me/missions`. Tidak ada pemanggilan jaringan antar-kartu atau goroutine fanout.

`month.from` tanggal 1 pukul 00.00 WIB, inklusif; `month.to` sama dengan `server_time`, eksklusif. Hitungan mengikuti `deposit_claims.claimed_at`, bukan tanggal pembuatan/item. Waktu voucher aktif juga dibandingkan dengan sampel `server_time`. Visibilitas artikel tetap memakai `now()` transaksi sebagaimana endpoint artikel, bukan jaminan tepat pada sampel wall-clock Home.

## Verifikasi yang dijalankan

Runner sementara `.proof-home/run.ps1` menyiapkan cluster PostgreSQL portable loopback disposable dengan guard penyimpanan, lalu menjalankan:

```powershell
go test -p 1 -mod=readonly -count=1 -v ./internal/service ./internal/repository/postgres ./internal/delivery/http
go vet -p 1 -mod=readonly ./internal/service ./internal/repository/postgres ./internal/delivery/http
go build -p 1 -mod=readonly ./...
```

Hasil: **83 tes top-level + 140 subtes PASS, 0 FAIL/SKIP** untuk tiga package terdampak. Vet tiga package tersebut dan build seluruh package exit 0. Ini bukan pengulangan full suite `go test ./...`. Semua hitungan di laporan ini diambil dari output aktual dengan Python eval.

Runner menghasilkan `HOME_PROOF_CHECKS_PASS`, PostgreSQL berhenti normal dan `HOME_PROOF_CLUSTER_REMOVED`. Log lokal: `%TEMP%/ecosnap-home-proof.log`. Runner sementara dihapus setelah bukti disimpan.

Format Go: 10 file perubahan sesuai `gofmt` setelah normalisasi CRLF checkout di memori (tanpa mengubah line ending file pengguna). Sebelum normalisasi, dua file checkout CRLF dilaporkan oleh `gofmt -l`; bukan perubahan format source. Sebanyak 61 tautan lokal/anchor pada lima dokumen integrasi diperiksa tanpa tautan rusak pada pemeriksaan tersebut. Contoh JSON Home valid dan aritmetika saldo/ledger contoh konsisten; pemeriksaan ini tidak menjalankan Android.

## HTTP nyata dengan PostgreSQL

Tes baru `TestHomePackageIntegration` memakai `bootstrap.NewServer` asli melalui `httptest.NewServer`: handler, business logic dan repository asli. Pool aplikasi dibatasi **MaxConns=1**; semua kartu dan endpoint refresh berhasil tanpa acquire bersarang. Verifier token **sintetis**, bukan Firebase nyata. Database fixture terpisah menerapkan migration 00001–00012.

Enam skenario konsumen:

1. **Batas bulan dan kartu:** klaim tepat awal bulan WIB masuk, satu mikrodetik sebelumnya tidak masuk, bulan depan tidak masuk hitungan bulan berjalan. Klaim multi-item dihitung sekali; setoran dibuat tahun sebelumnya tetap dihitung menurut waktu klaim. Nama, saldo dan ID kartu benar; artikel draft/terjadwal tersembunyi dan waktu publikasi seri memakai ID stabil.
2. **Misi dan refresh:** katalog Home sama dengan `/me/missions`; token periode dari Home dapat diklaim. Klaim pertama 201, retry 200; refresh menunjukkan 50 XP dan status claimed tanpa menambah saldo EcoPoints atau menggandakan inbox.
3. **Isolasi dan unread:** akun lain tidak memperoleh saldo/aktivitas caller; read notifikasi milik orang lain 404, read sendiri idempoten. Refresh Home memperbarui jumlah unread; preferensi false tidak menghapus inbox lama.
4. **Visibilitas dan kosong:** pemilik mitra nonaktif/lokasi suspended/pending tidak menjadi saran. Tanpa data, `partner`/`article` adalah null dan `recent_activity` adalah array kosong, bukan demo.
5. **Kompatibilitas mitra:** mitra aktif tetap bisa membaca `/me/home`, dengan `missions:null`; `/partner/home` tetap read-only saat lokasi suspended.
6. **Akses:** tanpa token 401, profil missing 404, suspended 403 tanpa data kartu.

Distribusi status pada skenario HTTP baru (termasuk penolakan yang memang diharapkan):

| Status | Jumlah |
|---|---|
| 200 | 19 |
| 201 | 1 |
| 404 | 2 |
| 401 | 1 |
| 403 | 1 |

Sampel respons aktual: `user_name=Siti Home`, saldo **225**, `monthly_deposits=2`, `deposits_claimed=4`, unread **4**. Setelah klaim misi + retry: saldo tetap 225, XP 50, unread 5. Fixture batas waktu memakai klaim historis sintetis nol poin dari helper gamifikasi yang sudah ada; pendanaan wallet memakai transaksi setoran/QR/klaim asli sebesar 1.500 kg. Fixture sintetis tidak berarti endpoint produksi menerima klaim nol poin atau waktu klaim dari klien.

## Handoff untuk integrator

- [README API](../api/README.md): konfigurasi, token, daftar endpoint, error dan prasyarat migration.
- [Kontrak Home + mapping HomeUiState](../api/wallet-history.md#get-apiv1mehome): nama field, null/array, ID navigasi, bulanan vs all-time dan XP vs EcoPoints.
- [Panduan flow Android](../api/android-integration.md): sesi/register recovery, role/pending, onboarding, peta/favorit, transaksi/retry, reward, misi, profil/settings/logout, PDF, inbox dan bantuan; termasuk refresh serta pemisahan key per UID.

## Batas dan prasyarat

- **Home baru membutuhkan migration 00011** karena membaca notifications; untuk flow tiket bantuan dibutuhkan 00012. Keduanya belum diterapkan ke Supabase oleh AI. Tidak ada fallback nol jika tabel belum tersedia.
- Mitra pada Home adalah saran global menurut urutan daftar, bukan hasil GPS terdekat. Artikel/mitra null harus ditangani FE; katalog misi tidak menetapkan prioritas satu kartu.
- Tidak mengubah source/desain Android, menjalankan emulator/perangkat, memakai Firebase/Supabase nyata, atau membuktikan DTO/network client Android. Flow sudah memiliki kontrak BE, tetapi integrasi FE tetap pekerjaan tim.
- AI/game/media/admin, banding/refund, hapus akun dan hosting yang belum diputuskan tetap tidak dibuat. Ini bukan klaim seluruh aplikasi atau deployment selesai.
