# API EcoSnap untuk tim Android

Mulai dari sini. Dokumen ini adalah indeks, konvensi umum, ringkasan alur per layar, dan katalog error. Detail kolom per endpoint ada di dokumen fitur (bagian 4); urutan panggilan, retry, refresh, dan perubahan yang diperlukan di Android ada di [android-integration.md](android-integration.md).

**Status:** 46 route, tanpa endpoint baru pada paket integrasi. OpenAPI valid dan cocok dengan 435 respons JSON/58 body request sukses dari 447 HTTP lokal, seluruh 46 operasi mendapat respons sukses. Newman: 104 request/354 assertion tanpa kegagalan; smoke penuh: 186 PASS, harness offline: 84 PASS ([bukti integrasi](../test-results/integration-tools-2026-11-23.md)). Server dan PostgreSQL asli, auth sintetis. Verifikasi kode Home sebelumnya tetap terpisah: [83 tes top-level + 140 subtes PASS, vet/build](../test-results/home-package-2026-11-23.md).

Checkpoint full suite sebelum paket Home: 90 tes top-level + 134 subtes PASS ([bukti fondasi](../test-results/server-foundation-2026-11-22.md)). Smoke nyata historis 57 PASS mendahului settings/laporan/bantuan/inbox/Home baru. Laporan pemilik 2026-11-24: migration **00001–00012 Applied**, `-SettingsOnly` **47 PASS**, lalu `-FlowsOnly` **66 PASS/3 FAIL** karena timeout Home dua peran dan misi. Laporan JSON/PDF, daftar inbox, FAQ dan daftar tiket terbaca, tetapi full flow layanan asli serta runtime Android belum selesai dibuktikan. Setelah perbaikan, ulangi `-FlowsOnly` dahulu; laporkan X-Request-ID dan durasi tanpa token/credential.

Perbaikan latensi Home/misi mempertahankan deadline 5 detik dan satu snapshot, tetapi mengurangi pertukaran SQL serial. Reproduksi dengan latensi PostgreSQL terkontrol berubah dari tiga 504 menjadi seluruhnya 200; [bukti, regresi dan batasnya](../test-results/supabase-read-latency-2026-11-24.md). Smoke Firebase/Supabase setelah fix masih menunggu hasil pemilik.

> **Prasyarat database:** Home baru membaca `unread_count` dari tabel notifikasi (migration 00011), jadi **terapkan migration 00011 sebelum memakai `GET /me/home` versi baru**; tabel tidak ada → request gagal, tidak ada fallback `0`. Seluruh flow, termasuk bantuan/tiket, memerlukan 00011 **dan** 00012. Tidak ada migration baru di paket Home. Ini rujukan tunggal; dokumen lain menautkan ke sini.

**Paket integrasi:** [OpenAPI 3.0.3](openapi.yaml) mencakup semua 46 route untuk acuan DTO/client. Impor [koleksi Postman](ecosnap.postman_collection.json) dan [environment kosong](ecosnap.postman_environment.json), lalu ikuti [panduan Postman](postman.md) untuk token dua peran, urutan flow, replay idempotency, dan efek samping. Jangan menjalankan seluruh koleksi pada database nyata; registrasi dan helper intent baru dipilih manual.

**Kontrak model AI (service terpisah):** [Panduan scan dan DTO](ai-scan.md) serta [OpenAPI 3.1 JSON](ai-scan.openapi.json). Implementasi saat ini ada di folder **`ai-service/`** repo **EcoSnap-BE** (FastAPI mandiri, port **8000**, `POST /api/scan`, provider Groq dengan model `qwen/qwen3.8-27b`, konfigurasi `GROQ_API_KEY` dari `.env` root BE); asal historisnya repo EcoSnap-FE branch `13-feat/ai-waste-scan` (referensi upstream terbaru `4310cd7dac442b71130f375ab61a4025d298686c`, lihat [ai-scan.md](ai-scan.md)). Service ini **bukan bagian dari 46 route Go port 8080**; aturan Firebase Bearer, envelope respons, dan deadline Go di bawah tidak berlaku padanya, dan tidak ada port tunggal bersama (Emulator: Go `10.0.2.2:8080`, AI `10.0.2.2:8000`). Autentikasi Firebase dan penyimpanan riwayat scan belum terintegrasi.

## 1. Alamat server

| Lingkungan | Base URL |
|---|---|
| Backend lokal | `http://127.0.0.1:8080` |
| Android Emulator → backend di laptop | `http://10.0.2.2:8080` |
| Perangkat fisik | Alamat IP laptop di jaringan yang sama; server harus dijalankan dengan `HTTP_ADDR=0.0.0.0:8080` dan firewall mengizinkan port 8080 |
| Demo/produksi | Belum ada (hosting menunggu keputusan Q-15) |

