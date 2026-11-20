# Audit kebutuhan backend berdasarkan frontend EcoSnap

Tanggal pemeriksaan: 2026-11-17. Status: audit source dan rencana implementasi; tidak ada source aplikasi yang diubah pada tahap ini. Dokumen ini dibaca bersama [BUILD_PROCESS.md](../BUILD_PROCESS.md), yang tetap menjadi tracker utama.

Snapshot audit ini dipertahankan sebagai bukti sebelum implementasi akun. Setelah audit, pemilik mengizinkan batch akun; status aktif dan perubahan sumber dirujuk ke BUILD_PROCESS.md dan docs/api/accounts.md. Pernyataan dua route dan hash dalam audit ini menggambarkan waktu snapshot.

## 1. Acuan dan batas pemeriksaan

Pemilik menetapkan folder EcoSnap-FE di samping EcoSnap-BE sebagai versi final acuan. Seluruh menu placeholder tetap masuk scope. Backend diselesaikan berdasarkan kebutuhan frontend, lalu integrasi dilakukan setelahnya. Desain dan alur yang ada dipertahankan; kebutuhan perubahan data/callback untuk integrasi dicatat. Konflik bisnis dan kebutuhan layar baru harus dibahas, tidak diputuskan dari asumsi implementasi.

Target pemilik adalah seluruh fitur selesai secepatnya. Tanggal deadline pasti belum diberikan. Pernyataan ini tidak menghasilkan estimasi tanggal selesai atau prioritas fitur yang disepakati.

Bukti yang diperiksa:

- Seluruh file Kotlin di app/src/main dicakup inventaris deklarasi dan hash; penelaahan perilaku berfokus pada navigation, model, sumber data lokal, layar fitur, dan bagian yang menangani aksi. Style/theme serta layout dekoratif bukan sumber aturan bisnis.
- Konfigurasi app/build.gradle.kts, build.gradle.kts, gradle/libs.versions.toml, dan AndroidManifest.xml diperiksa untuk dependency dan izin.
- Seluruh source Go aktif, router, middleware, startup, migration SQL, dan DBML awal diperiksa. DBML adalah rancangan; keberadaannya tidak membuktikan semua tabel telah dimigrasi.
- PDF dan gambar UI yang sudah diarsipkan tetap menjadi bukti fitur yang belum tampak sebagai layar lengkap pada source Kotlin. Tidak ada penarikan ulang Figma dalam audit ini.
- [source-inventory.json](audit/source-inventory.json) berisi 167 file: 149 Kotlin termasuk dua file contoh test, 11 Go termasuk satu file test, serta file konfigurasi/schema. Hash menunjukkan working tree saat audit, bukan sekadar commit. HEAD frontend: `7b08292332cec5b1b3539195eb94aeffb52a5110`.

Build/runtime Android, layanan Firebase nyata, model AI, dan database remote tidak diuji dalam audit source ini. Bukti pengujian backend dicatat terpisah pada tracker. Tidak ada pembacaan private key, file konfigurasi lokal berisi credential, atau perubahan schema remote.

Path bukti FE di bawah relatif ke `../EcoSnap-FE/app/src/main/java/com/example/ecosnap/` dari root backend. Nomor baris adalah snapshot dan perlu dicocokkan ulang jika hash source berubah.

## 2. Keputusan pemilik selama audit

