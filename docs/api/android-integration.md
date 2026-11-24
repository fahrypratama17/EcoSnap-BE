# Panduan integrasi Android EcoSnap

Panduan alur untuk tim Android: urutan panggilan, retry, refresh, dan penanganan error per layar. Dokumen ini tidak mengulang detail kolom wire. Indeks, konvensi umum, dan katalog error ada di [README.md](README.md); kolom per endpoint ada di dokumen fitur (rujukan di tiap bagian). Pemetaan beranda per field ada di [wallet-history.md](wallet-history.md#get-apiv1mehome).

## Status

- Backend menyediakan 46 operasi Go. FE memakai Firebase Auth dan klien server untuk akun/profil/settings, beranda user/mitra, mitra/favorit, artikel, estimasi/setoran/QR, saldo/riwayat/reward/voucher, misi/badge/XP mingguan, inbox, bantuan/tiket, dan laporan JSON/PDF. Cakupan bukti runtime dicatat terpisah; keberadaan klien bukan pernyataan semua 46 operasi sudah diuji Android/cloud.
- Status migration dan smoke Firebase/Supabase development mengikuti [README API](README.md), bukan catatan historis di dokumen ini. Pengujian Android dengan Firebase Auth Emulator dan PostgreSQL disposable tidak membuktikan koneksi project cloud yang dipilih developer.
- Data tarif, reward, artikel, FAQ, dan mitra seed adalah **demo** (`is_demo`/`uses_demo_rates`); tampilkan labelnya.

### Menjalankan dari Android Studio

- Simpan konfigurasi aplikasi Android untuk `com.example.ecosnap` di `EcoSnap-FE/app/google-services.json`. Project Firebase harus sama dengan verifier Go; provider Email/Password harus aktif. Jangan memakai service-account JSON di Android.
- `API_BASE_URL` di `EcoSnap-FE/local.properties` adalah **origin**. Emulator tanpa reverse memakai `http://10.0.2.2:8080/`; alamat `10.0.2.2` bukan alamat laptop untuk HP fisik. HP USB bisa memakai `http://127.0.0.1:8080/` setelah `adb reverse tcp:8080 tcp:8080` pada perangkat tersebut (ulang setelah tersambung kembali); listener Go tetap loopback. Alternatif HP lewat LAN: IP LAN laptop, `HTTP_ADDR=0.0.0.0:8080`, dan firewall sesuai. Setelah mengubah URL, build/Run ulang APK, bukan sekadar membuka aplikasi lama.
- Tombol Run Android Studio hanya menjalankan FE. Di terminal BE, isi environment server sesuai runbook lalu jalankan `go run ./cmd/app`; password database dan Firebase Admin credentials tetap di server. Tidak ada auto-deploy atau auto-start Go dari Android.
- `/health` hanya liveness HTTP. Login lalu buka beranda/peta untuk menguji token dan PostgreSQL; daftar kosong yang sah tidak diganti mitra/artikel demo. Home membutuhkan migration 00011, seluruh fitur termasuk bantuan membutuhkan 00012.
- Build memakai JVM 25 sesuai konfigurasi daemon Gradle; pilih JBR/JDK 25 di Android Studio. Peta memakai [artifact MapLibre OpenGL](https://maplibre.org/maplibre-native/android/examples/artifacts/), membutuhkan perangkat/AVD dengan [OpenGL ES 3.0](https://maplibre.org/maplibre-native/docs/book/platforms/index.html). Versi API Android saja tidak menjamin kemampuan GPU tersebut.

### Bukti verifikasi lokal terdahulu: fondasi Home/mitra

- Satu skenario beranda/peta kosong dan delapan skenario data terisi berhasil **secara kumulatif** pada Android 24 x86 dengan GLES 3.0, Firebase Auth Emulator, handler Go produksi, dan PostgreSQL disposable bermigration 00001–00012. Tidak memakai akun/data Firebase atau Supabase cloud.
- Cakupan: data Home/saldo/setoran bulanan/unread, nullable mitra/artikel, pencarian/filter/paging/detail mitra, kategori baru Tekstil, favorit lintas sesi dan isolasi akun, kegagalan jaringan tanpa Home palsu, UUID kartu Home → detail mitra/artikel, daftar/cursor/pencarian artikel, klaim/replay misi tanpa perubahan EcoPoints, serta beranda mitra verified dan pending tanpa logout.
- Run awal menemukan kebutuhan renderer: artifact Vulkan gagal pada perangkat tanpa Vulkan; artifact OpenGL kemudian membutuhkan AVD GLES 3.0, bukan GLES 2.0. Peta akhirnya dirender dengan tile OpenStreetMap dan data mitra server. Selector uji UI diperbaiki untuk posisi scroll yang dipertahankan dan pemilihan daftar vertikal, bukan baris filter horizontal; navigasi artikel diuji ulang secara terarah. Tidak mengubah halaman/state untuk membuat test tampak berhasil.
- Probe server development yang sedang berjalan: `/health` 200 dan tujuh GET untuk Home/partner Home/kategori/mitra/favorit/artikel/misi mengembalikan 401 tanpa token, sehingga route terdaftar dan terlindungi. Probe itu bukan bukti query database cloud atau login Firebase nyata dari Android.
- Instrumentation test dan lingkungan fixture bersifat sementara dan dibersihkan setelah smoke; tidak ada `useEmulator` atau data fixture yang ditambahkan ke source aplikasi produksi. Tidak menjalankan ulang seluruh suite backend 46 operasi.

### Paket lanjutan: implementasi dan handoff pengujian manual

- Source integrasi lanjutan berhasil dibuild sebagai APK debug. Smoke Android lokal melaporkan lima skenario lulus: PATCH profil, email reset Firebase, inbox, FAQ/tiket, serta laporan/unduh PDF. Lingkungannya Firebase Auth Emulator + Go produksi + PostgreSQL disposable, bukan Firebase/Supabase cloud pengguna.
- Run awal menemukan parsing QR yang menolak offset RFC 3339 `+07:00` pada Android 24. Parser diganti ke `OffsetDateTime`; hasil setoran selesai juga dipertahankan di penyimpanan sampai pengguna memilih **Buka Setoran**, bukan dihapus sebelum hasil diakui.
- Pengulangan smoke terarah tidak selesai: runner tertahan pada skenario UI mitra pertama dan timeout. Pengujian otomatis kemudian dihentikan atas permintaan pemilik. **QR/setoran, reward/voucher, settings, badge/Games, pemulihan process-death, dan AI tidak dinyatakan lulus end-to-end.** File instrumentation sementara dibersihkan; pengujian lengkap diteruskan pemilik secara manual.
- Masalah registrasi pada HP fisik memperlihatkan pesan BE tidak terhubung ketika FE masih memakai alamat emulator `10.0.2.2`. Konfigurasi lokal disiapkan untuk USB `127.0.0.1:8080`; jalur localhost/reverse terbukti `/health` 200 pada AVD. HP fisik terputus sebelum APK baru dan jalurnya dapat diverifikasi, jadi keberhasilan registrasi cloud belum dibuktikan. Hubungkan kembali, pasang `adb reverse` pada serial yang benar, lalu Run ulang.
- Uji alur dua peran memakai dua perangkat/sesi: mitra terverifikasi membuat dan menampilkan QR, user memindai lalu mengklaim. Mitra baru tetap pending dan perlu persetujuan/pelengkapan lokasi oleh pemilik BE; Android tidak mempunyai tombol persetujuan. Periksa saldo/riwayat/inbox/misi setelah klaim, lalu reward/voucher, profil/settings, bantuan dan laporan. AI membutuhkan service terpisah aktif sebelum hasil model dapat diuji.




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

Status FE: login/registrasi memakai Firebase dan API akun; validator email/telepon sudah diperbaiki. Metadata registrasi tanpa password disimpan per UID di DataStore yang dikecualikan dari backup agar pembuatan profil dapat dipulihkan setelah timeout atau app dimatikan.

### 2.2 Role dan mitra pending

- `GET /me` → `role`: `user` → graph utama user; `mitra` → graph operasional mitra dengan ringkasan, buat/riwayat/detail setoran, terbitkan/terbitkan ulang QR, estimasi, telusur/favorit, inbox, profil/settings, bantuan, dan laporan. Graph mitra tidak mendaftarkan rute game/reward/voucher/badge/klaim QR milik user.
- Mitra baru: `business.status = "pending"`, belum punya lokasi: tampilkan banner "menunggu verifikasi". `GET /partner/home` dan pembuatan setoran/QR → 403 `deposit_forbidden`. Persetujuan dilakukan pemilik repo lewat CLI (D-21), bukan dari aplikasi; tidak ada polling khusus, cukup muat ulang `/me` saat layar dibuka.
- Mitra dengan lokasi: `GET /partner/home` → `partner_status` (`verified`/`suspended`/`rejected`/`pending`). Bukan `verified`: riwayat tetap terbaca, pembuatan setoran/QR ditolak 403 `deposit_forbidden` (tampilkan status).
- Mitra boleh: favorit, peta, settings, inbox, laporan, bantuan, riwayat setoran mitra. Mitra tidak boleh: klaim QR (`claim_forbidden`), reward (`redeem_forbidden`), dan seluruh misi/badge/leaderboard termasuk membaca (`gamification_forbidden`).

### 2.3 Onboarding

Fitur: settings di [accounts.md](accounts.md). FE: `OnboardingPreferences` (DataStore lokal), `Routes.ONBOARDING`.

- Onboarding tampil sebelum login, jadi flag lokal tetap diperlukan. Server menyimpan `settings.onboarding_completed` (dari `GET /me` atau `GET /me/settings`) tetapi **tidak** menggerbang akses.
- FE menyinkronkan flag onboarding secara best-effort setelah sesi terautentikasi dan ketika profil dibuka: flag lokal selesai → server `onboarding_completed=true` bila belum. PATCH hanya mengirim field itu; kegagalan tidak memblokir sesi. Onboarding sebelum login tetap memakai preferensi lokal.
- Aturan produk lanjutan untuk onboarding (Q-12) belum ditetapkan.

### 2.4 Beranda

> Prasyarat: Home baru membutuhkan migration 00011 (`unread_count`); seluruh flow termasuk bantuan membutuhkan 00011 dan 00012. Lihat [Prasyarat database di README](README.md).

Fitur: [wallet-history.md](wallet-history.md). FE: `HomeRepository`, `HomeRoute`, `HomeUiState`, dan `PartnerHomeRoute`; data beranda dimuat dari server, bukan `HomeUiState.demo()`/artikel lokal.

- User: **satu** `GET /me/home` untuk kartu poin, setoran bulanan, misi, satu mitra, satu artikel, dan lonceng. Mitra: `GET /partner/home`.
- Jika 200 dengan `partner`/`article`/`missions` null: tampilkan state kosong. 404/403: lihat §2.1/2.2. Jangan menampilkan data demo bila gagal.
- Klik mitra → `GET /partners/{partner.id}` lewat `Routes.partnerDetail(id)`; klik artikel → `GET /articles/{article.id}` lewat `Routes.articleDetail(id)`; lonceng → inbox (§2.11).
- Perbedaan arti yang harus dijaga di UI: EcoPoints (`wallet.balance`) ≠ XP (`reward_xp`); `monthly_deposits` (bulan WIB) ≠ `deposits_claimed` (all-time); mitra pada beranda adalah saran global, bukan "terdekat".

### 2.5 Peta, daftar mitra, favorit

Fitur: [partners.md](partners.md), [partner-favorites.md](partner-favorites.md). FE: `PartnerRepository`, `PartnerMapRoute`, `PartnerDetailRoute`, serta model bersama `Partner`/`WasteCategory`.

1. `GET /waste-categories` untuk filter material; pilih lewat `slug`. "Semua" = tanpa `material`.
2. `GET /partners?q=&material=&limit=&offset=` (offset; berhenti saat `has_more=false`). `q` mencari nama/area, bukan alamat. Tidak ada radius/GPS: urutan jarak dihitung di Android dari `location.latitude/longitude` bila perlu.
3. Detail: `GET /partners/{id}` untuk jam (`hours[]`, 0=Minggu), material (`accepted_materials[]`), kontak, deskripsi, dan koordinat. Kategori membawa nama/slug/code dari server; kode asing tetap ditampilkan dengan ikon generik. Rute memakai UUID server, bukan ID dummy seperti `tunas`.
4. Favorit: `GET /me/favorites` mengembalikan hanya `partner_id`+`created_at` (offset); pegang himpunan ID, data lokasi dari daftar/detail. PUT/DELETE tanpa body, 204, idempotent. Optimistic update; gagal → rollback. 404 `partner_not_found` pada PUT → mitra tidak lagi visible, muat ulang daftar. Favorit milik mitra yang disembunyikan tidak muncul di GET.
5. Error: 400 `invalid_partner_query` (bug klien), 404 `partner_not_found`, 504 → ulangi.

Status FE: daftar dan detail memakai API, filter memakai kategori server, pencarian diproses server dengan debounce, daftar memakai offset/load-more, dan favorit dimuat/disimpan per akun. Toggle gagal di-rollback; jika favorit awal belum berhasil dimuat, toggle dinonaktifkan dengan pesan/retry.

### 2.6 Kalkulator estimasi

Fitur: [point-estimates.md](point-estimates.md). `POST /point-estimates` `{items:[{category_id, weight_kg:"2.500"}], partner_id?}`. Berat string desimal bertitik, kategori unik (gabungkan beratnya di Android). Hasil hanya estimasi; tampilkan penanda demo. Error: 400 `invalid_request`/`invalid_estimate`/`waste_not_accepted`; 404 `waste_category_not_found`/`partner_not_found`; 409 `point_rate_unavailable`. `zero_points` bukan error estimasi: itu hanya penolakan `POST /deposits` saat total 0.

### 2.7 Transaksi: setoran mitra, QR, klaim user

Fitur: [deposits.md](deposits.md). FE: `DepositRepository`, `ScanQrRoute`, `DepositDetailRoute`, `PartnerDepositCreateRoute`, dan `PartnerDepositsRoute`. Pemindaian kamera/galeri memakai decoder QR; input kode manual tersedia. Decode hanya membuka preview, bukan otomatis mengklaim.

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

Status FE: `LocalDeposits` dan layar hasil lokal dihapus. Preview/klaim memakai server dan hasil dirutekan dengan UUID setoran, tanpa token pada URL. Form mitra menyimpan niat per UID sebelum POST; hasil belum diketahui dipulihkan dengan payload/key sama, hasil selesai dengan ID setoran. QR/token hanya di memori, warning selalu ditampilkan dan countdown memakai waktu monotonic.

Screenshot layar QR kini diizinkan atas permintaan pemilik untuk laporan; FE tidak lagi memasang `FLAG_SECURE`. QR tetap bearer token: siapa pun yang memperoleh screenshot QR aktif berpotensi mengklaim setoran. Gunakan QR yang sudah diklaim/kedaluwarsa atau sensor gambarnya sebelum membagikan laporan. Aplikasi tetap tidak menulis token ke penyimpanan/log secara otomatis.

### 2.8 Saldo, riwayat, reward, voucher

Fitur: [wallet-history.md](wallet-history.md), [rewards.md](rewards.md). FE: `PointsRoute`, `PointHistoryRoute`, `VoucherListRoute`, `VoucherDetailRoute`, serta `WalletRepository`; bukan lagi placeholder. Katalog/detail reward, konfirmasi tukar, filter/cursor riwayat/voucher, dan konfirmasi pemakaian voucher memakai server. Niat tukar disimpan per UID sebelum POST dan hasil belum diketahui tidak dibuang saat logout/proses mati.

- Saldo: `GET /me/wallet`. Riwayat: `GET /me/point-history?type=deposit|redemption&from=&to=&limit=&cursor=`. Cursor buram, tidak disimpan lintas sesi, tidak dipakai ulang bila filter berubah (mulai dari halaman pertama). `+` pada offset waktu di-encode `%2B07:00`, atau pakai `Z`; `from` inklusif, `to` eksklusif; batas hari/bulan dihitung Android menurut zona pengguna. Entri `deposit` → detail `GET /me/deposits/{deposit.id}`; entri `redemption` bernilai negatif.
- Reward: `GET /rewards` (`available=false` = stok habis; urut biaya termurah, maks. 100, tanpa pagination) dan `/rewards/{id}`.
- Tukar: `POST /rewards/{id}/redeem` dengan key baru per penekanan tombol (§1, tersimpan sebelum kirim); 201/200 → `voucher` (+`code`), `wallet_balance`. Kegagalan tidak memotong poin. Error: 409 `insufficient_points` (tampilkan selisih dari saldo), 409 `out_of_stock` (muat ulang katalog), 404 `reward_not_found`, 403 `redeem_forbidden`, 409 `idempotency_conflict` (key baru hanya bila niat berubah).
- Voucher: `GET /me/vouchers?status=issued|used|expired` (cursor), detail, `POST /me/vouchers/{id}/use` (idempotent; 410 `voucher_expired`). Berlaku 7 hari, tanpa refund, tanpa pencairan nyata (simulasi, D-08, D-20). `code` hanya ditampilkan di layar voucher, tidak dicatat di log.

### 2.9 Misi, badge, leaderboard

Fitur: [gamification.md](gamification.md). Home/misi, Games, badge/pencapaian, serta leaderboard memakai data server. Mini game tetap latihan offline: tidak mengirim skor atau memberi XP/EcoPoints/badge server karena endpoint-nya tidak tersedia.

- `GET /me/missions`: katalog misi mingguan (Senin 00.00 WIB, akhir eksklusif). Sisa waktu = `period_end − server_time`. Progres per `unit`: `deposit` (jumlah) dan `gram` (3000 = 3 kg). Hadiah **XP**.
- `status = claimable` → `POST /me/missions/{id}/claim {"period_start":"<nilai persis dari GET>"}`: 201 pertama, 200 ulangan, 410 `mission_expired` (periode berganti → muat ulang), 409 `mission_incomplete`. Simpan `period_start` selama proses klaim; retry memakai nilai yang sama.
- Beranda hanya menampilkan satu misi dari katalog yang sama; pilihan misi adalah keputusan FE.
- `GET /me/badges`: permanen dan otomatis (tanpa klaim). `GET /leaderboard?limit=`: XP mingguan, `me` bisa `null`; peringkat seri berperingkat sama (1,1,3); `photo_path` bisa `null` dan bukan URL penuh. `PATCH /me/leaderboard-visibility {"visible":bool}` (user saja).
- Status FE: daftar/klaim misi memakai periode server, badge memakai progres/earned aktual, dan kedua pintu leaderboard menampilkan **XP mingguan**, bukan peringkat setoran/skor game palsu. `LocalLeaderboard` dan kartu hadiah game tetap telah dihapus. Saldo EcoPoints tidak ditambah lokal.

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

Fitur: [support.md](support.md). FE: `SupportRoute` menangani FAQ, form, daftar dan detail tiket lewat state internal; pusat bantuan tidak lagi membaca FAQ lokal. Form memakai `PendingMutationStore` sebelum POST dan dapat memulihkan key/payload atau ID hasil yang sudah selesai.

- `GET /help/faqs` (demo, plain text). Tiket: `POST /me/support-tickets` `{kind: problem|feedback, subject, message}` dengan key (§1); `GET /me/support-tickets?status=` dan detail untuk melihat balasan.
- Tidak ada kontak publik, email, lampiran, push, atau janji waktu balasan. Balasan manual dibaca dengan membuka ulang daftar/detail; `staff_response=null` → "Belum ada balasan". Error: 400 `invalid_ticket`, 429 `ticket_limit_reached`, 413 bila >64 KiB.

### 2.13 Profil, pengaturan, logout

Fitur: [accounts.md](accounts.md). FE: `ProfileRoute`, `AccountSettingsRoute`, `AccountSecurityRoute` dan `ProfileRepository` memuat ulang profil/statistik saat resume; edit nama/telepon memakai PATCH dan meminta refresh akun root setelah sukses. Settings menampilkan nilai server, write gagal mempertahankan nilai sebelumnya, dan logout Firebase membuang graph/state akun. Toggle/fitur yang tidak didukung server tidak dibuat seolah tersimpan.

- Profil: `GET /me`; edit `PATCH /me` hanya `full_name` dan `phone` (field lain ditolak 400 `invalid_profile`/`invalid_request`). Statistik (`GreenStats`, semua String di FE; formatkan di UI): `ecoPoints` = `wallet.balance`; `totalDeposits` = `deposits_claimed` dari `/me/home` (**all-time**, bukan `monthly_deposits`; label "bulan ini" hanya untuk `monthly_deposits`); `totalBadges` = jumlah elemen `badges[]` dari `GET /me/badges` dengan `earned == true`, **bukan** panjang seluruh katalog. Mitra tidak memiliki statistik ini (`gamification_forbidden`); sembunyikan atau ganti dengan `/partner/home`.
- Settings: `GET/PATCH /me/settings`: backend hanya menyimpan `notification_enabled` dan `onboarding_completed`; `show_on_leaderboard` ditulis lewat `/me/leaderboard-visibility`. Toggle notifikasi lain di UI tidak punya penyimpanan server.
- Logout: `FirebaseAuth.signOut()` dan kosongkan token/state di memori serta cache turunan (profil, saldo, daftar). **Jangan** menghapus niat finansial yang hasilnya belum diketahui (setoran, penukaran, tiket dengan key tersimpan): simpan per UID, sembunyikan dari UI saat logout, dan pertahankan sampai UID yang sama login lagi lalu menyelesaikannya (ulangi key yang sama atau `GET` untuk memeriksa hasil). Tidak pernah dipakai oleh UID lain. Hapus hanya niat yang sudah resolve. Tidak ada endpoint logout di backend.
- Email/password: Firebase. Tidak ada upload foto profil dan penghapusan akun.

### 2.14 Artikel

Fitur: [articles.md](articles.md). FE: `ArticleListRoute`, `ArticleDetailRoute`, dan `ArticleRepository` memakai API daftar/detail; kartu Home meneruskan UUID artikel server. `LocalArticles` tidak lagi menjadi sumber runtime.

1. Daftar: `GET /articles?limit=&cursor=&q=&category=&featured=true`. Cursor buram: teruskan `pagination.next_cursor`, berhenti saat `null`; mulai ulang dari halaman pertama bila `q`/`category`/`featured` berubah (cursor tidak dipakai lintas filter). `q` mencari judul/ringkasan (maks. 100 karakter). `category` salah satu `GUIDE`, `SORTING`, `RECYCLING`, `SPECIAL_WASTE`, `NEWS`; kategori baru yang tidak dikenal enum FE ditampilkan generik. `featured=true` untuk kartu unggulan. Hanya artikel terbit (bukan draft/terjadwal), terbaru di atas.
2. Detail: `GET /articles/{id}` memakai `id` dari daftar atau dari Home. Konten (`introduction`, `sections[].title/body`, `tip`) adalah **teks biasa**, bukan HTML/Markdown; tampilkan sebagai teks. `sections` bisa `[]`.
3. Error: 400 `invalid_query` (parameter asing/ganda, kategori/`featured`/cursor salah: bug klien), 404 `article_not_found` (draft, belum terbit, atau ID salah; kembali ke daftar dan muat ulang).
4. "Selesai membaca" hanyalah navigasi lokal Android: server tidak mencatatnya dan tidak memberi poin atau XP. Jangan menampilkan hadiah.
5. State: loading, daftar kosong (`[]`, mis. filter tanpa hasil), error dapat diulang (jaringan/5xx/504). Konten adalah data demo (`is_demo`).

## 3. Ringkasan integrasi FE

| Area | Status FE | Batas / langkah berikutnya |
|---|---|---|
| Dependency/jaringan | Firebase Auth, `BackendClient`, DTO/parser JSON, konfigurasi URL build dan cleartext debug per host | Token diambil dari SDK per request; satu refresh/replay pada 401, tanpa log token/body sensitif |
| Auth | Login/registrasi, pemulihan profil, sesi/logout dan pengiriman email reset Firebase nyata | Tidak ada Google sign-in atau form ubah password tanpa reset code |
| Navigasi per role | Graph user dan operasional mitra terpisah | Pending/suspended tetap mengikuti izin server; persetujuan bukan dari Android |
| Onboarding | Flag lokal sebelum login dan sinkronisasi selesai ke server secara best-effort | Bukan gerbang akses server; kebijakan produk lanjutan tetap Q-12 |
| Beranda | `GET /me/home` dan `/partner/home`; kartu saldo/setoran/misi/mitra/artikel serta unread count memakai server | Loading/error/empty nyata; saran mitra bukan hasil GPS |
| Peta/favorit | Kategori, daftar/filter/paging, detail UUID, dan favorit per akun tersambung | Tidak ada radius/GPS; peta perlu OpenGL ES 3.0 |
| Transaksi | Estimasi, create setoran mitra, issue/reissue QR, preview/klaim user dan detail/riwayat setoran memakai server | QR bearer hanya memori; mutasi create memakai niat/key persisten per UID |
| Riwayat/reward/voucher | Saldo, cursor/filter riwayat, katalog/detail/tukar reward, daftar/detail/pakai voucher tersambung | Voucher simulasi; kode tidak disimpan/log; replay memakai key sama |
| Misi/badge/leaderboard | Daftar/klaim, badge dan XP mingguan tersambung; Games menampilkan progres nyata | Game offline tidak mempunyai endpoint skor/hadiah |
| Laporan, bantuan, inbox | JSON/PDF terautentikasi, FAQ/tiket, inbox cursor/unread/read/deep-link tersedia di FE | Tidak ada push/SLA; PDF perlu viewer eksternal atau aplikasi berbagi |
| Artikel | Daftar/detail/pencarian/filter/cursor dari server; UUID kartu Home konsisten | Konten seed tetap berlabel demo |
| Profil/logout | Profil/statistik per role, PATCH nama/telepon, preferensi notifikasi/visibilitas dan logout tersambung | Tidak ada upload foto, penghapusan akun atau perubahan email server |

### 3.1 Cakupan UI lintas branch frontend

Inventaris membandingkan tip GitHub branch `00`–`13`, `dev`, dan `main`, bukan hanya cached `origin/*` lokal. Sumber desain yang sebelumnya terlewat: `12-feat/admin` (`316e390`), `11-feat/points` (`e71e050`), `13-feat/ai-waste-scan` (`4310cd7`), serta podium/peringkat dari `07-feat/leaderboard` (`428bb24`). Desain dipakai ulang dengan state/repository BE saat ini; tidak mengembalikan transaksi, saldo, ranking, atau respons AI palsu.

| UI sumber | Layar/alur integrasi dan keputusan |
|---|---|
| Design system, splash, onboarding | Theme/Eco controls tetap dipakai; splash/onboarding dirender langsung dari state sesi |
| Login, register, forgot/check email | Firebase + akun BE; status email terkirim berada dalam layar reset yang sama |
| New password | Ditangani tautan reset resmi Firebase, bukan form sukses palsu tanpa action code |
| Bantuan sebelum login | Petunjuk akun; pusat bantuan/tiket milik akun dibuka setelah login |
| Beranda user, misi | Data Home server; aksi saldo, scan, peta, riwayat, artikel, misi, inbox dan profil tetap terhubung |
| Peta dan detail mitra | Dipakai kedua role; pencarian/filter/favorit/detail/kontak/arah memakai data server |
| Daftar dan detail artikel | Pencarian/filter/cursor/UUID server; bukan artikel lokal |
| Games home, katalog, detail, main, hasil | Hasil game dirender **inline** oleh `GamePlayScreen`, bukan rute yang hilang. Mini game tetap latihan offline tanpa XP server |
| Leaderboard, podium, ranking, posisi sendiri | Desain podium/kartu asli dipulihkan memakai rank/XP server dan kondisi seri; kedua pintu lama menuju XP mingguan, tanpa mode setoran/skor game fiktif |
| Kamera/galeri, review, hasil AI | Kamera → **Konfirmasi Foto Sampah** → Analisis Foto → hasil/error. Tidak mengunggah ke AI sebelum konfirmasi; retake/cancel membersihkan salinan cache milik aplikasi |
| Scan QR, preview, hasil transaksi | Decoder nyata → preview BE → konfirmasi klaim → detail setoran server; komponen poin/item menggantikan hasil transaksi lokal |
| Poin/reward dan detail/konfirmasi tukar | Hero saldo, ilustrasi, aksi Scan QR/Kalkulator, katalog dua kolom dan detail mengikuti desain branch poin; saldo/stok/penukaran nyata |
| Kategori/promo reward lama | Tidak dikembalikan sebagai data palsu. Katalog dikelompokkan berdasarkan stok dan kecukupan saldo aktual, dengan angka kekurangan poin; banner menjelaskan voucher simulasi |
| Voucher, riwayat poin | Daftar/filter/cursor, detail dan pemakaian voucher; deep-link riwayat menuju ID setoran/voucher server |
| Profil, pengaturan, akun/keamanan | Profil/statistik per role, PATCH nama/telepon, preferensi notifikasi/visibilitas dan email reset; bukan toggle lokal |
| Help center/FAQ, form/daftar/detail tiket | `SupportRoute` menggantikan FAQ lokal dengan FAQ/tiket/balasan server; beberapa layar memakai state internal, bukan rute mati |
| Laporan/PDF, pencapaian/badge, inbox | Rute laporan dua role, badge user-only, inbox dua role dengan read/deep-link |
| AdminWorkspace/MitraScaffold | Dipulihkan sebagai `PartnerTabScaffold`: header aplikasi dan tab **Beranda · QR · Kalkulator**. Folder sumber “admin” adalah workspace mitra, bukan role admin baru |
| AdminHome | Layout PANEL MITRA, kartu Menu Utama dan Cara Kerja dipulihkan; identitas/status/ringkasan/recent deposits tetap dari BE |
| AdminCalculator | Layout kalkulator/hero input dipulihkan; beberapa kategori/berat dan tarif server. Setelah estimasi, **Lanjut Buat QR** membawa kategori/berat ke form setoran tanpa membawa saldo/tarif/poin buatan klien |
| GenerateQr | Form → POST setoran idempotent → Buka Setoran → issue/reissue QR. Pending intent selalu mendahului draft kalkulator; token/warning/countdown berasal dari BE, TTL 10 menit |
| Riwayat/antrean setoran mitra | Tab QR membuka daftar/pilih setoran dan aksi buat baru, **tidak** membuat transaksi atau menerbitkan QR hanya karena tab ditekan |
| Profil/pelengkapan sesi gagal | Pemulihan profil dan sesi unavailable tetap tersedia; back stack/state dipisahkan per UID dan role |

Ketercakupan layar/rute dan kesesuaian desain bukan bukti seluruh fitur cloud lolos end-to-end. Tidak ada endpoint server untuk skor/hadiah game, ranking setoran/game, kategori/promo reward lama, upload foto profil, penghapusan akun, push, refund/banding, riwayat scan AI, maupun persetujuan mitra melalui dashboard. Google sign-in juga belum dikonfigurasi. Kontrol yang tidak didukung tidak boleh mengaku menyimpan atau memberi hadiah.

## 4. Belum tersedia atau ditunda

Android mempunyai klien multipart scan AI tanpa fallback demo; service FastAPI port 8000 kini tersedia di `EcoSnap-BE/ai-service/` ([ai-scan.md](ai-scan.md)), terpisah dari 46 operasi Go. Runtime/model nyata memerlukan service aktif dan credential provider di server; bukan API key di FE. Tidak ada endpoint riwayat scan, skor/hadiah game, push, upload foto profil, banding/refund, penghapusan akun atau persetujuan mitra dari Android. Hosting dan HTTPS publik juga belum disediakan. Bagian itu tidak dinyatakan sudah berfungsi end-to-end.

## 5. Arsip: audit frontend 17 November 2026

Dipertahankan sebagai bukti, bukan status terkini. Pada audit itu Android belum memanggil backend: login/registrasi hanya navigasi, beranda memakai data demo, peta/klaim/game/leaderboard/artikel memakai data lokal, dan logout TODO. Sebagian besar kontrak yang saat itu hanya rencana kini sudah ada (lihat README §4); sisa temuan yang masih berlaku ada di §3.

Kontrak dua route awal:

- `GET /health` (publik) → `{"status":"ok"}`; hanya memeriksa proses HTTP.
- `GET /api/v1/auth/me` → `{"data":{"uid":"…","email":"…","email_verified":false}}` dari claim token terverifikasi; tidak membaca PostgreSQL dan bukan dasar otorisasi. 401 `unauthorized` untuk header hilang/salah atau token ditolak. Untuk profil aplikasi gunakan `GET /me`.

Backend memakai HTTP biasa saat development dan default listener 127.0.0.1; alamat emulator/perangkat dan cleartext dijelaskan di [README §1](README.md). Seluruh riwayat keputusan dan status task ada di [BUILD_PROCESS.md](../../BUILD_PROCESS.md).
