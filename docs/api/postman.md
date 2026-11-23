# Koleksi Postman EcoSnap

File: [`ecosnap.postman_collection.json`](ecosnap.postman_collection.json) (Postman v2.1) dan [`ecosnap.postman_environment.json`](ecosnap.postman_environment.json). Koleksi memuat **111 request** yang menutup **46 route** (semua route di `router.go`/`support_routes.go`), dikelompokkan per alur dan role (`[USER]`, `[MITRA]`, `[PUBLIC]`; `[WRITE]` = mengubah database). Request negatif (400/403/404/409/410) dihitung dalam 111.

> **Peringatan:** koleksi ini mengubah data (setoran, klaim poin, voucher, tiket, favorit, misi). **Bukan** untuk bulk runner pada database nyata. Pakai database development/disposable.

## Prasyarat

1. Migration `00001`–`00012` diterapkan ([runbook](../supabase-migration-runbook.md)).
2. Seed: mitra + lokasi verified (`cmd/seed-partners --demo --mitra-uid …`), tarif (`cmd/seed-point-rates --demo`), reward (`cmd/seed-rewards --demo`), artikel (`cmd/seed-articles --demo`). Kategori `plastic` dan `paper` wajib ada dan diterima lokasi mitra.
3. Dua akun Firebase: satu `user`, satu `mitra` (lokasi verified, pemilik lokasi). Profil dibuat dengan folder `01-register` bila belum ada.
4. Saldo user harus mencukupi reward tersedia termurah. Satu setoran koleksi (5,000 kg plastik + 1,000 kg kertas) menghasilkan poin sesuai tarif aktif; sesuaikan bila tarif rendah.

## Impor dan environment

Impor kedua file, pilih environment **EcoSnap Local**.

| Variabel (environment) | Nilai |
|---|---|
| `base_url` | `http://127.0.0.1:8080` |
| `user_token` | kosong, tipe secret: tempel Firebase ID token akun user |
| `mitra_token` | kosong, tipe secret: tempel Firebase ID token akun mitra |

Tidak ada request login. Ambil ID token dari Firebase SDK (`getIdToken(true)`); token berlaku sekitar 1 jam dan **disegarkan manual**. Klaim QR dan penukaran reward memeriksa token dicabut, jadi pakai token segar. Setiap guard (token/variabel wajib kosong, `base_url` kosong, write ke host non-lokal) memanggil `pm.execution.skipRequest()` **sebelum** `throw`, karena `throw` saja tidak menjamin runner berhenti mengirim. `skipRequest` memerlukan Postman desktop terbaru atau Newman 6; versi lama akan gagal dengan TypeError. Bahwa tidak ada HTTP terkirim belum dibuktikan oleh file ini. Jangan menaruh token di koleksi atau commit environment berisi token.

## Scope variabel

- **Environment** hanya `base_url`, `user_token`, `mitra_token`. Jangan menambah variabel environment bernama sama dengan variabel koleksi: environment mengalahkan collection variable, sehingga nilai kosong akan menimpa hasil tangkapan.
- **Collection variables** (via `pm.collectionVariables`): ID sementara (`partner_id`, `deposit_id`, `voucher_id`, `ticket_id`, `notification_id`, …), `qr_token`, snapshot periode `mission_period_start`, cursor `cursor_*`/`p1_*`, dan key idempotency `idem_*`. Nilai awal kosong di file ekspor.
- Variabel lokal (`pm.variables`) hanya untuk rentang waktu laporan (`range_from`, `range_to`), dihitung per request.
- Token QR dihapus setelah replay klaim. Kode voucher tidak pernah disimpan (hanya dicek ada). Token Firebase tidak pernah ditulis oleh script. Key `idem_*` sengaja dipertahankan (bukan rahasia; hilangnya key menghilangkan jaminan replay) dan hanya dihapus lewat request HELPER manual. Sebelum mengekspor koleksi dari Postman, jalankan `HELPER Clear cursors and transient secrets` agar `qr_token` dan cursor kosong; kosongkan `idem_*` hanya bila intent sudah selesai.
- `seed_partner_id` = ID partner seed (`20000000-0000-4000-8000-000000000001`): dipakai untuk memilih partner bila ada, selain itu item pertama. ID kategori, reward, dan artikel **ditangkap dari respons katalog saat dijalankan**, bukan ditanam. [INFERENCE] ID seed di repo tidak diverifikasi terhadap runtime di sini.