| ID | Keputusan atau laporan | Konsekuensi |
|---|---|---|
| AUD-D01 | Folder FE saat ini menjadi acuan final; semua placeholder termasuk scope. | Placeholder tetap mempunyai task backend dan integrasi; tidak dihapus dari scope. |
| AUD-D02 | Target seluruh fitur selesai secepatnya. | Urutan teknis disusun dari dependency; tanggal selesai belum bisa ditetapkan. |
| AUD-D03 | Desain/alur dipertahankan; kebutuhan integrasi boleh dicatat. | Penggantian data demo, DTO, callback, dan state bukan alasan merombak tampilan. Layar tambahan perlu keputusan. |
| AUD-D04 | Data mitra demo menggunakan seed. | Isi lokasi, jam, kategori, serta hubungan akun/cabang secara deterministik pada database development. |
| AUD-D05 | Tarif sementara boleh memakai data contoh yang nanti dapat diganti tim. | Data dummy ditaruh pada seed/config. Rekomendasi teknis: hasil kalkulasi tetap deterministik dan tarif transaksi disimpan sebagai snapshot. Nilai tarif dan formula final belum dikonfirmasi. |
| AUD-D06 | Model AI dan game sedang dikerjakan teman pemilik. | Ini laporan pekerjaan berjalan; spesifikasi/artefak integrasi belum diterima. Jangan menyatakan model siap atau mengarang output. |
| AUD-D07 | Reward tetap simulasi voucher; sistem reward harus bekerja sebenarnya. | Saldo, stok, ledger, kepemilikan, penerbitan voucher, dan retry harus nyata. Integrasi DANA/pencairan tidak termasuk. Lifecycle pemakaian/refund/expiry belum final. |
| AUD-D08 | Mitra boleh mendaftar tetapi tidak boleh membuat QR sebelum data usaha lengkap dan disetujui. | Registrasi tidak otomatis memberi izin transaksi. Backend perlu memeriksa kelengkapan, status persetujuan, dan kepemilikan lokasi. |
| AUD-D09 | Keputusan terakhir: satu akun mitra mengelola satu tempat, agar sesuai frontend yang ada. Ini menggantikan jawaban multi-cabang sebelumnya. | Unique partners.mitra_uid pada DBML tetap sesuai. Tidak menambah pemilihan cabang atau rancangan usaha multi-cabang. Setoran/QR terikat tempat milik akun. |
| AUD-D10 | Repo/spec AI dan game belum tersedia; handoff boleh ditunda. Aturan XP belum ada dan harus fair/tidak dapat dieksploitasi. | Fitur tetap scope, tetapi kontrak AI dan kebijakan reward replay belum final. Pengerjaan fitur independen dapat direncanakan tanpa output model palsu. |

Keputusan lama yang tetap berlaku: Go; PostgreSQL Supabase; Firebase Auth dan media Firebase Storage; setoran multi-material; QR berlaku 10 menit dan reissue menonaktifkan QR lama; EcoPoints terpisah dari XP; admin dan Docker belum dikerjakan. Seed akun demo tidak berarti password atau private key disimpan pada repository.

## 3. Peta layar dan operasi backend

Status FE berikut menjelaskan perilaku source lokal, bukan penilaian kualitas desain. Operasi backend selain dua route yang disebut pada bagian 6 adalah kebutuhan/usulan, belum tersedia. Nama endpoint final dibekukan pada kontrak per fitur.