Backend memakai HTTP biasa saat development, jadi Android perlu mengizinkan cleartext untuk host tersebut (`network_security_config`). Jangan mengaktifkan cleartext untuk seluruh aplikasi.

## 2. Autentikasi

- Login/registrasi/reset password dilakukan **Firebase Auth di Android**. Backend tidak menerima atau menyimpan password.
- Setiap request (kecuali `GET /health`) membawa `Authorization: Bearer <Firebase ID token>`.
- Ambil token dengan `FirebaseUser.getIdToken(false)` **sebelum setiap request** (SDK menyegarkannya otomatis). Token berlaku sekitar 1 jam.
- Respons **401** (`unauthorized`): panggil `getIdToken(true)` satu kali lalu ulangi request. Jika tetap 401, arahkan ke layar login.
- Identitas (UID) selalu dari token. Jangan mengirim UID di body atau query; field itu ditolak.
- **Klaim QR dan penukaran reward** juga mengecek apakah akun Firebase dinonaktifkan atau token dicabut. Itu menambah satu panggilan jaringan di server, jadi dua endpoint ini sedikit lebih lambat.
- Setelah Firebase sign-up berhasil, **profil harus dibuat di backend** lewat `POST /api/v1/accounts`. Jika langkah ini gagal (jaringan putus), ulangi saja: endpoint idempotent.

## 3. Konvensi yang berlaku di semua endpoint

| Hal | Aturan |
|---|---|
| Format | JSON, `Content-Type: application/json` untuk body. Field memakai `snake_case`. |
| Sukses | `{"data": …}`; daftar menambah `"pagination": {…}` |
| Error | `{"error": {"code": "…", "message": "…"}}`. Percabangan logika memakai `code` + status HTTP; `message` boleh ditampilkan ke pengguna. |
| ID | UUID string. UID akun adalah UID Firebase. |
| Waktu | RFC 3339 dengan zona, mis. `2026-11-21T08:00:00Z`. Batas periode Home/laporan dapat memakai WIB `+07:00`; parse sebagai instant, jangan menganggap semua timestamp berakhiran `Z`. Format tanggal tampilan dibuat Android. |
| Poin | Integer (bukan desimal). |
| Berat | **String desimal bertitik**, maks. 3 desimal, mis. `"2.500"`. Jangan mengirim angka JSON, koma, atau notasi ilmiah. |
| Tarif/hasil per item | String desimal (`"150.00"`, `"375.00000"`) |
| Daftar kosong | Array `[]`, bukan `null` |
| Parameter asing | Query atau field body yang tidak dikenal → **400**. Jangan mengirim parameter tambahan. |
| Body | Maks. 16 KiB; khusus pembuatan tiket bantuan 64 KiB. Satu objek JSON saja, tanpa teks tambahan sesudahnya. |
| Idempotency-Key | Wajib pada `POST /deposits`, `POST /rewards/{id}/redeem`, dan `POST /me/support-tickets` (8–128 karakter `A-Z a-z 0-9 - _`). Lihat bagian 5. |
| Rute/metode yang tidak ada | Envelope JSON: 404 `not_found` atau 405 `method_not_allowed`; header `Allow` dipertahankan. Redirect normal dan HEAD tetap mengikuti net/http. |
| Korelasi | Setiap respons dari handler aplikasi membawa `X-Request-ID` baru buatan server. Kirim ID ini saat melapor; jangan kirim token/body sensitif. ID masukan klien tidak dipercaya. |
| Deadline | Context request maksimal 15 detik, termasuk auth; operasi database umumnya 5 detik, laporan 10 detik. Deadline bersifat kooperatif, bukan penghentian paksa goroutine. Batas transport dapat menutup koneksi tanpa JSON. |

**Dua gaya pagination:**
- **Cursor** (riwayat poin, setoran mitra, voucher, artikel): kirim `limit` (1–100, default 20) dan `cursor` dari `pagination.next_cursor`. Berhenti jika `next_cursor` bernilai `null`. Cursor bersifat buram: jangan diurai, jangan disimpan permanen, jangan dipakai ulang dengan filter lain.
- **Offset** (daftar mitra, favorit): `limit` dan `offset`; berhenti jika `has_more` bernilai `false`.

## 4. Peta endpoint

Semua memerlukan Bearer token kecuali `/health`. Kolom "Siapa": **user**, **mitra** (lokasi verified), atau **semua** akun active.

