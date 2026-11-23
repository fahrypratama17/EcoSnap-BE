# Panduan integrasi Android EcoSnap

Panduan alur untuk tim Android: urutan panggilan, retry, refresh, dan penanganan error per layar. Dokumen ini tidak mengulang detail kolom wire. Indeks, konvensi umum, dan katalog error ada di [README.md](README.md); kolom per endpoint ada di dokumen fitur (rujukan di tiap bagian). Pemetaan beranda per field ada di [wallet-history.md](wallet-history.md#get-apiv1mehome).

## Status

- Semua flow di bawah **sudah tersedia di backend** (kode + tes lokal). **Android belum terhubung ke satu pun**: `EcoSnap-FE` belum memiliki dependency Firebase Auth, klien API, atau DTO; OkHttp hanya dipakai peta. Panduan ini ditulis dari source FE yang dibaca (Routes, navigation, model, `build.gradle.kts`), tanpa mengubah FE.
- Migration 00011–00012 belum diterapkan ke Supabase development, dan fitur baru (settings, laporan/PDF, bantuan, inbox, paket beranda baru) belum diuji dengan Firebase/Supabase nyata. Smoke 57 PASS pemilik sebelumnya mendahului fitur-fitur itu. Hasil verifikasi paket beranda dicatat terpisah, bukan di sini.
- Data tarif, reward, artikel, FAQ, dan mitra seed adalah **demo** (`is_demo`/`uses_demo_rates`); tampilkan labelnya.

## 1. Aturan lintas flow

**Token.** Sebelum tiap request: `getIdToken(false)`. 401 → `getIdToken(true)` sekali lalu ulangi; masih 401 → layar login. UID tidak pernah dikirim klien. Klaim QR dan penukaran reward mengecek pencabutan token di server (sedikit lebih lambat).

**Yang tidak boleh masuk log atau penyimpanan aplikasi sendiri:** password, ID token, refresh token, token QR, kode voucher. Persistensi sesi/token yang dikelola SDK Firebase sendiri tidak dilarang; aplikasi hanya tidak menyalinnya ke penyimpanan/log miliknya. Log cukup memuat path, status, `error.code`, dan `X-Request-ID`.

**Retry dan Idempotency-Key.** Tiga POST memakai `Idempotency-Key` (8–128 karakter `A-Za-z0-9_-`): `POST /deposits`, `POST /rewards/{id}/redeem`, `POST /me/support-tickets`.

| Aturan | Isi |
|---|---|
| Satu key = satu niat pengguna | Buat (UUID) saat pengguna **memulai** niat baru ("Buat setoran"/"Tukar"/"Kirim"), bukan per request HTTP dan bukan per ketukan. Nonaktifkan tombol selama request berjalan; ketukan ulang/double tap tidak boleh membuat key baru |
| Simpan **sebelum** request dikirim | Di penyimpanan persisten per UID (mis. DataStore/Room) beserta niatnya (isi item/reward ID/isi tiket). Process death atau 504 tidak boleh menghilangkan key |
| Ulangi dengan key + isi yang **sama** | Untuk jaringan putus, 5xx, 504, dan app dibuka ulang. Hasil 200 = objek yang sama, bukan duplikat |
| Hapus key | Setelah 2xx diterima dan hasil ditampilkan, atau 4xx final yang menandakan operasi tidak terjadi (400/403/404/409). Jangan menghapus pada jaringan putus, 5xx, atau 504: hasilnya belum diketahui |
| Key baru | Hanya bila pengguna **sengaja memulai niat baru atau mengubah isinya**. Isi berbeda dengan key lama → 409 `idempotency_conflict` |
| 429 `ticket_limit_reached` | Final; tampilkan pesan. Retry tiket yang sudah terbuat tetap lolos |
| Hasil selesai | Simpan `id` hasil (setoran/voucher/tiket) untuk niat yang sudah selesai sampai layar ditutup, sehingga ketukan atau kembali ke layar menampilkan hasil yang sama, bukan niat baru yang tidak disengaja |

Dua operasi tanpa key punya retry sendiri: **klaim QR** (kirim ulang token yang sama; pengklaim yang sama → 200 hasil sama) dan **klaim misi** (kirim ulang `period_start` yang sama).

**Refresh setelah mutasi.** Tidak ada push. Setelah perubahan, muat ulang data turunan dari server, jangan menghitung sendiri.

| Mutasi sukses | Muat ulang |
|---|---|
| Klaim QR | `/me/home`, `/me/wallet`, `/me/point-history` (halaman pertama), `/me/missions`, `/me/badges`, `/me/notifications` |
| Penukaran reward | `/me/home`, `/me/wallet`, `/me/vouchers`, `/rewards` (stok), `/me/point-history` |
| Pakai voucher | `/me/vouchers`, `/me/home` (`active_vouchers`) |
| Klaim misi | `/me/missions` (atau hasil klaim), `/me/home`, `/leaderboard`, `/me/notifications` |
| Favorit PUT/DELETE | Tidak ada; perbarui himpunan lokal (rollback bila gagal) |
| Edit profil / settings | Ganti state dengan respons PATCH |
| Mitra: buat setoran, terbitkan QR | `/partner/home`, `/partner/deposits` |
| Tandai notifikasi dibaca | Gunakan respons PATCH; muat ulang `unread_count` (`/me/notifications` atau `/me/home`) |

**State layar.** Tiap layar punya empat state yang diputuskan FE: loading, kosong (`[]`/`null`, bukan error), error dapat diulang (jaringan/5xx/504), dan error final (4xx; tampilkan `message`). Jangan menampilkan data demo sebagai fallback jaringan. Daftar pagination: kosong bila halaman pertama `[]`; cursor/offset sesuai [README §3](README.md).

## 2. Flow per layar

### 2.1 Sesi, pemulihan profil, registrasi

Fitur: [accounts.md](accounts.md). FE: `AuthNavGraph`, `RegisterScreen`, `LoginScreen`, `PasswordResetScreens`.

1. **Login:** Firebase `signInWithEmailAndPassword`. Berhasil → langsung `GET /api/v1/me` (bukan `/auth/me`, yang hanya identitas token).
2. **Registrasi:** Firebase `createUserWithEmailAndPassword` → `POST /api/v1/accounts` dengan `role` (`AuthRole.USER`→`user`, `PARTNER`→`mitra`), `full_name`, `phone`, dan untuk mitra `business_name`, `business_category`. Password dan email tidak dikirim ke backend.
3. **Pemulihan:** setiap start aplikasi dengan sesi Firebase: `GET /me`. 
   - 200 → pilih rute dari `role` (2.2).
   - 404 `account_not_found` → Firebase ada, profil belum: tampilkan lanjutan registrasi dan `POST /accounts`. Data form (kecuali password) disimpan lokal sampai 201/200 agar bisa diulang setelah app dimatikan.
   - 403 `account_forbidden` → pesan, `signOut`.
   - 401 → alur token (§1).
4. `POST /accounts`: 201 baru, 200 ulangan identik (aman diulang saat timeout). 409 `account_conflict` = profil sudah ada dengan isi berbeda → `GET /me` dan tampilkan profil yang tersimpan; jangan memakai POST untuk sinkronisasi tiap login. 400 `invalid_account` → pesan di form.
5. Reset password sepenuhnya Firebase; backend tidak terlibat.

Perubahan FE: login/registrasi memanggil Firebase (callback sekarang mengabaikan kredensial); tambah dependency Firebase Auth; perbaiki `isValidPhone` (regex berakhir `$}`) dan pemanggilan `isValidPhone(email)` di `RegisterScreen`; backend menerima 9–15 digit dengan `+` opsional.

### 2.2 Role dan mitra pending

- `GET /me` → `role`: `user` → graph utama user. `mitra` → UI mitra (belum ada di FE; `features/deposit` kosong).
- Mitra baru: `business.status = "pending"`, belum punya lokasi: tampilkan banner "menunggu verifikasi". `GET /partner/home` dan pembuatan setoran/QR → 403 `deposit_forbidden`. Persetujuan dilakukan pemilik repo lewat CLI (D-21), bukan dari aplikasi; tidak ada polling khusus, cukup muat ulang `/me` saat layar dibuka.
- Mitra dengan lokasi: `GET /partner/home` → `partner_status` (`verified`/`suspended`/`rejected`/`pending`). Bukan `verified`: riwayat tetap terbaca, pembuatan setoran/QR ditolak 403 `deposit_forbidden` (tampilkan status).
- Mitra boleh: favorit, peta, settings, inbox, laporan, bantuan, riwayat setoran mitra. Mitra tidak boleh: klaim QR (`claim_forbidden`), reward (`redeem_forbidden`), dan seluruh misi/badge/leaderboard termasuk membaca (`gamification_forbidden`).

### 2.3 Onboarding

Fitur: settings di [accounts.md](accounts.md). FE: `OnboardingPreferences` (DataStore lokal), `Routes.ONBOARDING`.

- Onboarding tampil sebelum login, jadi flag lokal tetap diperlukan. Server menyimpan `settings.onboarding_completed` (dari `GET /me` atau `GET /me/settings`) tetapi **tidak** menggerbang akses.
- Rekomendasi: setelah sesi dan profil siap, jika server `false` dan lokal selesai → `PATCH /me/settings {"onboarding_completed":true}`; jika server `true`, tandai lokal selesai (perangkat baru tidak mengulang onboarding). Jangan mengirim field lain; kegagalan PATCH tidak memblokir pengguna.
- Aturan produk lanjutan untuk onboarding (Q-12) belum ditetapkan.

### 2.4 Beranda

> Prasyarat: Home baru membutuhkan migration 00011 (`unread_count`); seluruh flow termasuk bantuan membutuhkan 00011 dan 00012. Lihat [Prasyarat database di README](README.md).

Fitur: [wallet-history.md](wallet-history.md). FE: `HomeScreen`, `HomeUiState`, `MainNavGraph` (sekarang `HomeUiState.demo()` dan `LocalArticles`).

- User: **satu** `GET /me/home` untuk kartu poin, setoran bulanan, misi, satu mitra, satu artikel, dan lonceng. Mitra: `GET /partner/home`.
- Jika 200 dengan `partner`/`article`/`missions` null: tampilkan state kosong. 404/403: lihat §2.1/2.2. Jangan menampilkan data demo bila gagal.
- Klik mitra → `GET /partners/{partner.id}` lewat `Routes.partnerDetail(id)`; klik artikel → `GET /articles/{article.id}` lewat `Routes.articleDetail(id)`; lonceng → inbox (§2.11).
- Perbedaan arti yang harus dijaga di UI: EcoPoints (`wallet.balance`) ≠ XP (`reward_xp`); `monthly_deposits` (bulan WIB) ≠ `deposits_claimed` (all-time); mitra pada beranda adalah saran global, bukan "terdekat".

### 2.5 Peta, daftar mitra, favorit

Fitur: [partners.md](partners.md), [partner-favorites.md](partner-favorites.md). FE: `PartnerMapScreen`, `PartnerDetailScreen`, `DummyPartners`, `WastePartner`.

1. `GET /waste-categories` untuk filter material; pilih lewat `slug`. "Semua" = tanpa `material`.
2. `GET /partners?q=&material=&limit=&offset=` (offset; berhenti saat `has_more=false`). `q` mencari nama/area, bukan alamat. Tidak ada radius/GPS: urutan jarak dihitung di Android dari `location.latitude/longitude` bila perlu.
3. Detail: `GET /partners/{id}` untuk jam (`hours[]`, 0=Minggu), material (`accepted_materials[].code` → `WasteMaterial`; kode asing ditangani generik), kontak. Rute memakai UUID server, bukan ID dummy seperti `tunas`.
4. Favorit: `GET /me/favorites` mengembalikan hanya `partner_id`+`created_at` (offset); pegang himpunan ID, data lokasi dari daftar/detail. PUT/DELETE tanpa body, 204, idempotent. Optimistic update; gagal → rollback. 404 `partner_not_found` pada PUT → mitra tidak lagi visible, muat ulang daftar. Favorit milik mitra yang disembunyikan tidak muncul di GET.
5. Error: 400 `invalid_partner_query` (bug klien), 404 `partner_not_found`, 504 → ulangi.

Perubahan FE: ganti `DummyPartners`/`WastePartner` dengan DTO; `Routes.partnerDetail` memakai UUID; mapping kategori.

### 2.6 Kalkulator estimasi

Fitur: [point-estimates.md](point-estimates.md). `POST /point-estimates` `{items:[{category_id, weight_kg:"2.500"}], partner_id?}`. Berat string desimal bertitik, kategori unik (gabungkan beratnya di Android). Hasil hanya estimasi; tampilkan penanda demo. Error: 400 `invalid_request`/`invalid_estimate`/`waste_not_accepted`; 404 `waste_category_not_found`/`partner_not_found`; 409 `point_rate_unavailable`. `zero_points` bukan error estimasi: itu hanya penolakan `POST /deposits` saat total 0.

### 2.7 Transaksi: setoran mitra, QR, klaim user

Fitur: [deposits.md](deposits.md). FE: `ScanQrScreen`, `ScanSuccessScreen`, `LocalDeposits`, `Routes.SCAN_QR`, `SCAN_QR_RESULT`.

**Mitra (membuat dan menerbitkan):**
1. Susun item (kategori unik, berat string). Buat key, simpan persisten bersama isi (§1), `POST /deposits` → 201 (atau 200 ulangan). Simpan `data.id` sebagai ID setoran.
2. `POST /deposits/{id}/qr` (tanpa body) → `token`, `expires_at`, `server_time`, `warning`. Tampilkan `warning` di layar yang sama dengan QR; encode `token` apa adanya.
3. Countdown = `expires_at − server_time` dengan jam monotonic, bukan jam HP. Token **hanya muncul sekali**; simpan di memori layar, jangan persisten/log. Layar tertutup, app mati, atau kedaluwarsa → panggil endpoint yang sama lagi (reissue); QR lama langsung `qr_revoked`.
4. Pulihkan setelah app mati sebelum QR terbit: key tersimpan → ulangi `POST /deposits` (hasil setoran sama), lanjut ke QR. Setelah QR terbit setoran terkunci (tanpa edit/batal).
5. Pantau: `GET /deposits/{id}` (`status` `created`→`qr_issued`→`claimed`) atau `GET /partner/deposits?status=&limit=&cursor=`. Tanpa push: muat ulang saat layar dibuka dan lewat tarik-untuk-menyegarkan. Identitas pengklaim tidak tersedia.
6. Error: 403 `deposit_forbidden` (belum verified → tampilkan status), 409 `deposit_already_claimed` (sudah diklaim, hentikan QR), 404 `deposit_not_found`, 400 `zero_points`/`waste_not_accepted`/`invalid_estimate`, 409 `point_rate_unavailable`.

**User (memindai dan mengklaim):**
1. Pindai → string token. Kirim hanya lewat body.
2. `POST /deposit-claims/preview {"token"}` → mitra, item, berat, poin. Tidak mengubah apa pun; menutup layar tidak membatalkan.
3. `POST /deposit-claims {"token"}` → 201 `deposit_id`, `points`, `wallet_balance`, `claimed_at`. Siapa pun yang mengklaim pertama mendapat poin (QR pembawa, D-18).
4. Respons hilang/timeout: kirim ulang token yang sama (pengklaim sama → 200, tanpa poin ganda). Jika token sudah hilang (app mati): pindai ulang; bila ia sudah diklaim akun ini, hasilnya 200 yang sama. Cara lain memastikan hasil: `GET /me/point-history` atau inbox `deposit_credited`.
5. Layar hasil (`Routes.scanQrResult(...)`): gunakan `deposit_id` (bukan token) sebagai ID; rincian lewat `GET /me/deposits/{deposit_id}`. Hapus token dari memori setelah layar hasil.
6. Error: 404 `qr_invalid`, 410 `qr_expired`/`qr_revoked` (minta mitra menerbitkan ulang/menunjukkan QR terbaru), 409 `deposit_already_claimed`, 403 `claim_forbidden` (mitra), 401 (token dicabut/akun nonaktif).

Perubahan FE: `LocalDeposits` menganggap isi QR sebagai ID transaksi lokal; ganti dengan dua panggilan di atas. Tidak ada UI mitra (setoran/QR) di FE saat ini.

### 2.8 Saldo, riwayat, reward, voucher

Fitur: [wallet-history.md](wallet-history.md), [rewards.md](rewards.md). FE: `Routes.POINTS`, `HISTORY`, `PROFILE_ACTIVITY_HISTORY`, `PROFILE_REWARDS` (sebagian `ProfilePlaceholderScreen`).

- Saldo: `GET /me/wallet`. Riwayat: `GET /me/point-history?type=deposit|redemption&from=&to=&limit=&cursor=`. Cursor buram, tidak disimpan lintas sesi, tidak dipakai ulang bila filter berubah (mulai dari halaman pertama). `+` pada offset waktu di-encode `%2B07:00`, atau pakai `Z`; `from` inklusif, `to` eksklusif; batas hari/bulan dihitung Android menurut zona pengguna. Entri `deposit` → detail `GET /me/deposits/{deposit.id}`; entri `redemption` bernilai negatif.
- Reward: `GET /rewards` (`available=false` = stok habis; urut biaya termurah, maks. 100, tanpa pagination) dan `/rewards/{id}`.
- Tukar: `POST /rewards/{id}/redeem` dengan key baru per penekanan tombol (§1, tersimpan sebelum kirim); 201/200 → `voucher` (+`code`), `wallet_balance`. Kegagalan tidak memotong poin. Error: 409 `insufficient_points` (tampilkan selisih dari saldo), 409 `out_of_stock` (muat ulang katalog), 404 `reward_not_found`, 403 `redeem_forbidden`, 409 `idempotency_conflict` (key baru hanya bila niat berubah).
- Voucher: `GET /me/vouchers?status=issued|used|expired` (cursor), detail, `POST /me/vouchers/{id}/use` (idempotent; 410 `voucher_expired`). Berlaku 7 hari, tanpa refund, tanpa pencairan nyata (simulasi, D-08, D-20). `code` hanya ditampilkan di layar voucher, tidak dicatat di log.

### 2.9 Misi, badge, leaderboard

Fitur: [gamification.md](gamification.md). FE: `Routes.MISSIONS`, `GAMES`, `PROFILE_ACHIEVEMENTS`, `GAME_LEADERBOARD`/`DEPOSIT_LEADERBOARD`, `WeeklyChallengeCard`, `GamesHomeScreen`, `LocalLeaderboard`.

- `GET /me/missions`: katalog misi mingguan (Senin 00.00 WIB, akhir eksklusif). Sisa waktu = `period_end − server_time`. Progres per `unit`: `deposit` (jumlah) dan `gram` (3000 = 3 kg). Hadiah **XP**.
- `status = claimable` → `POST /me/missions/{id}/claim {"period_start":"<nilai persis dari GET>"}`: 201 pertama, 200 ulangan, 410 `mission_expired` (periode berganti → muat ulang), 409 `mission_incomplete`. Simpan `period_start` selama proses klaim; retry memakai nilai yang sama.
- Beranda hanya menampilkan satu misi dari katalog yang sama; pilihan misi adalah keputusan FE.
- `GET /me/badges`: permanen dan otomatis (tanpa klaim). `GET /leaderboard?limit=`: XP mingguan, `me` bisa `null`; peringkat seri berperingkat sama (1,1,3); `photo_path` bisa `null` dan bukan URL penuh. `PATCH /me/leaderboard-visibility {"visible":bool}` (user saja).
- Perubahan FE: ganti teks tetap "3 jenis sampah"/"+50 EcoPoints" dan progres 1/3 dengan `missions[]`; layar misi/badge belum punya tujuan di `GamesNavGraph`; mode leaderboard selain XP mingguan tidak punya endpoint.

### 2.10 Laporan dan PDF

Fitur: [activity-reports.md](activity-reports.md). FE: `Routes.PROFILE_ACTIVITY_REPORT`.

- `GET /me/activity-report` (tanpa query = bulan WIB berjalan; `from` dan `to` selalu berpasangan, RFC 3339 dengan zona, `+`→`%2B`, maks. 366 hari). Hanya setoran terklaim, diukur dari waktu klaim; saldo bukan metrik periode.
- `GET /me/activity-report.pdf`: unduh sebagai berkas dengan header Bearer (tidak bisa lewat tautan tanpa auth). Sukses = `application/pdf`, jangan diurai sebagai JSON; selain 2xx tetap JSON error. Batas waktu 10 detik di server. Simpan ke cache/MediaStore lalu buka dengan viewer; ini rekap aktivitas, bukan bukti keuangan.
- Mitra tanpa lokasi → 403 `deposit_forbidden`; `invalid_query` untuk rentang salah.

### 2.11 Inbox dan notifikasi

Fitur: [notifications.md](notifications.md). FE: `Routes.NOTIFICATION` (lonceng di Home, Games, Leaderboard, Articles).

- `GET /me/notifications?limit=&cursor=&unread=true`; `unread_count` total milik caller. `PATCH /me/notifications/{id}/read` saat item dibuka (idempotent).
- Deep-link menurut `target` dan role: `deposit` → user `GET /me/deposits/{id}`, mitra `GET /deposits/{id}`; `voucher` → `GET /me/vouchers/{id}`; `mission` → layar misi (`GET /me/missions`). Tipe asing: tampilkan tanpa navigasi.
- Tanpa push: muat ulang saat layar dibuka, tarik-untuk-menyegarkan, dan saat beranda mengubah `unread_count`. Notifikasi lama tidak dibuat ulang (tanpa backfill). `notification_enabled=false` menahan notifikasi **baru**, yang sudah ada tetap terlihat.

### 2.12 Bantuan dan tiket

Fitur: [support.md](support.md). FE: `Routes.HELP`, `PROFILE_HELP_CENTER`, `FaqItem`.

- `GET /help/faqs` (demo, plain text). Tiket: `POST /me/support-tickets` `{kind: problem|feedback, subject, message}` dengan key (§1); `GET /me/support-tickets?status=` dan detail untuk melihat balasan.
- Tidak ada kontak publik, email, lampiran, push, atau janji waktu balasan. Balasan manual dibaca dengan membuka ulang daftar/detail; `staff_response=null` → "Belum ada balasan". Error: 400 `invalid_ticket`, 429 `ticket_limit_reached`, 413 bila >64 KiB.

### 2.13 Profil, pengaturan, logout

Fitur: [accounts.md](accounts.md). FE: `ProfileScreen`, `AccountSettingsScreen`, `ProfileNavGraph` (`onLogoutClick` masih TODO), `ProfileDummyData`, `GreenStats`.

- Profil: `GET /me`; edit `PATCH /me` hanya `full_name` dan `phone` (field lain ditolak 400 `invalid_profile`/`invalid_request`). Statistik (`GreenStats`, semua String di FE; formatkan di UI): `ecoPoints` = `wallet.balance`; `totalDeposits` = `deposits_claimed` dari `/me/home` (**all-time**, bukan `monthly_deposits`; label "bulan ini" hanya untuk `monthly_deposits`); `totalBadges` = jumlah elemen `badges[]` dari `GET /me/badges` dengan `earned == true`, **bukan** panjang seluruh katalog. Mitra tidak memiliki statistik ini (`gamification_forbidden`); sembunyikan atau ganti dengan `/partner/home`.
- Settings: `GET/PATCH /me/settings`: backend hanya menyimpan `notification_enabled` dan `onboarding_completed`; `show_on_leaderboard` ditulis lewat `/me/leaderboard-visibility`. Toggle notifikasi lain di UI tidak punya penyimpanan server.
- Logout: `FirebaseAuth.signOut()` dan kosongkan token/state di memori serta cache turunan (profil, saldo, daftar). **Jangan** menghapus niat finansial yang hasilnya belum diketahui (setoran, penukaran, tiket dengan key tersimpan): simpan per UID, sembunyikan dari UI saat logout, dan pertahankan sampai UID yang sama login lagi lalu menyelesaikannya (ulangi key yang sama atau `GET` untuk memeriksa hasil). Tidak pernah dipakai oleh UID lain. Hapus hanya niat yang sudah resolve. Tidak ada endpoint logout di backend.
- Email/password: Firebase. Tidak ada upload foto profil dan penghapusan akun.

### 2.14 Artikel

Fitur: [articles.md](articles.md). FE: `ArticleListScreen`, `ArticleDetailScreen`, `LocalArticles`, `Routes.ARTICLES`, `Routes.articleDetail(id)`; Home hanya membuka satu artikel (§2.4), daftar penuh dari "Lihat Semua".

1. Daftar: `GET /articles?limit=&cursor=&q=&category=&featured=true`. Cursor buram: teruskan `pagination.next_cursor`, berhenti saat `null`; mulai ulang dari halaman pertama bila `q`/`category`/`featured` berubah (cursor tidak dipakai lintas filter). `q` mencari judul/ringkasan (maks. 100 karakter). `category` salah satu `GUIDE`, `SORTING`, `RECYCLING`, `SPECIAL_WASTE`, `NEWS`; kategori baru yang tidak dikenal enum FE ditampilkan generik. `featured=true` untuk kartu unggulan. Hanya artikel terbit (bukan draft/terjadwal), terbaru di atas.
2. Detail: `GET /articles/{id}` memakai `id` dari daftar atau dari Home. Konten (`introduction`, `sections[].title/body`, `tip`) adalah **teks biasa**, bukan HTML/Markdown; tampilkan sebagai teks. `sections` bisa `[]`.
3. Error: 400 `invalid_query` (parameter asing/ganda, kategori/`featured`/cursor salah: bug klien), 404 `article_not_found` (draft, belum terbit, atau ID salah; kembali ke daftar dan muat ulang).
4. "Selesai membaca" hanyalah navigasi lokal Android: server tidak mencatatnya dan tidak memberi poin atau XP. Jangan menampilkan hadiah.
5. State: loading, daftar kosong (`[]`, mis. filter tanpa hasil), error dapat diulang (jaringan/5xx/504). Konten adalah data demo (`is_demo`).

## 3. Ringkasan perubahan FE yang diperlukan

| Area | Kondisi source FE | Kebutuhan |
|---|---|---|
| Dependency/jaringan | Belum ada Firebase Auth/klien API/DTO | Tambah Firebase Auth, klien dengan interceptor token + 401 refresh, DTO `snake_case`, `network_security_config` cleartext untuk host dev. Konfigurasi parser JSON agar **mengabaikan field tak dikenal** (mis. `ignoreUnknownKeys = true` pada kotlinx.serialization, atau setara di library lain) supaya field tambahan seperti pada Home tidak mematahkan DTO lama; field nullable (`partner`, `article`, `missions`, `claimed_at`, `photo_path`) dimodelkan nullable |
| Auth | Callback login/registrasi mengabaikan kredensial; validasi telepon salah | §2.1 |
| Navigasi per role | Hanya graph user | Graph mitra + banner pending (§2.2) |
| Onboarding | Hanya DataStore lokal | §2.3 |
| Beranda | `HomeUiState.demo()`, `LocalArticles`; `NearbyPartnerUi` tanpa ID | Mapping wallet-history.md; tambah ID partner/artikel |
| Peta/favorit | `DummyPartners` | §2.5 |
| Transaksi | `LocalDeposits`; tanpa UI mitra | §2.7 |
| Riwayat/reward/voucher | Placeholder | §2.8 |
| Misi/badge/leaderboard | Teks dan data lokal | §2.9 |
| Laporan, bantuan, inbox | Placeholder/lokal | §2.10–2.12 |
| Artikel | `LocalArticles`; daftar/detail lokal | §2.14 |
| Profil/logout | `ProfileDummyData`, logout TODO | §2.13 |

## 4. Belum tersedia atau ditunda

Scan/foto sampah (menunggu model AI, Q-06), game dan XP/progres game (ditunda, D-24), push notification, upload foto/media, banding dan refund, penghapusan akun, admin/persetujuan mitra dari aplikasi (CLI pemilik, D-21), kontak dukungan publik, hosting demo/produksi (Q-15), HTTPS publik, dan pembuktian Firebase/Supabase/Android nyata untuk fitur baru. Jangan menampilkan fitur ini sebagai berfungsi.

## 5. Arsip: audit frontend 17 November 2026

Dipertahankan sebagai bukti, bukan status terkini. Pada audit itu Android belum memanggil backend: login/registrasi hanya navigasi, beranda memakai data demo, peta/klaim/game/leaderboard/artikel memakai data lokal, dan logout TODO. Sebagian besar kontrak yang saat itu hanya rencana kini sudah ada (lihat README §4); sisa temuan yang masih berlaku ada di §3.

Kontrak dua route awal:

- `GET /health` (publik) → `{"status":"ok"}`; hanya memeriksa proses HTTP.
- `GET /api/v1/auth/me` → `{"data":{"uid":"…","email":"…","email_verified":false}}` dari claim token terverifikasi; tidak membaca PostgreSQL dan bukan dasar otorisasi. 401 `unauthorized` untuk header hilang/salah atau token ditolak. Untuk profil aplikasi gunakan `GET /me`.

Backend memakai HTTP biasa saat development dan default listener 127.0.0.1; alamat emulator/perangkat dan cleartext dijelaskan di [README §1](README.md). Seluruh riwayat keputusan dan status task ada di [BUILD_PROCESS.md](../../BUILD_PROCESS.md).