| ID | Layar/aksi FE dan bukti | Data dan operasi yang diperlukan | Bukti selesai yang harus diperiksa |
|---|---|---|---|
| FE-01 | Splash/onboarding: navigation/EcoSnapNavHost.kt:22 dan :30 selalu menuju onboarding; OnboardingPreferences ada tetapi belum dipakai NavHost. | Tentukan state onboarding lokal vs akun dan pemulihan sesi Firebase. Tidak setiap slide perlu endpoint. | First launch, skip, launch ulang, sesi aktif, logout punya hasil navigasi yang ditentukan. |
| FE-02 | Login: navigation/AuthNavGraph.kt:62 mengabaikan email/password lalu navigasi. | Firebase login di Android; verifikasi token Go; ambil profil PostgreSQL dan role/status. | Token valid, invalid, expired, akun belum punya profil, serta akun suspended ditangani. |
| FE-03 | Registrasi user/mitra: features/auth/model/AuthRole.kt:10; AuthNavGraph.kt:91 belum menyimpan data. | Pembuatan profil idempotent dengan settings/wallet; simpan permintaan usaha tanpa otomatis memberi izin QR. | Retry dan request bersamaan tidak menggandakan data; role ditentukan aturan server. |
| FE-04 | Forgot/check email/new password: navigation/AuthNavGraph.kt. | Firebase reset email dan action code; backend tidak membangun penyimpanan password kedua. | Reset link kedaluwarsa, resend, perubahan password, dan login ulang diuji. |
| FE-05 | Beranda: MainNavGraph.kt:86 memakai HomeUiState.demo(). | Nama, saldo, jumlah setoran bulanan, progres misi, mitra sekitar, dan berita terbaru. | Data terhubung ke sumber fitur; zero/empty tidak diganti angka demo. |
| FE-06 | Peta: features/map/PartnerMapScreen.kt:58 dan :76 memfilter data lokal menurut nama/area dan material. | Daftar cabang mitra aktif, pencarian nama/area, filter kategori; lokasi/radius hanya setelah ketersediaan posisi disepakati. | Kombinasi filter, hasil kosong, koordinat invalid, serta branch visibility diuji. |
| FE-07 | Detail mitra: WastePartner model; MainNavGraph.kt:146. Telepon dan petunjuk arah memakai intent Android. | ID cabang, nama, area, alamat, telepon, jam, koordinat, kategori diterima. | Detail konsisten dengan peta; dial/directions tetap dijalankan Android, tidak perlu endpoint telepon/rute Go. |
| FE-08 | Favorit mitra: PartnerMapScreen.kt:72 menyimpan favoriteIds dengan remember. | Jika favorit harus persisten: baca/simpan/hapus favorit milik caller per cabang. | Status bertahan setelah aplikasi dibuka ulang; tidak dapat mengubah favorit user lain. Kebijakan persistensi belum dikonfirmasi. |
| FE-09 | Foto/galeri: features/photos/CameraScreen.kt menerima gambar dan menyimpannya pada cache perangkat. | Upload byte atau upload storage lalu metadata terverifikasi; URI lokal Android tidak dapat dibaca langsung oleh Go. | Izin ditolak, file salah/terlalu besar, upload gagal, serta kepemilikan media ditangani. |
| FE-10 | Hasil scan: features/photos/PhotoResultScreen.kt:60 memakai DemoWastePhotosRepository. | Identifikasi, material/kategori/tags, deskripsi, panduan, ide reuse, potensi industri; status processing/failed/unknown. | Output model nyata dipetakan ke tampilan; kegagalan tidak disajikan sebagai hasil botol hardcoded. |
| FE-11 | Panduan reuse buka/tutup: PhotoReuseIdeaCard dan ReuseIdea model. | Daftar alat/bahan, langkah terurut, deskripsi; pertimbangkan difficulty dan safety dari DBML. | Urutan stabil dan konten aman; buka/tutup merupakan state UI, bukan mutasi server. |
| FE-12 | Scan QR: ScanQrScreen.kt:113 dan :122 menunda lalu mengirim payload dummy; analyzer belum terpasang. | Decoder QR di Android lalu preview/klaim token di Go. | Pemindaian kamera/galeri menghasilkan payload sebenarnya; malformed/expired/revoked/used dibedakan. |
| FE-13 | Hasil setoran: ScanNavGraph mencari LocalDeposits; DepositTransaction model. | Receipt transaksi tersimpan: kode, mitra/cabang, total berat, poin, waktu, status. | Sukses hanya setelah commit klaim; timeout ditangani dengan status/retry aman. |
| FE-14 | Poin/reward: MainNavGraph.kt:257 hanya judul dan tombol leaderboard. UI lengkap ada pada bukti desain. | Wallet, katalog/kategori/detail reward, penukaran simulasi, koleksi voucher. | Saldo/stock tidak negatif; retry tidak menggandakan potongan atau voucher; voucher milik user benar. |
| FE-15 | Games home/catalog/detail: LocalGames; MiniGame model; pencarian katalog. | Daftar/detail game, about, difficulty, instructions, total soal, XP per benar. | Metadata konsisten dengan attempt yang dibuat; game inactive tidak dapat dimulai. |
| FE-16 | Gameplay/result/replay: GamePlayScreen.kt:120, :267, :268 menilai dan menambah XP lokal. | Attempt, question/choice ID, submit jawaban, feedback, hasil, replay policy. | Jawaban dinilai server; soal/choice milik attempt; retry tidak menggandakan XP. Hasil adalah state layar, bukan route mandiri yang wajib dibuat. |
| FE-17 | Misi: MainNavGraph.kt:109 dan GamesNavGraph menuju MISSIONS; tidak ditemukan destination pendaftarannya. WeeklyChallengeCard memakai contoh. | Daftar/detail/progres misi, deadline, hadiah, klaim, hubungan event transaksi/game/scan. | Aturan tim diterima; progres dan hadiah tidak dapat digandakan; kebutuhan destination Android dicatat. |
| FE-18 | Leaderboard dua mode: LeaderboardScreen dan LocalLeaderboard. | Top 3, ranking lanjutan, rank caller, periode dan unit score. | Poin setoran tidak turun akibat redemption; XP terpisah; tie-break/privasi dipastikan. |
| FE-19 | Artikel daftar/detail/selesai membaca: LocalArticles, ArticleListScreen, ArticleDetailScreen. | Daftar terbit, pencarian title/summary, kategori, featured, durasi baca, introduction/sections/tip. | Konten draft tersembunyi; detail sesuai ID; selesai membaca saat ini hanya navigasi, tidak otomatis menghasilkan poin. |
| FE-20 | Profil: ProfileScreen memakai ProfileDummyData; edit menuju halaman privasi placeholder. | Nama/email, saldo atau total earned sesuai definisi, jumlah setoran/badge; perubahan profil yang diizinkan. | Data caller benar; makna ecoPoints disepakati; role/status tidak dapat diedit oleh user. |
| FE-21 | Pengaturan: AccountSettingsScreen; NotificationPreferenceRow.kt:28 hanya label/icon tanpa switch/state. | Jika dapat diubah: preference transaksi dan misi yang terpisah; visibilitas leaderboard. | Pilihan tersimpan dan benar-benar memengaruhi pengiriman/visibilitas; jangan menganggap label sebagai kontrol aktif. |
| FE-22 | FAQ/help: ProfileDummyData dan HelpCenterScreen; kontak auth placeholder. | FAQ dapat tetap statis kecuali tim meminta pengelolaan server. Form masalah/masukan dari desain memerlukan support request dan media. | Pengiriman tersimpan, field wajib tervalidasi, user tidak membaca laporan orang lain. |
| FE-23 | Riwayat, reward saya, pencapaian, laporan, privasi: ProfileNavGraph menggunakan ProfilePlaceholderScreen. | Daftar/detail aktivitas, voucher, badges/progres, ringkasan laporan dan PDF, pengaturan privasi. | Tiap operasi diuji ownership/filter/empty state; tampilan desain dipertahankan saat integrasi. |
| FE-24 | Notifikasi dan History utama: MainNavGraph.kt:299 dan sesudahnya membuat MainPlaceholder. | Inbox/read status; riwayat setoran dan ledger dengan filter/pagination. | Read status idempotent, data per user; push dibahas terpisah dari inbox. |
| FE-25 | Dashboard mitra/kalkulator/generate QR terlihat pada arsip UI, tidak ditemukan layar/graph mitra pada source lokal. | Cabang milik mitra, estimasi multi-item, create deposit, issue/reissue QR 10 menit, status klaim. | Mitra pending dilarang; pemilik cabang diperiksa; reissue vs claim paralel aman. Tetap scope sesuai AUD-D01. |
| FE-26 | Logout: ProfileNavGraph callback TODO. | Firebase signOut di Android; kebijakan revocation hanya jika dibutuhkan. | Sesi lokal/token terhapus dan navigasi terlindungi; perubahan status server ditegakkan pada endpoint bisnis. |

