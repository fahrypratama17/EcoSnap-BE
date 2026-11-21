# Kontrak statistik aktivitas dan laporan PDF

Dua endpoint membaca aktivitas akun caller sendiri. Keduanya memakai `Authorization: Bearer <Firebase ID token>`; UID selalu dari token, tidak ada parameter akun. Akun harus `active`. Tidak ada file yang disimpan di server, tidak ada proses asinkron: PDF dibuat saat request dan dikirim langsung.

Verifikasi lokal pada worktree integrasi: tes PostgreSQL/HTTP/service dan render PDF dengan dua parser serta inspeksi visual lulus ([bukti integrasi dev](https://github.com/fahrypratama17/EcoSnap-BE/blob/dev/docs/test-results/backend-support-reports-notifications-2026-11-21.md)). Firebase/Supabase nyata dan Android untuk endpoint ini belum diuji.

| Endpoint | Hasil |
|---|---|
| `GET /api/v1/me/activity-report` | JSON statistik |
| `GET /api/v1/me/activity-report.pdf` | Unduhan PDF dengan isi yang sama |

## Filter

Query hanya `from` dan `to`; parameter lain atau berulang ditolak (400 `invalid_query`).

- Format RFC 3339 dengan zona eksplisit (`Z` atau offset), mis. `2026-11-01T00:00:00%2B07:00`. **Tanda `+` di URL harus ditulis `%2B`**, kalau tidak server membacanya sebagai spasi dan menolaknya.
- `from` inklusif, `to` eksklusif. `to` harus setelah `from`; rentang maksimum 366 hari (tepat 366 hari masih diterima). Waktu dibulatkan ke bawah ke mikrodetik.
- `from` dan `to` diisi keduanya atau tidak sama sekali. Tanpa keduanya: **bulan kalender berjalan WIB** dari tanggal 1 pukul 00:00 WIB sampai saat request (`to` = sekarang, tidak melewati waktu server).
- `to` eksplisit di masa depan **diperbolehkan**; hari yang belum terjadi muncul sebagai bucket bernilai nol. Hanya default yang dibatasi sampai sekarang.

## Apa yang dihitung

Hanya setoran yang **sudah diklaim**, diukur dengan **waktu klaim** (`claimed_at`), bukan waktu setoran dibuat. Setoran yang belum diklaim (`created`, `qr_issued`) tidak pernah dihitung.

- Role `user`: setoran yang diklaim akun ini.
- Role `mitra`: setoran milik lokasi mitra sendiri yang sudah diklaim. Lokasi yang tidak lagi `verified` tetap dapat membaca laporannya (read-only, sama seperti riwayat setoran). Identitas pengklaim tidak pernah ditampilkan.
- Setiap setoran dihitung sekali untuk jumlah, berat, dan poin walau berisi banyak item. Berat dan poin berasal dari angka yang tersimpan saat klaim/setoran (poin yang benar-benar dikreditkan), tidak dihitung ulang dari tarif sekarang.
- Kategori memakai nama kategori yang tersimpan pada item setoran, sehingga mengganti nama kategori tidak mengubah laporan lama. `deposit_count` per kategori = jumlah setoran berbeda yang memuat kategori itu, jadi **tidak dijumlahkan** (satu setoran dapat memuat beberapa kategori); berat per kategori dijumlahkan sama dengan berat total.
- **Saldo dompet dan total sepanjang masa bukan metrik periode** dan tidak ada di laporan. Poin diperoleh pada periode bisa berbeda dari saldo.
- Role `user` mendapat `points_earned` (dikreditkan dari klaim) dan `points_spent` (penukaran reward pada periode, angka positif) terpisah; tidak dinetkan. Role `mitra` mendapat `points_credited` (poin yang diterima pengguna dari setoran lokasinya) dan tidak memiliki `points_spent`.

Seluruh angka dibaca dalam satu snapshot database (`REPEATABLE READ`), jadi status akun, kepemilikan, total, dan rincian harian konsisten satu sama lain.

## Respons JSON

```json
{"data":{
  "account":{"role":"user","full_name":"Siti"},
  "period":{"from":"2026-11-01T00:00:00+07:00","to":"2026-11-04T00:00:00+07:00","timezone":"Asia/Jakarta","days":3},
  "generated_at":"2026-11-21T03:00:00Z",
  "totals":{"claimed_deposits":2,"total_weight_grams":3500,"total_weight_kg":"3.500","points_earned":490,"points_spent":100},
  "daily":[{"date":"2026-11-01","claimed_deposits":1,"total_weight_grams":1500,"total_weight_kg":"1.500","points_earned":190,"points_spent":0}],
  "categories":[{"category_id":"…","category_name":"Plastik","deposit_count":2,"total_weight_grams":3000,"total_weight_kg":"3.000"}]
}}
```

- `daily` berisi satu entri untuk setiap tanggal kalender **WIB** (UTC+7) pada rentang, urut naik, tanpa tanggal terlewat (hari tanpa aktivitas bernilai nol). Hari pertama dan terakhir dibatasi oleh `from`/`to`, jadi bisa hanya sebagian hari. `period.days` = jumlah entri `daily`. `totals` = jumlah dari `daily`.
- `period.from`/`to` ditampilkan dalam WIB; instan yang sama dengan masukan.
- `categories` urut berat menurun; array kosong bila tidak ada setoran.
- `account.business_name` hanya ada untuk mitra.

## PDF

- `Content-Type: application/pdf`, `Content-Disposition: attachment; filename="ecosnap-activity-report-YYYYMMDD-YYYYMMDD.pdf"` (tanggal WIB hari pertama dan terakhir), `Content-Length`, `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`. Error tetap JSON biasa (bukan PDF).
- Isi: akun, periode, zona waktu, waktu dibuat, tabel ringkasan, tabel harian lengkap dengan baris jumlah, dan tabel kategori. Tabel dilanjutkan ke halaman berikutnya dengan header diulang dan nama panjang dibungkus, tanpa pemotongan. Nomor halaman "Halaman X dari N".
- Font Go (BSD) ditanam di PDF; huruf Latin beraksen, Yunani, dan Sirilik tampil benar. **Batasan:** karakter di luar font (mis. CJK, emoji) tidak dapat digambar; karakter itu diganti `�` dan PDF memuat catatan jumlahnya, bukan diganti `?` diam-diam.
- PDF adalah rekap aktivitas setoran, bukan dokumen keuangan atau bukti resmi.

## Error

| Kondisi | Status | Kode |
|---|---|---|
| `from`/`to` tidak valid, hanya satu diisi, tanpa zona, terbalik, >366 hari, parameter asing/ganda | 400 | invalid_query |
| Tidak ada/invalid token | 401 | unauthorized |
| Akun tidak aktif | 403 | account_forbidden |
| Mitra tanpa lokasi | 403 | deposit_forbidden |
| Profil belum dibuat | 404 | account_not_found |
| Batas waktu 10 detik terlampaui | 504 | request_timeout |
| Kegagalan lain | 500 | internal_error |
