# Verifikasi estimasi poin multi-material

Tanggal: 20 November 2026. Branch: feat/deposit-estimation, dibuat dari dev pada 937a9b7. Perbaikan runner chore/test-runner-preflight telah digabung ke dev secara fast-forward. Baseline db51b4b dipertahankan; main tetap 071320d. Tidak ada push.

## Pemeriksaan baseline

Setelah ruang C tersedia sekitar 6,9 GB, runner PostgreSQL portable menyelesaikan seluruh suite baseline dengan exit 0 tanpa SKIP. Build baseline juga exit 0. Hasil ini menutup GIT-03; kegagalan disk pada checkpoint sebelumnya tetap tercatat sebagai riwayat. Tidak ada penghapusan data untuk menyediakan ruang pada batch ini.

## Pengujian fitur

Seam yang diuji adalah PointEstimationService.Estimate, HTTP POST /api/v1/point-estimates, repository tarif melalui service dengan PostgreSQL nyata, serta CLI seed-point-rates. Tes service awal gagal kompilasi karena interface belum tersedia. Tes HTTP awal juga gagal karena route/handler belum tersedia; keduanya lulus setelah implementasi.

Run PostgreSQL pertama gagal pada fixture lokasi yang belum mengisi area/latitude/longitude NOT NULL. Fixture diperbaiki agar mengikuti schema yang sudah ada; constraint tidak dilonggarkan. Suite lengkap kemudian diulang melalui:

```powershell
./scripts/test-postgres-portable.ps1 -PostgresBin (Join-Path $env:LOCALAPPDATA 'EcoSnapTestPostgres/minimal/pgsql/bin')
```

Runner menjalankan `go test -p 1 -mod=readonly -count=1 -v ./...` terhadap PostgreSQL portable 17.12 pada cluster disposable loopback. Hasil akhir exit 0, seluruh tes PASS, tanpa SKIP. Server mencetak server stopped dan direktori cluster run dibersihkan.

| Tes | Perilaku yang dibuktikan | Hasil |
|---|---|---|
| TestPointEstimateFloorsAfterTotal | Dua item masing-masing 0,6 poin menghasilkan total 1, bukan 0. | PASS |
| TestPointEstimatePrecisionAndLimits | Pecahan tarif 1,23, berat minimum, total maksimum 1 miliar poin, invalid ID/partner, batas total berat dan jumlah item. | PASS |
| TestPointEstimateRejectsInvalidWeightsAndDuplicates | Nol/negatif, koma, exponent, NaN, berat terlalu besar, presisi berlebih, kategori ganda ditolak. | PASS |
| TestPointEstimateHTTP | Envelope, hasil floor, item dan penanda demo. | PASS |
| TestPointEstimateHTTPRejectsInvalidRequests | JSON number untuk berat, tarif client, trailing JSON, null, duplikasi, query override, body besar, auth dan content type. | PASS |
| TestPointEstimateHTTPMapsErrors | 400/404/409/504/500 sesuai error; detail internal tidak dikirim. | PASS |
| TestPointEstimationIntegration | Empat kelompok: global/lokasi/fallback/versi terbaru; expired/future dikesampingkan; material tidak diterima/kategori/lokasi hilang; lokasi pending/owner suspended; estimasi berulang tidak mengubah wallet. | PASS |
| TestPointRateSeedCommandIntegration | CLI dua kali idempotent; perubahan harga dengan ID sama ditolak; tarif yang tersimpan tetap. | PASS |
| Suite sebelumnya | Profil, akun dan rollback/race 16 request, favorit/race 12 request, query mitra, seed mitra tetap lolos. SQL migration 00001-00005 Up/Down/reapply. | PASS |

Pesan CLI exit status 1 pada output suite adalah skenario penolakan yang memang diuji: seed mitra untuk role user serta penimpaan tarif dengan ID sama. Kedua tes berakhir PASS. Kegagalan fixture pada run pertama telah dijelaskan di atas dan tidak disembunyikan oleh skenario penolakan tersebut.

Pemeriksaan akhir `go vet -p 1 -mod=readonly ./...` dan `go build -p 1 -mod=readonly ./...` exit 0. Proses memakai GOMAXPROCS=2 dan GOMEMLIMIT=128MiB untuk membatasi kebutuhan resource. Pemeriksaan formatting dan whitespace juga dicatat pada checkpoint BUILD_PROCESS.md.

## Batas hasil

Database tes memakai data sintetis; tidak ada mutation Supabase, credential/token Firebase nyata, atau integrasi runtime Android. Tes SQL Up/Down tidak memverifikasi version tracking Goose. Estimasi tidak membuat setoran, QR, ledger, maupun saldo. Pemakaian ulang aturan hitung pada pembuatan setoran tetap pekerjaan DEPOSIT-03. Tarif dan batas teknis masih konfigurasi demo yang terdokumentasi.