## 4. Bentuk data yang harus dipenuhi backend

Model Kotlin adalah bentuk kebutuhan tampilan. DTO jaringan boleh berbeda selama pemetaannya jelas dan hasil UI tetap sesuai. Snake_case/envelope pada endpoint pertama adalah keputusan implementasi awal; belum ada DTO jaringan Android yang membekukan semua endpoint.

| Model FE | Field yang dipakai | Pemetaan dan celah |
|---|---|---|
| RegistrationData | role, fullName, email, phone, password, businessName?, businessCategory? | USER/PARTNER perlu pemetaan user/mitra; password hanya ke Firebase; display_name wajib DB belum ada inputnya; alamat/cabang belum ada pada form. |
| HomeUiState / NearbyPartnerUi | userName, points, monthlyDeposits, missionTitle/Description/Progress/Target/Reward, nearbyPartner(name/location/acceptedMaterials) | Tambahkan ID stabil dalam DTO untuk detail mitra; source sekarang hardcode ID tunas. Definisi bulanan dan pemilihan misi perlu kontrak. |
| WastePartner / PartnerLocation | id, name, area, address, phone, openingHours, latitude/longitude, acceptedMaterials | ID harus menunjuk cabang. DBML belum punya area; jam terstruktur perlu formatter. Enum material PLASTIC/PAPER/METAL/GLASS tidak mencakup semua kategori desain. |
| WastePhotosResult | objectName, category, material, tags, description, handlingSteps, reuseIdeas, industryPotential | DBML ai_result JSONB belum menetapkan struktur field ini. Output provider harus divalidasi dan dipetakan; jangan menganggap kategori/model selalu cocok. |
| ReuseIdea | id, title, description, toolsAndMaterials[], steps[] | DBML tools_json/steps_json perlu schema JSON; safety dan difficulty belum menjadi field model FE. |
| DepositTransaction | id, partnerName, points, weightKg, date, status | Pisahkan UUID/kode tampilan; date dan status tampilan diformat Android; receipt ringkas tidak menghilangkan rincian multi-item server. |
| MiniGame | id, title, description, about, type, difficulty, xpPerCorrect, instructions[], questions[] | DBML games belum memuat about/difficulty/instructions; jangan mengirim semua kunci jawaban bersama questions. |
| GameQuestion / GameVisual | id, title, prompt, visual, options[], correctAnswer, explanation | Butuh choice ID stabil; visual enum perlu representasi yang cocok dengan aset FE. correctAnswer hanya boleh dibuka sesuai aturan feedback setelah jawaban diterima. |
| GameResult | correctAnswers, totalQuestions, earnedXp | Dihitung server dari attempt, bukan menerima skor hasil hitung client sebagai bukti. |
| LeaderboardEntry / RankedLeaderboardEntry | id, name, score, isCurrentUser, rank | Penentuan caller memakai token; current-user dummy bukan ID nyata; top/my-rank harus konsisten. |
| Article / ArticleSection / ArticleCategory | id/title/summary/category/readingMinutes/introduction/sections(title/body)/tip/isFeatured; GUIDE/SORTING/RECYCLING/SPECIAL_WASTE/NEWS | DBML hanya body_markdown/category teks dan tidak punya readingMinutes/tip/isFeatured. Pilih penyimpanan terstruktur atau adapter yang eksplisit; jangan menebak pemecahan markdown. |
| UserProfile / GreenStats | name/email; ecoPoints/totalDeposits/totalBadges berupa String | Server mengirim angka asli dan Android memformat. Total earned vs saldo belum dipastikan. |
| NotificationPreferenceItem / FaqItem / ProfileMenuItemData | label/deskripsi; question/answer; judul/subjudul/icon | Label dan icon navigasi tidak perlu tabel per komponen. State preference baru diperlukan bila setting benar-benar dapat diubah. |