| Endpoint | Siapa | Dokumen |
|---|---|---|
| `GET /health` | publik | [android-integration.md](android-integration.md#5-arsip-audit-frontend-17-november-2026) |
| `GET /api/v1/auth/me` | semua | [android-integration.md](android-integration.md#5-arsip-audit-frontend-17-november-2026) |
| `POST /api/v1/accounts` | semua (sekali) | [accounts.md](accounts.md) |
| `GET /api/v1/me` · `PATCH /api/v1/me` | semua | [accounts.md](accounts.md) |
| `GET /api/v1/me/settings` · `PATCH /api/v1/me/settings` | semua (active) | [accounts.md](accounts.md) |
| `GET /api/v1/waste-categories` | semua | [partners.md](partners.md) |
| `GET /api/v1/partners` · `GET /api/v1/partners/{id}` | semua | [partners.md](partners.md) |
| `GET /api/v1/me/favorites` · `PUT/DELETE /api/v1/me/favorites/{partnerID}` | semua | [partner-favorites.md](partner-favorites.md) |
| `POST /api/v1/point-estimates` | semua | [point-estimates.md](point-estimates.md) |
| `POST /api/v1/deposits` · `GET /api/v1/deposits/{id}` | mitra | [deposits.md](deposits.md) |
| `POST /api/v1/deposits/{id}/qr` | mitra | [deposits.md](deposits.md) |
| `POST /api/v1/deposit-claims/preview` · `POST /api/v1/deposit-claims` | user | [deposits.md](deposits.md) |
| `GET /api/v1/me/wallet` · `/me/point-history` · `/me/deposits/{id}` · `/me/home` | user (wallet: semua) | [wallet-history.md](wallet-history.md) |
| `GET /api/v1/partner/deposits` · `/partner/home` | mitra | [wallet-history.md](wallet-history.md) |
| `GET /api/v1/rewards` · `/rewards/{id}` · `POST /rewards/{id}/redeem` | user (redeem) | [rewards.md](rewards.md) |
| `GET /api/v1/me/vouchers` · `/me/vouchers/{id}` · `POST /me/vouchers/{id}/use` | user | [rewards.md](rewards.md) |
| `GET /api/v1/articles` · `/articles/{id}` | semua | [articles.md](articles.md) |
| `GET /api/v1/me/missions` · `POST /api/v1/me/missions/{missionID}/claim` · `GET /api/v1/me/badges` · `GET /api/v1/leaderboard` · `PATCH /api/v1/me/leaderboard-visibility` | user (klaim/visibility: user) | [gamification.md](gamification.md) |
| `GET /api/v1/me/activity-report` · `/me/activity-report.pdf` | semua (active; mitra perlu lokasi sendiri) | [activity-reports.md](activity-reports.md) |
| `GET /api/v1/help/faqs` · `POST/GET /api/v1/me/support-tickets` · `GET /me/support-tickets/{ticketID}` | semua (active) | [support.md](support.md) |
| `GET /api/v1/me/notifications` · `PATCH /me/notifications/{notificationID}/read` | semua (active) | [notifications.md](notifications.md) |

## 5. Alur per layar

### Registrasi dan masuk
1. Firebase `createUserWithEmailAndPassword` (atau login).
2. `POST /api/v1/accounts` dengan `role` (`user` atau `mitra`), `full_name`, `phone`; mitra menambah `business_name` dan `business_category`. Hasilnya profil lengkap. Panggilan ulang dengan isi sama aman.
3. Setiap kali aplikasi dibuka: `GET /api/v1/me` untuk memuat profil. **404 `account_not_found`** berarti Firebase sudah ada tetapi profil belum dibuat → ulangi langkah 2.
4. Mitra baru berstatus **pending**: `GET /me` menampilkan `business.status = "pending"`, belum punya lokasi, dan **tidak bisa membuat setoran/QR**. Persetujuan dilakukan manual oleh pemilik repo (D-21). Tampilkan banner "menunggu verifikasi".
5. Mitra yang sudah punya lokasi tetapi disuspend: `GET /partner/home` menampilkan `partner_status` (`verified` | `suspended` | `rejected` | `pending`). Riwayat tetap terbaca; pembuatan setoran dan QR ditolak 403.

### Pengaturan akun
`GET /api/v1/me/settings` memuat `notification_enabled`, `show_on_leaderboard`, `onboarding_completed`. `PATCH /api/v1/me/settings` menerima hanya `notification_enabled` dan/atau `onboarding_completed` (boolean; dihilangkan = tidak berubah; `false` sah; null/kosong/asing/query ditolak dengan 400 `invalid_request`). Visibilitas leaderboard tetap diubah lewat `PATCH /me/leaderboard-visibility` (hanya user). Setelah migration 00011, preferensi notifikasi mengendalikan pembuatan inbox baru, bukan push. Belum diuji dari Android.

### Beranda
- User: satu `GET /api/v1/me/home`: nama, saldo, `monthly_deposits` (bulan WIB), `deposits_claimed` dan `total_weight_kg` (all-time), voucher aktif, 3 aktivitas terbaru, daftar misi minggu ini (sama dengan `/me/missions`), satu mitra saran global (bukan terdekat), satu artikel terbaru, dan `unread_count`. `partner` dan `article` bernilai `null` bila kosong. Kolom lengkap dan pemetaan `HomeUiState`: [wallet-history.md](wallet-history.md#get-apiv1mehome).
- Mitra: `GET /api/v1/partner/home`. Mitra aktif yang memanggil `/me/home` mendapat `missions: null`.
- EcoPoints (saldo) bukan XP (hadiah misi); jangan menjumlahkan keduanya.

### Peta dan daftar mitra
`GET /api/v1/partners?q=&material=<slug>&limit=&offset=` dan detail per ID. Pencarian berdasarkan nama/area; belum ada radius/GPS (urutan jarak dihitung di Android jika perlu).

### Kalkulator estimasi
`POST /api/v1/point-estimates` dengan `items[]` (kategori + berat) dan `partner_id` opsional. Hasil bukan janji saldo: setoran nyata dihitung ulang server. `is_demo`/`uses_demo_rates` menandai tarif contoh.

### Mitra membuat setoran dan QR
1. Mitra mengisi item (kategori + berat sebagai string). Kategori ganda ditolak; gabungkan beratnya di Android.
2. Buat **`Idempotency-Key` baru untuk satu percobaan** (mis. UUID). `POST /api/v1/deposits`. Total poin dihitung server; jangan mengirim total.
3. **Jika timeout/jaringan putus: ulangi dengan key yang sama.** Hasilnya setoran yang sama (200). Jangan membuat key baru atau setoran bisa ganda.
4. `POST /api/v1/deposits/{id}/qr` → `token`, `expires_at`, `server_time`, `warning`.
5. Tampilkan `token` sebagai QR (encode apa adanya) dan **wajib tampilkan teks `warning`** di layar yang sama. Countdown = `expires_at − server_time`, dihitung mundur dengan jam monotonic perangkat, **bukan** membandingkan dengan jam HP.
6. Token hanya dikembalikan sekali. Jika layar tertutup atau QR kedaluwarsa, panggil endpoint yang sama lagi: QR lama otomatis tidak berlaku, QR baru berlaku 10 menit.
7. Setelah QR terbit, setoran **terkunci** (tidak ada edit/batal). Penolakan barang dilakukan sebelum setoran dibuat.
8. Pantau status lewat `GET /api/v1/deposits/{id}` atau `GET /api/v1/partner/deposits`: `created` → `qr_issued` → `claimed`.

### User memindai dan mengklaim
1. Scan QR → string token.
2. `POST /api/v1/deposit-claims/preview` `{"token": "…"}`: tampilkan mitra, item, berat, poin. Tidak mengubah apa pun; menutup preview tidak membatalkan setoran.
3. `POST /api/v1/deposit-claims` `{"token": "…"}` → 201 dengan `points` dan `wallet_balance`.
4. Jika respons hilang: kirim ulang request yang sama. Pengklaim yang sama mendapat **200** dengan hasil yang sama, tanpa poin ganda.
5. Siapa pun yang berhasil klaim pertama mendapat poin (QR bearer, D-18). Karena itu mitra hanya boleh menunjukkan QR kepada penyetor.
6. Token tidak boleh ditaruh di URL atau log; kirim hanya lewat body.

### Saldo dan riwayat
- `GET /api/v1/me/wallet`: saldo, total didapat, total terpakai.
- `GET /api/v1/me/point-history?type=deposit|redemption&from=&to=`: entri bertanda (`amount` negatif untuk penukaran). Dari entri setoran, buka detail rincian lewat `GET /api/v1/me/deposits/{depositID}`.
- Filter waktu: RFC 3339 dengan zona; `+` pada offset harus di-encode `%2B`. `from` inklusif, `to` eksklusif; hitung batas hari/bulan sesuai zona waktu pengguna.

### Reward dan voucher
1. `GET /api/v1/rewards`: `available=false` berarti stok habis.
2. `POST /api/v1/rewards/{id}/redeem` dengan `Idempotency-Key` baru per tombol "Tukar"; ulangi key yang sama saat retry. Kegagalan **tidak memotong poin**.
3. Voucher berlaku **7 hari**; kedaluwarsa dan terpakai tidak dikembalikan poinnya (tanpa refund, D-20). Status `expired` dihitung server saat dibaca.
4. `GET /api/v1/me/vouchers?status=issued|used|expired` dan `POST /me/vouchers/{id}/use` untuk "tandai sudah dipakai".
5. Voucher bersifat simulasi; tidak ada pencairan nyata.

### Misi, badge, dan leaderboard
Kontrak: [gamification.md](gamification.md). Pemilik melaporkan migration 00010 Applied dan smoke Firebase/Supabase 57 PASS; belum diuji dari Android.
1. `GET /api/v1/me/missions`: progres minggu ini (Senin 00.00 WIB, akhir eksklusif). Tampilkan `period_end` sebagai tanggal berakhir dan hitung sisa waktu dari `server_time`.
2. Saat `status = "claimable"`, kirim `POST /api/v1/me/missions/{missionID}/claim` dengan JSON `{"period_start":"…"}` dari response GET. 201 = klaim pertama; 200 = retry periode sama; periode lama/masa depan mendapat 410 `mission_expired`. Retry selalu memakai token periode yang sama.
3. `GET /api/v1/me/badges`: badge permanen, otomatis, tanpa klaim.
4. `GET /api/v1/leaderboard?limit=20`: XP mingguan; tampilkan `me` bila tidak `null`. `PATCH /api/v1/me/leaderboard-visibility` mengatur keikutsertaan (visible true/false). XP bukan EcoPoints dan tidak menambah saldo.

### Artikel
`GET /api/v1/articles?q=&category=&featured=true` dan detail. Isi berupa **teks biasa** (bukan HTML/Markdown): tampilkan sebagai teks. "Selesai membaca" belum dicatat server.

### Statistik, laporan, dan bantuan
- `GET /me/activity-report` membaca statistik setoran terklaim milik sendiri, default bulan WIB berjalan. Kirim `from` dan `to` bersama untuk rentang lain (maksimal 366 hari). Unduh PDF dari `/me/activity-report.pdf`; jangan mem-parsing respons sukses sebagai JSON. Ini rekap aktivitas, bukan bukti keuangan resmi.
- FAQ: `GET /help/faqs` (konten demo). Laporan masalah/masukan: `POST /me/support-tickets` dengan `Idempotency-Key`; balasan manual terlihat melalui daftar/detail tiket. Tidak ada email, lampiran, SLA, atau refund.
- Inbox: `GET /me/notifications` dengan cursor dan `unread=true` opsional; tandai satu item melalui `PATCH /me/notifications/{id}/read`. Arahkan `target` ke detail setoran/voucher atau daftar misi sesuai role. Tidak ada push maupun backfill transaksi lama.

## 6. Katalog error

Percabangan memakai `code`. Status yang sama dapat membawa code berbeda.

| Status | Code | Arti | Tindakan Android |
|---|---|---|---|
| 400 | `invalid_request` | Body bukan satu objek JSON valid, atau ada field asing | Perbaiki request (bug klien) |
| 400 | `invalid_query` / `invalid_partner_query` | Query tidak valid atau asing | Perbaiki request |
| 400 | `invalid_account` / `invalid_profile` | Isi profil tidak valid | Tampilkan `message` di form |
| 400 | `invalid_estimate` | Item/berat tidak valid (kosong, ganda, format, di luar batas) | Perbaiki input |
| 400 | `waste_not_accepted` | Lokasi tidak menerima material itu | Tampilkan pesan |
| 400 | `zero_points` | Total poin 0 (berat terlalu kecil) | Minta tambah berat |
| 400 | `invalid_idempotency_key` | Key hilang atau format salah | Perbaiki request |
| 400 | `invalid_ticket` | Kind/subjek/pesan tiket tidak valid | Perbaiki input |
| 401 | `unauthorized` | Token hilang/tidak valid/kedaluwarsa/dicabut | Segarkan token sekali, lalu ke login |
| 403 | `account_forbidden` | Akun suspended/deleted | Tampilkan pesan, keluar |
| 403 | `deposit_forbidden` | Bukan mitra, belum punya lokasi, atau lokasi belum verified | Tampilkan status verifikasi |
| 403 | `claim_forbidden` | Mitra mencoba klaim | Tampilkan pesan |
| 403 | `redeem_forbidden` | Mitra mencoba menukar reward | Tampilkan pesan |
| 404 | `not_found` | Tidak ada route aplikasi yang cocok | Periksa base URL/path; bukan bukti profil belum dibuat |
| 405 | `method_not_allowed` | Method tidak cocok dengan route | Perbaiki method sesuai header `Allow` |
| 404 | `account_not_found` | Profil belum dibuat | Panggil `POST /accounts` |
| 404 | `partner_not_found` / `waste_category_not_found` | ID tidak ada atau tersembunyi | Segarkan daftar |
| 404 | `deposit_not_found` | Setoran tidak ada, bukan milik caller, atau belum diklaim (untuk detail user) | Kembali ke daftar |
| 404 | `qr_invalid` | Token QR format salah atau tidak dikenal | "QR tidak dikenal" |
| 404 | `reward_not_found` / `voucher_not_found` / `article_not_found` | Tidak ada atau bukan milik caller | Segarkan daftar |
| 404 | `ticket_not_found` / `notification_not_found` | Tidak ada, ID salah, atau bukan milik caller | Kembali ke daftar |
| 409 | `account_conflict` | Pendaftaran berbeda dari profil yang sudah ada | Tampilkan profil yang ada |
| 409 | `idempotency_conflict` | Key dipakai untuk isi/reward lain | Buat key baru |
| 409 | `point_rate_unavailable` | Tarif aktif belum tersedia | Tampilkan pesan, coba lagi nanti |
| 409 | `deposit_already_claimed` | Setoran sudah diklaim orang lain | "QR sudah dipakai" |
| 409 | `insufficient_points` | Saldo kurang | Tampilkan selisih |
| 409 | `out_of_stock` | Stok reward habis | Segarkan katalog |
| 410 | `qr_expired` | QR lewat 10 menit | Minta mitra menerbitkan ulang |
| 410 | `qr_revoked` | QR sudah diganti QR baru | Minta mitra menunjukkan QR terbaru |
| 410 | `voucher_expired` | Voucher lewat 7 hari | Tampilkan sebagai kedaluwarsa |
| 413 | `body_too_large` | Melebihi batas body (16 KiB; khusus tiket 64 KiB) | Bug klien |
| 415 | `unsupported_media_type` | Content-Type bukan JSON | Bug klien |
| 429 | `ticket_limit_reached` | Sudah ada 20 tiket belum selesai | Tunggu tindak lanjut; retry tiket lama tetap memakai key yang sama |
| 500 | `internal_error` | Kesalahan server | Tampilkan pesan umum; boleh coba lagi |
| 504 | `request_timeout` | Deadline request/auth (15 detik) atau operasi (laporan 10 detik; umumnya 5 detik) | **Ulangi dengan Idempotency-Key yang sama** pada setoran, penukaran dan tiket; untuk klaim, kirim ulang token yang sama. Jangan menganggap timeout berarti transaksi belum commit. |

Aturan retry: 5xx/504/jaringan putus → ulangi dengan **key/token yang sama** (aman). 4xx → jangan diulang tanpa mengubah request.

## 7. Belum tersedia

Backend Go belum menyediakan endpoint scan/foto sampah; implementasi AI dan kontraknya tersedia sebagai [service FastAPI terpisah](ai-scan.md), belum terhubung ke autentikasi/riwayat Go. Game dan misi game (ditunda, D-24), push notification, upload foto profil, banding/refund, penghapusan akun, dan persetujuan mitra dari aplikasi (CLI pemilik, D-21) juga belum tersedia. Kontrak statistik/PDF, tiket bantuan dan inbox tersedia di atas; runtime Supabase/Android belum dibuktikan untuk fitur baru. Data tarif, reward, artikel, dan FAQ saat ini **data demo**.

## 8. Menjalankan dan menguji backend

### Konfigurasi lokal tanpa set environment manual

Jalankan `go run ./cmd/app` dari **root EcoSnap-BE**. Startup memuat `.env` di working directory sebelum membuka PostgreSQL dan Firebase; tidak mencari file di parent dan tidak memuat `.env.example`. File `.env` lokal diabaikan Git. Isi dengan nilai lokal sendiri:

```dotenv
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL="postgresql://postgres.PROJECT_REF:PASSWORD_URL_ENCODED@HOST_POOLER:5432/postgres?sslmode=require"
GOOGLE_APPLICATION_CREDENTIALS="C:/lokasi-di-luar-repo/firebase-admin.json"
GOOSE_DRIVER=postgres
GOOSE_DBSTRING=${DATABASE_URL}
GOOSE_MIGRATION_DIR=database/migrations
```

Ganti placeholder URL/path dengan nilai asli; jangan commit file atau service-account JSON. Gunakan `/` pada path Windows yang dikutip dan URL-encode karakter khusus password database. `HTTP_ADDR` opsional (default loopback); untuk HP fisik melalui LAN gunakan `0.0.0.0:8080`, firewall yang sesuai dan IP laptop di Android.

Environment proses yang sudah ada **selalu menang**, termasuk nilai kosong; pakai terminal baru jika masih ada `$env:DATABASE_URL` lama. `.env` boleh tidak ada untuk deployment yang sudah memasok environment. File yang ada tetapi tidak bisa dibaca/di-parse menghentikan startup dengan pesan generik tanpa isi credential. Loader aplikasi menerima UTF-8 dengan/tanpa BOM dan interpolasi `${DATABASE_URL}`; simpan **UTF-8 tanpa BOM** agar kompatibel dengan Goose. Restart server setelah mengubah `.env`; tidak ada hot reload.

Goose membaca `.env` sendiri: `goose -env .env status` (hanya cek status), lalu `goose -env .env up` hanya jika migration masih pending dan database development sudah dipastikan benar. Goose tidak mengekspor variabel ke terminal induk. CLI seed/verifikasi mitra/tiket dan script smoke **tidak** mendapatkan loader baru; mereka tetap mengikuti konfigurasi masing-masing. Groq/Qwen tetap service Python terpisah, bukan variabel wajib Go.

Kontrak di atas bisa dicoba tanpa Android: ikuti [Runbook Supabase](../supabase-migration-runbook.md) lalu jalankan `./scripts/smoke-test.ps1 -ApiKey <Firebase Web API Key>`. Akun uji: `mitra.demo@ecosnap.test` dan `user.demo@ecosnap.test` (password dipegang pemilik repo).

Mode smoke tambahan:

```powershell
# Settings dua peran saja; preferensi dikembalikan ke nilai awal.
./scripts/smoke-test.ps1 -ApiKey <FirebaseWebApiKey> -SettingsOnly
# Home, laporan JSON/PDF, inbox, FAQ dan bantuan tanpa transaksi finansial.
./scripts/smoke-test.ps1 -ApiKey <FirebaseWebApiKey> -FlowsOnly
# Flow penuh: dua setoran, satu reward, misi; tambah satu tiket hanya bila opt-in.
./scripts/smoke-test.ps1 -ApiKey <FirebaseWebApiKey> -CreateSupportTicket
# Harness offline validator; tidak menghubungi Firebase/database.
./scripts/test-smoke-flows.ps1
```

`-FlowsOnly` tetap menjalankan pemeriksaan PATCH settings yang dipulihkan; bukan mode tanpa write HTTP sama sekali. Full smoke memerlukan data demo dan akun khusus tanpa runner bersamaan. Tiket opt-in memakai satu slot dari batas 20 tiket belum selesai dan tidak dapat dihapus user. Preflight menolak route/migration yang belum siap sebelum setoran, penukaran, atau pembuatan tiket.

Tes otomatis backend: `./scripts/test-postgres-portable.ps1 -PostgresBin <portable-pg-bin>` atau `go test -p 1 -mod=readonly -count=1 -v ./...` dengan `ECOSNAP_TEST_ADMIN_URL` menuju PostgreSQL loopback disposable yang sesuai guard fixture (bukan DATABASE_URL development). Hasil terakhir mencakup laporan, bantuan, inbox dan settings; lihat bukti di atas. Riwayat keputusan dan status per task ada di [BUILD_PROCESS.md](../../BUILD_PROCESS.md).

### Gate CI kontrak dan smoke offline

Workflow `ci` berjalan otomatis pada push ke `main`/`dev` dan pull request menuju kedua branch itu. Job Go/PostgreSQL dipertahankan, bersama gate berikut:

- `api-contracts` (Ubuntu, Python 3.12, Node 22): validasi OpenAPI, kesesuaian route Go/OpenAPI/Postman, schema JSON koleksi, environment ekspor tanpa secret, dan sintaks script Postman; termasuk fixture negatif agar gate terbukti menolak kontrak rusak.
- `smoke-offline` (Windows PowerShell 5.1): parse ketiga script smoke lalu jalankan harness validator flow tanpa HTTP, Firebase atau database.
- `newman-full-flow` (Ubuntu, Node 22, Go dan PostgreSQL 17 sementara): menjalankan koleksi Postman melalui server HTTP aplikasi asli dengan verifier Firebase khusus tes. Registrasi memakai akun baru; dua putaran transaksi menyediakan data pagination. Assertion, error script/jaringan, atau request yang di-skip membuat job gagal.

Untuk menjalankan gate kontrak secara lokal dari virtualenv Python 3.12 aktif, dengan Node 22+ di PATH:

```text
python -m pip install -r scripts/requirements-api-contracts.txt
python scripts/check-api-contracts.py
node scripts/check-postman-scripts.cjs
python scripts/test-api-contracts.py
node --test scripts/test-postman-scripts.cjs
```

Kedua CLI validator menerima `--root <direktori-repo>`; default memakai lokasi script, bukan working directory terminal. Schema resmi Postman disimpan di repo sehingga validasi tidak perlu mengambil schema dari internet. Dependency Python hanya untuk tooling, bukan aplikasi Go.

Gate kontrak/offline tidak menggantikan flow server; job `newman-full-flow` menguji HTTP, service dan SQL melalui koleksi asli, tetapi belum memvalidasi semua field respons terhadap OpenAPI secara otomatis. Inventaris route statis membaca registrasi literal `Handle`/`HandleFunc` di `router.go` dan `*_routes.go`; route dinamis tidak diinventarisasi. Firebase/Supabase nyata dan Android tetap diuji terpisah. CI tidak deploy aplikasi atau menerapkan migration ke database development. Pengaturan branch protection GitHub tidak diubah.

Bukti perintah lokal dan batas gate: [CI integrasi 23 November 2026](../test-results/ci-integration-checks-2026-11-23.md).

Untuk full flow lokal, siapkan PostgreSQL **disposable pada loopback** melalui `ECOSNAP_TEST_ADMIN_URL` sesuai guard fixture (bukan `DATABASE_URL` development), lalu:

```text
npm ci --prefix scripts/newman --ignore-scripts
node --test scripts/newman/test-runner.cjs
go test -p 1 -mod=readonly -tags newman -count=1 -v ./internal/repository/postgres -run "^TestNewmanFullFlow$"
```

`go test ./...` biasa tidak memerlukan Newman; test khusus ini hanya masuk lewat build tag `newman` dan gagal jika prasyaratnya tidak ada. Database dan listener HTTP dibersihkan sesudah test, termasuk saat flow gagal. Token sintetis hanya dikenali verifier dalam file `_test.go`, bukan konfigurasi server produksi. Jangan mengisi runner CI dengan credential asli.

Bukti full flow dan jalur gagal: [Newman CI 24 November 2026](../test-results/newman-full-flow-2026-11-24.md) — 198 HTTP/677 assertion lokal, seluruh 46 operasi sukses, tujuh tes runner PASS.

## 9. Mengambil progress tim

Gunakan `dev` untuk mengambil seluruh progress backend, bukan `main` yang masih baseline:

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
```

Simpan perubahan lokal sendiri sebelum berpindah branch; jangan melakukan reset paksa. Untuk meninjau per fitur, gunakan `feat/account-settings`, `feat/activity-reports`, `feat/in-app-notifications`, dan `feat/support-tickets`; hardening berikutnya dibagi menjadi `feat/server-lifecycle`, `feat/database-pool`, dan `feat/http-transport`. Branch fitur bersifat stacked/berurutan, bukan perubahan yang harus digabung semua satu per satu. Branch historis serta `feat/gamification` juga dipertahankan untuk jejak progress. Gunakan `dev` untuk hasil integrasi.

Publikasi berarti progress kode dapat dibagikan, bukan semua integrasi selesai. Laporan lokal 90 tes top-level + 134 subtest PASS tidak menggantikan smoke Firebase/Supabase/Android fitur baru. Koordinasikan migration 00011–00012 ke database development dengan pemilik memakai runbook; jangan menjalankan SQL atau seed tanpa memastikan database yang dituju. Credential database dan Firebase tetap disediakan di environment masing-masing, tidak dikirim bersama repo.

## 10. Operasi server dan batas koneksi

Server membuka listener sebelum mencetak alamat listening, sehingga port bentrok tidak dilaporkan sebagai startup sukses. Shutdown memberi request aktif hingga 10 detik untuk selesai; jika batas terlampaui, koneksi HTTP ditutup paksa. `Pool.Close()` tetap menunggu koneksi yang dipinjam: batas 10 detik bukan jaminan batas total proses.

Default pool per proses adalah maksimum 4 koneksi, minimum 0, lifetime 30 menit, idle 5 menit, health check 1 menit, connect timeout 10 detik, dan startup/ping dibatasi 10 detik. Angka ini anggaran awal demo, bukan kuota Supabase atau jaminan kapasitas. Setiap laptop/CLI/proses memakai pool sendiri; hitung total proses sebelum menaikkan batas.

Konfigurasi eksplisit memakai opsi native pgx pada `DATABASE_URL`, misalnya `&pool_max_conns=4&pool_min_conns=0&pool_max_conn_lifetime=30m&pool_max_conn_idle_time=5m&pool_health_check_period=1m&connect_timeout=10` jika URL sudah mempunyai `?sslmode=require`. Health check harus positif; lifetime/idle tidak boleh negatif. Nilai lifetime/idle nol diteruskan sesuai perilaku pgx, bukan dijanjikan sebagai “unlimited”. `connect_timeout` positif mengganti default; nol/tidak diisi memakai 10 detik, dan startup tetap dibatasi context 10 detik. Jangan menyalin URL asli ke chat/log/Git. Server memuat `.env` lokal otomatis; `.env.example` tetap tidak pernah dimuat.

Header timeout 5 detik, read timeout 15 detik, write timeout 30 detik, idle timeout 60 detik, batas header 16 KiB. Request ID/logging/panic recovery tidak menambahkan buffering PDF atau mengubah saldo. Panic sebelum respons dimulai menghasilkan 500 generik; setelah respons dimulai koneksi dibatalkan, bukan menambahkan JSON ke file unduhan.

Batas wire HTTP (header terlalu besar, request rusak, timeout jaringan) ditangani net/http dan tidak dijanjikan mempunyai envelope JSON atau request ID. Batas operasional ini belum membuktikan hosting publik, Firebase nyata, maupun performa Supabase.

Bukti tambahan 2026-11-23: [sinyal OS Windows pada cmd/app](../test-results/process-signals-2026-11-23.md) membuktikan idle shutdown dan drain request aktif lewat Ctrl+C/Ctrl+Break (SIGTERM Linux belum diuji). [Beban tulis reward lokal](../test-results/write-load-2026-11-23.md) mencatat 248 request, termasuk 192 redeem dengan retry/balapan stok, tanpa debit ganda atau selisih wallet-ledger. Keduanya memakai auth sintetis dan PostgreSQL disposable; tidak menggantikan Firebase/Supabase/Android nyata atau menjanjikan kapasitas publik.

Relasi, constraint, retensi aktual dan batas data sensitif: [model data runtime](../data-model.md). SQL migration tetap acuan; `database/schema.dbml` adalah arsip desain lama, bukan generator runtime.