## Urutan aman (database disposable)

Jalankan folder berurutan (Collection Runner atau `newman run … --folder "<nama>"` berulang):

1. `00 Health`
2. `01-register (sekali per akun Firebase)` — lewati bila profil sudah ada dengan data lain (409)
3. `01-user (profil & settings)`, `01-mitra (profil & settings)`
4. `02 Katalog (baca)` — menangkap kategori, partner, artikel
5. `03 Favorit (user)`, `04 Estimasi poin (user)`
6. `05 Setoran & QR` (buat → replay → QR → pratinjau → klaim → replay klaim → status mitra)
7. `06 Saldo & riwayat`, `07 Reward & voucher`, `08 Misi, badge, leaderboard`
8. `09 Laporan aktivitas`, `10 Notifikasi`, `11 Bantuan & tiket`
9. Opsional: `12 Pagination cursor (butuh data >= 2 baris)`. Page 2 gagal keras bila page 1 tidak punya `next_cursor`.

Jangan sertakan `90 Helpers (manual, jangan di bulk run)` dalam run otomatis.

## Idempotency

`Idempotency-Key` dibuat **sekali per intent** oleh pre-request hanya bila variabel kosong (`idem_deposit`, `idem_redeem`, `idem_ticket`); tidak diacak ulang tiap kirim. Retry/timeout: kirim ulang request yang sama (key sama → 200, resource sama). Request `… REPLAY` terpisah memverifikasi 200 dan ID sama, tanpa membuat key. Karena key persisten, menjalankan ulang `Create deposit` setelah selesai hanya me-replay setoran lama. Untuk intent baru jalankan **manual**, setelah intent sebelumnya selesai: `HELPER New deposit intent`, `HELPER New redeem intent`, `HELPER New ticket intent`, `HELPER New mission snapshot`.

## Request khusus

- **PDF** (`/me/activity-report.pdf`): tidak di-parse sebagai JSON; test memeriksa header (`application/pdf`, `Content-Disposition`, `nosniff`, `no-store`) dan magic bytes `%PDF-`. Error PDF (query salah) tetap JSON dan diuji sendiri.
- **Misi:** `GET Missions` menyimpan `period_start` sebagai snapshot; klaim dan replay memakai nilai itu. Pergantian minggu membuat replay 410 `mission_expired` (benar).
- **Pagination:** cursor disimpan di collection variable per daftar, tidak dipakai lintas filter.
- Role mengikuti kontrak: mitra untuk setoran/QR/`partner/*`; user untuk klaim, reward, misi; endpoint "semua akun" memakai user atau mitra sesuai konteks.
- Redirect: perilaku bawaan Postman, tidak diubah.
- Request ke host non-lokal dengan method selain GET ditolak oleh pre-request koleksi kecuali collection variable `allow_remote_write=yes`.
- **Rerun misi:** `GET Missions` memilih misi `claimable`, atau bila tidak ada yang `claimable`, misi `claimed` pertama (tidak pernah `in_progress`). Klaim pada misi yang sudah claimed di periode yang sama dibalas 200 (idempotent) dan diterima oleh test (201 atau 200). Tanpa misi claimable/claimed, langkah klaim di-skip oleh guard variabel.

## Efek samping (tidak dipulihkan)

Tidak ada script yang mengembalikan state awal. Perubahan yang tertinggal pada akun uji:

- `[USER][WRITE] Patch profile (set full_name = 'Pengguna Uji EcoSnap', hardcoded)`: `full_name`/`display_name` user menjadi nilai tetap itu.
- `Patch settings`: `notification_enabled=true` (tidak berubah bila sudah true; `onboarding_completed` tidak disentuh). `Leaderboard visibility`: `visible=true`.
- Setiap run baru dengan key yang sudah dihapus helper menambah setoran, klaim poin, voucher (poin berkurang, stok reward berkurang), tiket, XP misi, dan notifikasi. Favorit dibersihkan oleh siklus PUT/DELETE.
- Pakai database disposable atau reset fixture sesudahnya.

## Validasi lokal

```bash
jq empty docs/api/ecosnap.postman_collection.json docs/api/ecosnap.postman_environment.json
jq '[.. | objects | select(has("request"))] | length' docs/api/ecosnap.postman_collection.json   # request
jq -r '[.. | objects | select(has("request")) | .request.method + " " + (.request.url.path | join("/"))] | length' docs/api/ecosnap.postman_collection.json
```

Cek sintaks seluruh script (Bun/Node):

```js
const c = JSON.parse(require('fs').readFileSync('docs/api/ecosnap.postman_collection.json', 'utf8'));
const walk = (x) => { (x.event || []).forEach((e) => new Function('pm', e.script.exec.join('\n'))); (x.item || []).forEach(walk); };
walk(c);
```

Jalankan hanya pada database disposable: `newman run docs/api/ecosnap.postman_collection.json -e docs/api/ecosnap.postman_environment.json --env-var user_token=… --env-var mitra_token=… --folder "00 Health" --folder "02 Katalog (baca)"` (subset baca saja aman).

## Status verifikasi

JSON memenuhi schema resmi Postman v2.1; 222 blok script lolos pemeriksaan sintaks dan 111 request memetakan seluruh 46 route. Newman pada server asli + PostgreSQL disposable menjalankan 104 request/354 assertion tanpa kegagalan; dua request registrasi akun yang sudah ada dan lima helper manual dikecualikan. Registrasi user/mitra baru diuji terpisah. Dua skenario guard terbukti tidak mengirim HTTP. Auth sintetis, bukan Firebase/Supabase nyata; [bukti dan batas lengkap](../test-results/integration-tools-2026-11-23.md).

## Full flow otomatis di CI

Job `newman-full-flow` berjalan pada push ke `main`/`dev` atau PR menuju keduanya. Go menyiapkan database PostgreSQL disposable dan server HTTP aplikasi asli; runner Newman memakai verifier Firebase sintetis khusus tes, bukan password/token Firebase nyata.

Koleksi tidak diedit saat tes: runner memakai salinan in-memory untuk registrasi akun baru, satu putaran utama, lalu putaran utama baru dengan pagination. Salinan baru adalah intent transaksi berbeda; replay dalam setiap putaran tetap memakai key yang sama. Folder helper manual tidak dijalankan. Putaran kedua menyediakan minimal dua setoran, voucher dan tiket agar halaman kedua benar-benar diuji.

Runner membandingkan jumlah request yang direncanakan dengan request yang selesai; guard `skipRequest` tidak boleh menghasilkan CI hijau palsu. Assertion gagal, error script/jaringan, respons hilang atau request tanpa pemeriksaan juga ditolak. Unit test runner menggunakan Newman nyata dan listener loopback untuk membuktikan jalur gagal.

Perintah lokal serta batas otomatisasi: [gate CI](README.md#gate-ci-kontrak-dan-smoke-offline). CI ini tidak otomatis mengintegrasikan Android, deploy server, atau menerapkan migration ke Supabase development.

Bukti lokal: [198 HTTP/677 assertion dan tujuh tes runner](../test-results/newman-full-flow-2026-11-24.md). Hasil GitHub dicatat terpisah dari hasil lokal.