## 5. Selisih schema dan aturan yang menahan code generation

Tabel ini memuat gap terbuka dan gap yang telah terjawab. Semua perubahan schema yang masih diusulkan tetap rancangan. Migration pertama yang sudah ada tidak diubah dalam audit ini.

| ID | Bukti | Tindakan sebelum migration fitur |
|---|---|---|
| GAP-01 (TERJAWAB) | Keputusan terakhir AUD-D09: satu akun satu tempat; partners.mitra_uid unique sudah sesuai. | Pertahankan constraint; jangan membuat business/branches atau selector cabang. Hipotesis konflik multi-cabang dari jawaban awal ditutup. |
| GAP-02 | partners.address wajib; registrasi hanya mengirim nama/kategori usaha; AUD-D08 mengharuskan pending. | Simpan permintaan/profil usaha belum lengkap tanpa alamat palsu; tentukan pelengkapan/persetujuan manual sementara. Seed hanya menyiapkan lokasi demo. |
| GAP-03 | deposits hanya total weight/rate/points; tidak ada deposit_items. | Tambah rincian kategori/berat/tarif snapshot/poin item; tentukan apakah tarif satu atau per kategori. UI total tetap dapat dipenuhi dari agregat. |
| GAP-04 | point_rates belum punya category_id atau aturan prioritas/fallback. | Tempatkan tarif dummy editable; finalisasi struktur tarif sebelum migration dan acceptance test kalkulasi. |
| GAP-05 | qr_claims deposit_id unique membatasi satu row. | Reissue dapat memperbarui row atau menyimpan histori terpisah; keputusan histori terbuka. Apa pun pilihannya, token lama langsung invalid dan tepat saat expiry ditolak. |
| GAP-06 | games/question schema belum memuat seluruh metadata MiniGame/GameVisual. | Cocokkan dengan hasil handoff teman; backend memegang jawaban, attempt, dan XP. |
| GAP-07 | mission_type belum mencakup distinct category deposit; kartu game menyebut tiga jenis sampah, kartu home menyebut tiga kali setoran. | Pastikan dua misi berbeda atau satu definisi yang dikoreksi. Jangan memakai deposit_count untuk distinct categories tanpa keputusan. |
| GAP-08 | Satu notification_enabled di DB vs dua label preference pada FE. | Tentukan apakah hanya global atau transaksi/misi terpisah. Push memerlukan device registration bila dipilih. |
| GAP-09 | support_requests tidak punya contact_email; laporan desain meminta email opsional. | Tentukan field kontak dan validasi, attachment ownership/retensi. |
| GAP-10 | Laporan desain memuat berat per material; deposits belum punya item. | Laporan kategori bergantung pada deposit_items; jangan menyimpulkan kategori dari foto AI sebagai bukti setoran. |
| GAP-11 | account display_name wajib tetapi form hanya fullName; photo_path berupa path, FE perlu gambar yang dapat diakses. | Pilih fallback display_name/aturan edit dan kontrak akses media. Tidak mengirim path lokal server sebagai URL publik. |
| GAP-12 | Artikel dan hasil scan memiliki kebutuhan data yang lebih kaya dari kolom eksplisit. | Tetapkan schema JSON/field atau adapter yang tervalidasi; jangan membekukan format dari contoh UI saja. |
| GAP-13 | rewards.stock nullable, redemptions belum memuat expiry/used/refund/idempotency khusus. | Tentukan unlimited stock, kode voucher, masa berlaku, penggunaan/pembatalan, dan refund; ledger/idempotency harus mendukung aturan tersebut. |
| GAP-14 | leaderboard_rows snapshot tidak menetapkan tie-break/refresh; belum ada ledger XP untuk bonus misi. | Tentukan sumber score dan periodisasi; XP game+bonus misi tidak boleh hilang atau dihitung dua kali. |

