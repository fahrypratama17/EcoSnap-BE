# API EcoSnap untuk tim Android

Mulai dari sini. Dokumen ini adalah indeks, konvensi umum, ringkasan alur per layar, dan katalog error. Detail kolom per endpoint ada di dokumen fitur (bagian 4); urutan panggilan, retry, refresh, dan perubahan yang diperlukan di Android ada di [android-integration.md](android-integration.md).

**Status:** 46 route, tanpa endpoint baru pada paket Home. Verifikasi paket Home: **83 tes top-level + 140 subtes PASS, 0 FAIL/SKIP** pada tiga package terdampak; vet package tersebut dan build seluruh package lulus ([bukti](../test-results/home-package-2026-11-23.md)). HTTP memakai server asli dan PostgreSQL disposable dengan pool satu koneksi, auth sintetis.

Checkpoint full suite sebelum paket Home: 90 tes top-level + 134 subtes PASS ([bukti fondasi](../test-results/server-foundation-2026-11-22.md)). Pemilik sebelumnya melaporkan migration 00001–00010 Applied dan smoke Firebase/Supabase **57 PASS** ([bukti](../test-results/gamification-2026-11-21.md)); smoke itu mendahului settings, laporan/PDF, bantuan, inbox dan Home baru. Fitur-fitur baru belum dibuktikan dengan Firebase/Supabase nyata; migration 00011–00012 dan integrasi runtime Android masih terbuka. Jika respons berbeda, laporkan X-Request-ID tanpa token/credential.

> **Prasyarat database:** Home baru membaca `unread_count` dari tabel notifikasi (migration 00011), jadi **terapkan migration 00011 sebelum memakai `GET /me/home` versi baru**; tabel tidak ada → request gagal, tidak ada fallback `0`. Seluruh flow, termasuk bantuan/tiket, memerlukan 00011 **dan** 00012. Tidak ada migration baru di paket Home. Ini rujukan tunggal; dokumen lain menautkan ke sini.

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
| Waktu | RFC 3339 UTC, mis. `2026-11-21T08:00:00Z`. Format tanggal tampilan dibuat Android. |
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

Tidak ada endpoint untuk scan/foto sampah (menunggu model AI, Q-06), game dan misi game (ditunda, D-24), push notification, upload foto profil, banding/refund, penghapusan akun, dan persetujuan mitra dari aplikasi (CLI pemilik, D-21). Kontrak statistik/PDF, tiket bantuan dan inbox tersedia di atas; runtime Supabase/Android belum dibuktikan untuk fitur baru. Data tarif, reward, artikel, dan FAQ saat ini **data demo**.

## 8. Menjalankan dan menguji backend

Kontrak di atas bisa dicoba tanpa Android: ikuti [Runbook Supabase](../supabase-migration-runbook.md) lalu jalankan `./scripts/smoke-test.ps1 -ApiKey <Firebase Web API Key>`. Akun uji: `mitra.demo@ecosnap.test` dan `user.demo@ecosnap.test` (password dipegang pemilik repo).

Tes otomatis backend: `./scripts/test-postgres-portable.ps1 -PostgresBin <portable-pg-bin>` atau `go test -p 1 -mod=readonly -count=1 -v ./...` dengan `ECOSNAP_TEST_ADMIN_URL` menuju PostgreSQL loopback disposable yang sesuai guard fixture (bukan DATABASE_URL development). Hasil terakhir mencakup laporan, bantuan, inbox dan settings; lihat bukti di atas. Riwayat keputusan dan status per task ada di [BUILD_PROCESS.md](../../BUILD_PROCESS.md).

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

Konfigurasi eksplisit memakai opsi native pgx pada `DATABASE_URL`, misalnya `&pool_max_conns=4&pool_min_conns=0&pool_max_conn_lifetime=30m&pool_max_conn_idle_time=5m&pool_health_check_period=1m&connect_timeout=10` jika URL sudah mempunyai `?sslmode=require`. Health check harus positif; lifetime/idle tidak boleh negatif. Nilai lifetime/idle nol diteruskan sesuai perilaku pgx, bukan dijanjikan sebagai “unlimited”. `connect_timeout` positif mengganti default; nol/tidak diisi memakai 10 detik, dan startup tetap dibatasi context 10 detik. Jangan menyalin URL asli ke chat/log/Git. Credential tetap di environment dan file `.env.example` tidak otomatis dimuat aplikasi.

Header timeout 5 detik, read timeout 15 detik, write timeout 30 detik, idle timeout 60 detik, batas header 16 KiB. Request ID/logging/panic recovery tidak menambahkan buffering PDF atau mengubah saldo. Panic sebelum respons dimulai menghasilkan 500 generik; setelah respons dimulai koneksi dibatalkan, bukan menambahkan JSON ke file unduhan.

Batas wire HTTP (header terlalu besar, request rusak, timeout jaringan) ditangani net/http dan tidak dijanjikan mempunyai envelope JSON atau request ID. Batas operasional ini belum membuktikan hosting publik, Firebase nyata, maupun performa Supabase.

Bukti tambahan 2026-11-23: [sinyal OS Windows pada cmd/app](../test-results/process-signals-2026-11-23.md) membuktikan idle shutdown dan drain request aktif lewat Ctrl+C/Ctrl+Break (SIGTERM Linux belum diuji). [Beban tulis reward lokal](../test-results/write-load-2026-11-23.md) mencatat 248 request, termasuk 192 redeem dengan retry/balapan stok, tanpa debit ganda atau selisih wallet-ledger. Keduanya memakai auth sintetis dan PostgreSQL disposable; tidak menggantikan Firebase/Supabase/Android nyata atau menjanjikan kapasitas publik.

Relasi, constraint, retensi aktual dan batas data sensitif: [model data runtime](../data-model.md). SQL migration tetap acuan; `database/schema.dbml` adalah arsip desain lama, bukan generator runtime.