## 6. Kondisi backend yang terbukti

- Router saat audit hanya memasang GET /health dan GET /api/v1/auth/me. Endpoint kedua mengembalikan UID/email/email_verified Firebase, tanpa profil aplikasi atau saldo.
- Firebase client sudah diteruskan dari main melalui bootstrap ke middleware. VerifyIDToken digunakan; pemeriksaan revocation/disabled account belum diaktifkan. Otorisasi bisnis dari PostgreSQL belum ada.
- Pool PostgreSQL dibuat dan Ping saat startup, tetapi belum ada repository fitur yang memakainya. Service/domain masih placeholder.
- Satu migration SQL membuat accounts, user_settings, wallets, dua enum, constraint saldo, RLS, dan pencabutan grant. DBML memuat rancangan 33 tabel dan 14 enum; schema remote tidak diaudit ulang.
- Router test menggunakan verifier pengganti. Keberhasilan test tidak membuktikan integrasi Firebase/project token sebenarnya, Android runtime, migration remote, atau transaksi bisnis.
- Config membaca environment proses. Default listener adalah loopback host; kebutuhan koneksi emulator/perangkat dan HTTPS diputuskan saat integrasi. Android mempunyai izin INTERNET/CAMERA; lokasi perangkat belum menjadi input nyata pada source ini.

## 7. Pertanyaan tersisa dan status jawaban

Jawaban yang sudah disampaikan dipindahkan ke keputusan bagian 2. Semua pertanyaan di bawah tetap terbuka sampai ada jawaban atau delegasi keputusan yang dicatat. Tidak harus dijawab sekaligus.

| ID audit / tracker | Pertanyaan | Dampak |
|---|---|---|
| ASK-01 / Q-01 | Adakah tanggal penilaian pasti dan kapasitas tim? Target seluruh fitur sudah dikonfirmasi. | Estimasi jadwal dan pembagian kerja. |
| ASK-02 / Q-04 (TERJAWAB) | Pemilik memilih satu akun satu tempat setelah pertanyaan pemilihan cabang. | Tidak perlu menambah selector cabang; gunakan AUD-D09 terbaru. |
| ASK-03 / Q-04 | Siapa yang melengkapi data/persetujuan mitra baru selama admin ditunda, dan lewat proses apa? Persetujuan berlaku profil tempat milik akun tersebut; siapa yang menjalankannya? | Mitra baru tidak boleh mengakses QR; seeded demo bisa dipakai sesuai status yang ditetapkan. |
| ASK-04 / Q-02/Q-03 | Untuk data dummy: satu tarif atau per kategori/cabang; presisi berat; minimum/maksimum; pembulatan per item/total? | Nilai contoh boleh ditentukan AI, tetapi aturan perhitungannya harus tercatat sebelum diimplementasi. |
| ASK-05 / Q-06 | Lokasi repo/spec model, cara panggil, contoh input/output, kategori, confidence/unknown, batas file, timeout? | Kontrak AI. Pemilik menyatakan spec belum tersedia dan boleh ditunda; tidak mengunci bahasa/protocol model. |
| ASK-06 / Q-07 | Lokasi repo/spec game, replay XP, hadiah misi, distinct category vs jumlah setoran, waktu reset, bonus XP? | Kontrak attempt/misi dan leaderboard. Pemilik meminta aturan fair/anti-exploit; kebijakan jumlah XP dan replay masih perlu rancangan yang eksplisit. |
| ASK-07 / Q-05 | Voucher simulasi dipakai dengan tombol user atau validasi petugas; ada expiry, stok tak terbatas, cancel/refund? | Penukaran dan lifecycle voucher nyata pada simulasi. |
| ASK-08 / Q-16 | QR dapat diklaim pengguna pertama yang memindai atau sudah terikat penerima? | Pencegahan klaim oleh akun yang tidak dimaksudkan. |
| ASK-09 / Q-08 | Setelah QR terbit boleh ubah rincian berat atau harus batalkan/buat transaksi baru? Histori reissue wajib? | State machine deposit/QR dan audit. |
| ASK-10 / Q-11/Q-14 | Timezone periode, awal minggu, tie-break, refresh, rentang laporan? | Rekap bulanan, leaderboard, deadline misi, PDF. |
| ASK-11 / Q-10 | Notifikasi hanya inbox atau push; kedua preference boleh diubah dan terpisah? | Schema setting/device dan proses pengiriman. |
| ASK-12 / Q-12 | Email verification wajib sebelum akses; cara profil belum lengkap; penghapusan/suspensi; nama tampilan? | Akses akun dan mapping registrasi. |
| ASK-13 / Q-13 | Upload lewat Android Storage SDK atau Go; batas jenis/ukuran/jumlah/retensi; email support opsional? | Media privat, support, dan pembersihan file. |
| ASK-14 / Q-09 | Apakah artikel/reuse/soal/reward contoh lokal boleh dijadikan seed awal? Siapa mengubahnya selama admin ditunda? | Konten demo; data contoh bukan aturan produk otomatis. |
| ASK-15 / Q-04 | Favorit harus persisten? Sumber area dan posisi pengguna untuk mitra sekitar? | Filtering/persistensi yang melampaui state UI lokal. |
| ASK-16 / Q-15 | Deployment Go dan akses tim/perangkat, TLS, serta waktu pengerjaan Docker? | Pengujian lintas perangkat/deployment, tidak menahan audit. |

### 7.1 Rancangan awal perlindungan XP

Requirement pemilik adalah sistem fair yang tidak mudah dieksploitasi. Kontrol berikut merupakan usulan teknis untuk dibahas pada kontrak game, bukan bukti bahwa aturan gamifikasi sudah final:

- Server memilih/menyimpan attempt dan memeriksa bahwa question serta choice memang milik attempt tersebut.
- Satu jawaban final per question/attempt; retry mengembalikan hasil yang sama tanpa menambah XP.
- Finalisasi attempt dan pencatatan hadiah harus atomik/idempotent; jangan menerima earnedXp atau correctAnswers dari client sebagai hasil sah.
- Pengulangan game untuk belajar tetap dipisahkan dari kelayakan memperoleh hadiah. Tentukan apakah hadiah hanya pertama kali, skor terbaik, atau batas per periode; nilainya belum dipilih.
- Misi tidak boleh diklaim berulang; event game yang sama tidak menggandakan progres/bonus. XP dipisahkan dari wallet EcoPoints.
- Rate limit membantu membatasi spam, tetapi tidak menggantikan constraint/idempotency dan kebijakan replay. Batas angka/frekuensi belum ditetapkan.

## 8. Urutan implementasi yang diusulkan

Urutan ini belum merupakan izin melanjutkan code generation pada tahap audit. Integrasi Android dikerjakan setelah backend sesuai permintaan pemilik. Kontrak dan fixture dibuat sebelum integrasi agar backend tetap dapat diuji sendiri.

| Batch | Requirement | Task tracker terkait | Hasil yang harus diuji sebelum batch dianggap selesai |
|---|---|---|---|
| PLAN | Tutup kebutuhan kontrak/pertanyaan per fitur; tetapkan pending mitra dan pelengkapan data satu tempat. | DOC-03/04/05, DATA-03, Q-04 | Traceability FE-ID, field, aturan, endpoint usulan, schema, dan acceptance test tersedia. Tidak menunggu semua pertanyaan untuk membaca source lain. |
| BE-01 | Identitas, profil user/mitra pending, settings/wallet awal. | AUTH-01 sampai AUTH-05 | Firebase nyata terpisah dari tes fake; pembuatan profil idempotent/atomik; suspended/pending ditolak sesuai operasi. |
| BE-02 | Mitra satu tempat, material, favorit, seed development. | DATA-04/05 dan backlog mitra | Satu akun satu tempat; listing sesuai status; seed ulang tidak menggandakan data; account UID demo dipetakan tanpa credential di repo. |
| BE-03 | Tarif, estimasi, item setoran, QR, klaim, ledger/wallet. | DATA-03, backlog transaksi | Berat/poin presisi; token 10 menit; expiry/reissue/claim bersamaan; retry aman; authorization per cabang. |
| BE-04 | Beranda, profil statistik, riwayat, reward/voucher. | Backlog wallet/reward/riwayat | Semua angka berasal dari data; saldo/stok/ledger atomik dan voucher nyata dalam simulasi; empty state. |
| BE-05 | Artikel dan hasil scan/media sesuai kontrak teman. | MEDIA-01, backlog scan/artikel | Output model tervalidasi, ownership upload, failure/unknown/timeout; draft artikel tidak terlihat. |
| BE-06 | Games, XP, misi, badge, leaderboard. | Backlog gamifikasi | Penilaian server, replay sesuai aturan, bonus satu kali, periodisasi dan privasi konsisten. |
| BE-07 | Settings, inbox/push jika disetujui, support, laporan PDF/privasi. | Backlog notifikasi/support/laporan | Preferensi berfungsi; report per material benar; PDF dan lampiran dibatasi pemiliknya. |
| INT | Hubungkan Firebase SDK/network layer/state Android dan semua layar/placeholder sesuai desain. | AUTH-06 dan integrasi Android | Skenario end-to-end user/mitra satu tempat, jaringan terputus, retry, logout, QR decoder, reward, AI/game nyata. |
| OPS | Hosting, backup, Docker jika diaktifkan, dokumentasi menjalankan tim. | DEV-01, Q-15 | Dapat dijalankan anggota lain dan diuji pada perangkat target tanpa credential dalam image/source. |

BE-05 dan BE-06 dapat dikembangkan setelah kontraknya tersedia; urutan tidak memaksa ketergantungan palsu antara artikel dan game. Tabel terpisah untuk tiap komponen tampilan, worker/goroutine khusus, atau framework baru tidak dibuat tanpa kebutuhan yang tercatat.

## 9. Aturan melanjutkan pekerjaan dari audit

1. Baca instruksi terbaru, tracker, dan jawaban baru. AUD-D tidak diganti oleh asumsi pada dokumen lama.
2. Cocokkan hash/source terbaru. Jika FE berubah, audit ulang layar/model terkait dan catat delta sebelum menghasilkan kode.
3. Pilih FE-ID dan task tracker yang dikerjakan; tetapkan field, authorization, state gagal, efek database, dan retry dalam kontrak endpoint.
4. Tautkan migration/repository/aturan bisnis/handler ke requirement tersebut. Jangan menandai endpoint usulan sebagai tersedia sebelum router dan implementasi benar-benar ada.
5. Uji perilaku melalui HTTP/aturan bisnis/database yang relevan. Tes fake tidak menggantikan pengujian layanan nyata atau race transaksi.
6. Perbarui checkbox tracker hanya jika syarat selesai terpenuhi; catat perintah, hasil, dan batas pengujian. Placeholder, DTO, dan schema rancangan bukan bukti fitur selesai.
7. Saat integrasi, pertahankan desain dan alur, ganti sumber data/callback secara terukur, serta catat kebutuhan tambahan yang sudah disetujui.

Tidak ada janji bahwa dokumen menghilangkan seluruh kemungkinan kesalahan generasi. Bukti file, traceability, keputusan eksplisit, dan pengujian digunakan untuk mengurangi asumsi dan menemukan kesalahan sebelum integrasi.
