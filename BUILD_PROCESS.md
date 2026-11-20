# EcoSnap Build Process

Status: rencana kerja bersama dan tracker implementasi, versi awal 0.1.
Snapshot awal: 2026-11-03. Audit source FE/BE mendalam dan keputusan terbaru: 2026-11-17; lihat docs/FRONTEND_BACKEND_AUDIT.md.
Cakupan: backend Go, database, integrasi layanan, kontrak dengan Android, pengujian, dan deployment.
Target terbaru pemilik: seluruh fitur, termasuk placeholder, selesai secepatnya. Tanggal deadline pasti dan kapasitas tim belum dikonfirmasi.

Navigasi: [Branch](#14-strategi-branch-dan-baseline-git) | [Progres](#2-ringkasan-progres) | [Keputusan](#4-keputusan-dan-aturan-produk) | [Pertanyaan terbuka](#5-daftar-keputusan-terbuka) | [Struktur](#6-struktur-dan-tanggung-jawab-kode) | [Data](#7-rencana-data) | [Kontrak API](#8-kontrak-http-dan-integrasi-android) | [Checklist task](#9-backlog-dan-syarat-selesai) | [Pengujian](#11-matriks-verifikasi) | [Checkpoint](#13-catatan-verifikasi-dan-pembaruan).

## 1. Mulai dari sini

Dokumen ini menjadi titik masuk untuk melanjutkan pekerjaan EcoSnap. Gunakan daftar task sebagai rencana, keputusan sebagai batas rancangan, dan kode serta hasil pengujian sebagai bukti progres. Dokumen ini perlu diperbarui bersama implementasi.

Urutan penggunaan oleh anggota tim atau AI:

1. Baca bagian 2-5 untuk memahami progres, keputusan, sumber, dan pertanyaan terbuka.
2. Periksa kode terbaru dan perubahan lokal. Cocokkan dengan snapshot; jangan menganggap status dokumen selalu paling baru.
3. Pilih satu task yang dependency-nya terpenuhi dari bagian 9. Catat PIC dan ubah statusnya menjadi BERJALAN.
4. Baca referensi fitur yang diperlukan, termasuk gambar UI jika pekerjaan memengaruhi perilaku Android.
5. Kerjakan perubahan kecil sampai satu hasil dapat diperiksa. Jelaskan tanggung jawab file dan konsekuensi perubahan.
6. Jalankan verifikasi yang sesuai, lalu perbarui task, bukti, dan catatan serah terima.
7. Jika keputusan bisnis belum tersedia, catat pada daftar Q dan lanjutkan pekerjaan lain yang independen.

Instruksi terbaru: pemilik meminta strategi branch dicatat dan pengerjaan dimulai dari baseline Git (bagian 14). Setiap batch dijelaskan terlebih dahulu: apa yang dibuat, lokasi file, dan rasionalisasinya. Gunakan nama file spesifik tanggung jawab, pertahankan desain/alur Android sebagai acuan, dan sertakan tes serta pembaruan progress. Implementasi lama yang sudah diuji disimpan sebagai baseline, bukan dibuat ulang. Tidak ada instruksi push atau merge main pada tahap lokal ini.

Contoh instruksi untuk sesi AI baru:

> Baca BUILD_PROCESS.md dan docs/api/android-integration.md, lalu periksa kode backend dan Android terbaru. Jelaskan batch implementasi beserta alasan dan kontraknya sebelum menulis kode. Jalankan pemeriksaan yang sesuai dan catat hasil aktual. Tandai pertanyaan terbuka tanpa mengarang keputusan bisnis.

## 2. Ringkasan progres

### 2.1 Aturan status dan checkbox

| Status | Arti |
|---|---|
| SELESAI | Hasil task memenuhi syarat selesai yang tertulis dan mempunyai bukti. |
| ADA KODE | Implementasi ditemukan, tetapi verifikasi perilaku yang diperlukan belum lengkap. |
| BERJALAN | Ada PIC yang sedang mengerjakan task tersebut. |
| RENCANA | Belum dikerjakan. |
| MENUNGGU KEPUTUSAN | Ada aturan produk atau pilihan teknis yang perlu diputuskan. |
| DITUNDA | Sengaja belum dikerjakan; alasannya harus dicatat. |

Checkbox [x] hanya digunakan pada task SELESAI. Checkbox [ ] tetap digunakan untuk status lainnya. Checklist terdapat satu kali pada backlog; dashboard ini hanya merangkum. Tidak ada persentase progres aplikasi karena bobot pekerjaan dan scope rilis belum disepakati.

Jenis bukti:

- KODE: file/fungsi atau commit yang dapat diperiksa.
- CEK: perintah dan hasil yang benar-benar dijalankan.
- LAPORAN: hasil yang disampaikan anggota tim, belum diulang pada audit.
- DOKUMEN: sumber atau keputusan yang sudah dicatat.

Keberadaan screenshot, folder, file stub, atau dependency tidak membuktikan fitur backend selesai. Hasil tes dengan label "no test files" juga tidak membuktikan aturan bisnis sudah diuji.

### 2.2 Kondisi saat snapshot

| Area | Kondisi | Bukti dan batasnya |
|---|---|---|
| Dokumentasi sumber | Tersimpan | Ekstraksi PDF 33 halaman; inventaris Figma; 10 PNG gabungan pengguna. |
| Struktur backend | Ada kerangka | Folder Handler, Service, Repository, domain, integration, migration, dan seed tersedia. Sebagian besar masih kosong. |
| Startup dan config | ADA KODE | `cmd/app/main.go`, `internal/config/config.go`, dan `internal/bootstrap/app.go`. |
| Pool PostgreSQL | ADA KODE | `NewPool` membuat pgxpool, melakukan Ping, dan membersihkan pool jika Ping gagal. |
| Supabase | LAPORAN | Pemilik telah membuat project, mengatur connection string, dan menyampaikan setup/koneksi selesai. Kredensial tidak diperiksa atau disalin dalam audit ini. |
| Endpoint HTTP | Ada satu | Hanya `GET /health` terdaftar pada router. Mengembalikan status ok tanpa query database. |
| Graceful shutdown | ADA KODE | Signal context, goroutine server, channel error, Shutdown, dan penutupan pool ditemukan di main.go. |
| Build dan vet | CEK: lolos | `go build -mod=readonly` dan `go vet -mod=readonly ./...` melalui RTK pada snapshot ini. |
| Tests | Belum tersedia | Tidak ditemukan file `*_test.go` pada inventaris kode saat audit. |
| Migration dan tabel aplikasi | Belum ada migration di repo | Direktori migration hanya berisi placeholder. Schema/tabel remote Supabase belum diaudit; jangan menganggap remote kosong. |
| Firebase, AI, dan fitur bisnis | RENCANA | Folder integrasi/service/domain masih placeholder; belum ada handler/repository fitur aplikasi. |
| Git dan artefak | Perlu dirapikan | Banyak perubahan lokal belum di-commit. `app.exe` muncul sebagai untracked; jangan ikut memasukkannya ke source commit. |

### 2.3 Posisi berikutnya

Baseline db51b4b tidak perlu dibuat ulang. Setelah ruang tersedia (awal run sekitar 6,9 GB), full suite PostgreSQL portable PASS tanpa SKIP dan build exit 0. GIT-03 selesai; kegagalan disk pada checkpoint lama tetap menjadi riwayat. Guard ruang minimum 4 GiB tetap dipakai oleh kedua runner.

Perbaikan runner telah digabung secara fast-forward ke dev (937a9b7). Estimator (feat/deposit-estimation, e41b1b1) dan pembuatan/detail setoran (feat/deposit-creation) tersedia dan lulus full suite PostgreSQL lokal tanpa SKIP. Kontrak: docs/api/point-estimates.md, docs/api/deposits.md; bukti: docs/test-results/point-estimates-2026-11-20.md, docs/test-results/deposits-2026-11-20.md. Pembulatan total mengikuti D-17. Langkah berikutnya QR/klaim/ledger menunggu Q-08/Q-16. Tidak ada promosi main, push, atau perubahan database Supabase pada tahap ini.

## 3. Produk, sumber, dan kewenangan keputusan

EcoSnap membantu pengguna mengenali sampah melalui foto, memperoleh panduan penanganan dan pemanfaatan kembali, menemukan mitra penerima, menyetorkan sampah secara fisik, memperoleh EcoPoints melalui klaim QR, dan menukar poin dengan reward. Aplikasi Android menggunakan Kotlin.

Sumber lokal yang dibaca sesuai kebutuhan:

| Sumber | Gunakan ketika | Batas interpretasi |
|---|---|---|
| [Audit source FE/BE terbaru](docs/FRONTEND_BACKEND_AUDIT.md) | Melanjutkan rancangan backend dari layar/model Android dan jawaban 2026-11-17. | Statis; semua endpoint usulan dibedakan dari dua route aktual; pertanyaan terbuka bukan keputusan. |
| [Ekstraksi laporan UTP](documentation/utp-resubmission.extracted.md) | Memeriksa tujuan, use case, flowchart, dan aturan bisnis dari laporan. | Halaman 30-33 masih contoh template aplikasi gudang; bukan implementasi EcoSnap. |
| [Catatan awal PRD](documentation/prd-evidence-notes.md) | Menelusuri kebutuhan awal dan celah antara laporan, UI, dan DBML. | Merupakan catatan historis; beberapa pertanyaan sudah dijawab melalui keputusan bagian 4. |
| [Inventaris Figma](documentation/figma/README.md) | Menghubungkan fitur dengan nama frame dan node ID. | 42 layar utama, 5 frame tambahan, 14 wireframe; export seluruh Figma belum selesai. |
| [Catatan kelengkapan UI](documentation/figma/ui-completeness-notes.md) | Menentukan state, transisi, dan data layar yang perlu diperjelas. | Semua 42 layar utama punya padanan pada 10 gambar; beberapa alur belum dijelaskan lengkap. |
| [Gambar kiriman pengguna](documentation/figma/user-image-manifest.json) | Memeriksa screenshot lokal dan asal gambar. | Angka saldo, berat, tarif, dan skor adalah contoh UI sampai ditetapkan sebagai aturan. |
| [Inventaris node Figma](documentation/figma/node-inventory.json) | Mencocokkan nama frame dengan file Figma yang diperiksa. | Metadata tidak membuktikan prototype interaction atau backend berjalan. |
| [DBML awal dari pemilik](database/schema.dbml) | Meninjau rancangan tabel dan relasi. | Arsip 33 tabel dan 14 enum tersedia; belum disesuaikan dengan seluruh keputusan terbaru. Metadata schema remote masih perlu dicatat pada DATA-01. |
| Kode dan pemeriksaan lokal | Memastikan apa yang sudah diimplementasikan. | Kode menunjukkan perilaku aktual, bukan persetujuan atas perubahan aturan produk. |

Urutan penyelesaian konflik:

1. Instruksi terbaru dan keputusan eksplisit pemilik/tim mengarahkan requirement.
2. Keputusan yang sudah disepakati dicatat di bagian 4; perubahan selanjutnya mempunyai alasan dan tanggal.
3. PDF, DBML awal, dan Figma menjadi bukti sumber, dengan perbedaan dicatat.
4. Ketidaksesuaian kode dengan requirement menjadi task perbaikan, bukan alasan mengubah requirement diam-diam.

Seluruh cakupan yang terlihat di laporan/UI tetap dicatat. Urutan pengerjaan di dokumen ini merupakan usulan; status wajib rilis dan ketentuan penilaian menunggu Q-01. Belum ada dokumen PRD final yang disepakati.

## 4. Keputusan dan aturan produk

### 4.1 Keputusan pemilik

| ID | Keputusan | Status | Konsekuensi |
|---|---|---|---|
| D-01 | Backend menggunakan Go; Android menggunakan Kotlin. | DISEPAKATI | Pertahankan pemisahan backend dan aplikasi Android. |
| D-02 | Data aplikasi menggunakan PostgreSQL yang di-host di Supabase. | DISEPAKATI | Supabase menyediakan database; backend Go mengaksesnya melalui driver PostgreSQL. |
| D-03 | Firebase Auth untuk identitas/login; Firebase Storage untuk media. | DISEPAKATI | Pemakaian PostgreSQL Supabase tidak otomatis memindahkan login atau media ke Supabase. |
| D-04 | Satu transaksi setoran bisa berisi banyak jenis sampah. | DISEPAKATI | Perlu rincian item per setoran; satu berat dan tarif pada header saja tidak mencukupi. |
| D-05 | QR berlaku 10 menit; tepat saat kedaluwarsa klaim ditolak. | DISEPAKATI | Gunakan waktu server untuk validasi, bukan countdown perangkat. |
| D-06 | QR yang dibuat ulang mendapat masa berlaku 10 menit dari penerbitan ulang. | DISEPAKATI | QR sebelumnya dinonaktifkan; transaksi yang sudah diklaim tidak diterbitkan ulang. |
| D-07 | Peran admin diperlukan, rancangan/cakupannya dibahas dengan tim. | DITUNDA | Admin tetap tercatat sebagai kebutuhan. Mekanisme verifikasi mitra dan pengelolaan konten belum final. |
| D-08 | Reward menggunakan simulasi voucher, dengan alur saldo/stok/penerbitan/kepemilikan yang benar-benar bekerja. | DISEPAKATI 2026-11-17 | Integrasi pencairan DANA tidak termasuk. Detail penggunaan/expiry/refund tetap Q-05. |
| D-09 | Model AI sedang dikerjakan teman pemilik. | LAPORAN 2026-11-17 | Artefak, protocol, dan contoh input/output belum diterima; Q-06 belum selesai. |
| D-10 | Game sedang dikerjakan teman pemilik; aturan XP belum tersedia dan diminta fair/anti-exploit. Handoff AI/game boleh ditunda sementara. | LAPORAN DAN ARAHAN 2026-11-17 | Fitur tetap scope; kontrol anti-duplikasi dicatat pada audit bagian 7.1. Jumlah hadiah/batas replay belum dipilih; Q-06/Q-07 tetap terbuka. |
| D-11 | Pemilik semula mengetik kode sendiri; kemudian mengizinkan AI mengimplementasikan setelah penjelasan. Setelah audit, pemilik mengizinkan batch akun dengan nama file spesifik. | INSTRUKSI TERBARU 2026-11-17 | Jelaskan apa/lokasi/rasional sebelum setiap batch; gunakan audit sebagai acuan implementasi. |
| D-12 | Target terbaru seluruh fitur selesai secepatnya, termasuk placeholder. | DISEPAKATI 2026-11-17 | Perkiraan lama 1-2 bulan merupakan riwayat. Tanggal pasti dan kapasitas tim belum tersedia; jangan menjanjikan jadwal. |
| D-13 | EcoSnap-FE lokal menjadi acuan final; desain/alur dipertahankan, kebutuhan integrasi dicatat; integrasi setelah backend. | DISEPAKATI 2026-11-17 | Lihat audit FE-01 sampai FE-26; jangan menghapus placeholder dari backlog. |
| D-14 | Mitra demo memakai seed; keputusan terakhir satu akun mengelola satu tempat, menggantikan jawaban beberapa cabang. | DISEPAKATI 2026-11-17 | Pertahankan unique partners.mitra_uid. Tidak perlu schema multi-cabang atau pemilihan cabang pada UI. |
| D-15 | Pendaftaran mitra diperbolehkan, QR dilarang sampai data lengkap dan disetujui. | DISEPAKATI 2026-11-17 | Role mitra bukan izin otomatis bertransaksi. Alur pelengkapan/persetujuan masih Q-04. |
| D-16 | Nilai tarif awal boleh memakai contoh yang nanti diubah tim. | DISEPAKATI 2026-11-17 | Seed deterministik data/seeders/dev/point_rates.json: plastic 100, paper 80, metal 150, glass 60 poin/kg. Mekanisme demo mengikuti docs/api/point-estimates.md; tarif usaha final/pengelolaan masih Q-02. |
| D-17 | Estimasi menjumlahkan hasil semua item terlebih dahulu, lalu membulatkan ke bawah sekali pada total poin. | DISEPAKATI pemilik | Contoh 0,6 + 0,6 menghasilkan 1 poin. Jangan membulatkan per item. Tarif tetap data contoh editable sesuai D-16. |
| D-18 | QR setoran bersifat bearer: siapa pun yang mengklaim pertama kali mendapatkan poin (menjawab Q-16). | DISEPAKATI pemilik 2026-11-20 | Penerbitan QR mengembalikan warning agar mitra hanya menunjukkan QR kepada penyetor. Klaim satu kali, atomik dengan ledger dan wallet. Hanya akun role user active yang dapat mengklaim. |
| D-19 | Setelah QR terbit, setoran terkunci: tidak dapat diedit atau dibatalkan (menjawab bagian Q-08). Reissue tetap boleh selama belum diklaim. | DISEPAKATI pemilik 2026-11-20 | Koreksi/pengembalian poin hanya melalui sistem banding di masa depan (Q-17). Penolakan fisik dilakukan sebelum setoran dibuat. |

D-05 menyelesaikan pertanyaan countdown 04:59 pada mockup. D-04 menyelesaikan pertanyaan jumlah jenis sampah per setoran, tetapi perubahan schema masih perlu dibuat. D-06 menetapkan reset masa berlaku; model riwayat token belum dipilih.

### 4.2 Aturan dari sumber yang dipertahankan dalam rencana

| ID | Aturan | Sumber |
|---|---|---|
| BR-01 | Mitra memeriksa material dan berat aktual secara fisik. Hasil AI menjadi referensi yang dapat dikoreksi. | PDF halaman 4 dan 6. |
| BR-02 | Estimasi poin pengguna belum menambah saldo. Poin setoran diberikan setelah klaim transaksi yang sah. | PDF halaman 6-10. |
| BR-03 | QR membawa referensi/token transaksi; jumlah poin dan penerima tidak dipercaya langsung dari payload client. | PDF halaman 8-9. |
| BR-04 | Preview transaksi tidak memakai QR atau mengubah saldo. Batal konfirmasi juga tidak mengubah saldo. | PDF halaman 8-9. |
| BR-05 | Saat klaim, validasi ulang dilakukan; satu setoran hanya mempunyai satu penerima klaim yang berhasil. | PDF halaman 9-10. |
| BR-06 | Penyelesaian setoran, pemakaian QR, pencatatan ledger, dan perubahan wallet dilakukan dalam satu transaksi database. | PDF halaman 9; kebutuhan konsistensi DBML. |
| BR-07 | Hasil request yang belum diketahui diselesaikan lewat pemeriksaan status/idempotency. Timeout client tidak membuktikan transaksi database gagal. | PDF halaman 9-10. |
| BR-08 | Penukaran reward memeriksa ulang saldo, harga/status/ketersediaan reward, dan identitas transaksi di server. | PDF halaman 13-14. |
| BR-09 | Pengurangan poin dan pencatatan redemption harus atomik dan tidak terulang akibat retry. | PDF halaman 13-14. |
| BR-10 | XP mini games terpisah dari EcoPoints. XP tidak menjadi saldo penukaran reward. Sumber bonus misi menunggu Q-07. | PDF halaman 6; UI Games. |
| BR-11 | Penolakan izin lokasi menyediakan pencarian lokasi manual; hasil kosong menyediakan perubahan filter atau area. | PDF halaman 11-12 dan 27-28. |
| BR-12 | Riwayat dan laporan menggunakan transaksi yang benar-benar tersimpan; contoh angka UI tidak menjadi data produksi. | PDF halaman 27-29; gambar pengguna 8-9. |

BR dalam file ini adalah ID lokal BUILD_PROCESS.md. Catatan awal PRD mempunyai penomoran historis sendiri; gunakan nama file ketika merujuk ID dari dokumen lain.

## 5. Daftar keputusan terbuka

Dependency task merujuk ID Q berikut. Anggota tim boleh melanjutkan bagian yang independen sambil menunggu jawaban.

| ID | Pertanyaan yang perlu dijawab | PIC keputusan | Menahan pekerjaan |
|---|---|---|---|
| Q-01 | Scope seluruh fitur/placeholder sudah dikonfirmasi. Tanggal deadline pasti, rubrik, dan pembagian kapasitas tim belum tersedia. | Pemilik + tim/asprak | Estimasi jadwal; tidak lagi mempertanyakan apakah placeholder masuk scope. |
| Q-02 | Tarif usaha final dan siapa yang mengelolanya. Implementasi demo memakai tarif per kategori, override mitra, fallback global, versi aktif terbaru (migration 00005). | Pemilik + tim | Persetujuan tarif produksi/admin; estimator demo sudah dapat diuji. |
| Q-03 | Pembulatan total floor sudah dipilih pemilik (D-17). Implementasi demo memakai gram, total 0.001-1000 kg, 1-50 kategori unik dan penolakan kategori ganda. Batas bisnis final belum disepakati. | Pemilik + tim | Validasi batas saat demo tim dan reuse perhitungan pada setoran nyata. |
| Q-04 | Seed mitra demo, pendaftaran pending tanpa QR, dan keputusan terakhir satu akun satu tempat sudah dikonfirmasi. Siapa melengkapi/menyetujui data serta mekanisme manual selama admin ditunda masih terbuka. | Pemilik + tim | Onboarding lengkap dan persetujuan mitra; struktur multi-cabang tidak diperlukan. |
| Q-05 | Voucher simulasi dengan sistem nyata sudah dikonfirmasi. Cara pakai, stok/unlimited, expiry, cancel, dan refund masih terbuka. | Pemilik + tim | Lifecycle redemption lengkap. |
| Q-06 | Model/provider AI, kategori didukung, lokasi inference, confidence/hasil tidak dikenali, biaya, timeout, dan mode sync/async. | PIC foto sampah + tim | Fitur identifikasi nyata dan kebutuhan worker. |
| Q-07 | Jenis misi, XP/bonus poin/badge, syarat selesai/klaim, replay game, batas farming, reset periode, dan soal Eco Quiz. | PIC gamifikasi + tim | Games/misi/leaderboard final. |
| Q-08 | Sebagian dijawab D-19 (terkunci setelah QR terbit; histori semua penerbitan QR disimpan). Sisa: kebijakan setoran created yang tidak pernah diberi QR. | PIC transaksi + tim | Pembersihan setoran terbengkalai. |
| Q-09 | Siapa menyediakan/memublikasikan artikel, ide reuse, kategori, soal, dan reward selama admin ditunda. | Pemilik + tim | Konten demo yang dapat dipertanggungjawabkan. |
| Q-10 | Notifikasi dalam aplikasi atau push; preferensi terpisah; proses pengiriman dan retensi. | Tim Android + backend | Firebase messaging/worker notifikasi; jangan menganggapnya otomatis termasuk Firebase Auth. |
| Q-11 | Periode, timezone, tie-break peringkat, visibilitas, dan kapan leaderboard diperbarui. | Tim | Perhitungan leaderboard. |
| Q-12 | Onboarding sebelum login, profil belum lengkap, email verification, pergantian peran, dan penghapusan/suspensi akun. | Tim Android + backend | Alur akun dan kebijakan akses lengkap. |
| Q-13 | Lampiran media: ukuran/jenis/jumlah, retensi dan penghapusan; dukungan email kontak opsional; penanganan laporan/masukan. | Tim | Kontrak upload dan support lengkap. |
| Q-14 | Rentang laporan, definisi metrik, timezone, isi PDF, dan apakah pembuatan PDF perlu background job. | Tim | Laporan final. |
| Q-15 | Hosting Go, akses jaringan, domain/TLS, backup/restore, dan kelayakan VPS bekas penelitian. | Pemilik + tim | Deployment publik. Docker bersifat opsional. |
| Q-16 | Dijawab D-18: QR bearer, pengklaim pertama. | Pemilik | - |
| Q-17 | Sistem banding: siapa mengajukan, siapa memutuskan, dan bagaimana poin yang sudah dikreditkan dikoreksi (ledger debit, bukan menghapus kredit). | Pemilik + tim | Koreksi setoran setelah QR terbit/klaim. |

Usulan yang dapat disiapkan tanpa mengunci jawaban: data contoh tarif/soal/reward diberi label development, kontrak provider AI dibuat setelah bentuk output ditentukan, dan akun mitra demo dapat disiapkan melalui proses seed yang terdokumentasi. Akun biasa yang mendaftar tetap tidak boleh otomatis memperoleh hak mitra terverifikasi.

## 6. Struktur dan tanggung jawab kode

### 6.1 Struktur acuan

Pertahankan pola Handler, Service, Repository yang sudah dipahami pemilik. Nama file fitur di masa depan mengikuti kebutuhan fitur; jumlah tabel tidak menentukan jumlah package atau interface.

~~~text
cmd/
  app/main.go                     Startup dan penghentian proses HTTP
  seed/                           Command data awal, ketika dibutuhkan
  worker/                         Dicadangkan; belum ada kebutuhan final
internal/
  config/config.go                Membaca dan memvalidasi konfigurasi
  bootstrap/app.go                Merakit dependency dan server
  delivery/http/
    router.go                     Route dan pemasangan middleware
    handler/                      Parsing request dan response HTTP
    middleware/                   Auth, logging, recovery, dan pemeriksaan bersama
  service/                        Aturan bisnis dan orchestration transaksi
  repository/postgres/
    connection.go                 Pool koneksi; file fitur berisi akses data
  integration/
    firebase/                     Firebase Admin/Auth dan akses media
    ai/                           Adapter provider identifikasi
domain/
  entity/                         Data inti aplikasi
  dto/                            Input/output fitur yang diperlukan
  contracts/                      Interface yang punya kebutuhan nyata
  enums/                          Nilai status/jenis yang diperlukan
  errx/                           Error aplikasi yang dapat dipetakan ke HTTP
database/migrations/              Migration schema berversi
data/seeders/dev/                 Data contoh development
data/seeders/prod/                Data awal produksi bila memang diperlukan
docs/api/                        Kontrak HTTP dan contoh integrasi Android
documentation/                   Bukti sumber produk, PDF, dan UI
test/                            Skenario lintas modul/integrasi bila diperlukan
~~~

Ini adalah peta tanggung jawab, bukan instruksi mengisi seluruh folder sekaligus. Saat snapshot, hanya file Go yang dicantumkan pada bukti fondasi sudah berisi implementasi.

Folder kosong lama di bawah `internal/delivery/bootstrap`, `internal/delivery/service`, `internal/delivery/repository`, `internal/delivery/integration`, dan `internal/delivery/platform` tidak dipakai kode aktif. Penataan berubah dari scaffold awal; alasan penulis awal tidak terdokumentasi. Perapian dilakukan melalui FND-09 setelah memastikan isinya tetap kosong/tidak dipakai. Dua folder bootstrap tidak diwajibkan oleh Go.

### 6.2 Alur dan batas tanggung jawab

~~~text
Startup:
main membaca config
main/bootstrap menyiapkan pool dan integrasi
bootstrap merakit repository, service, handler, middleware, dan router
main menjalankan server serta mengelola shutdown

Request:
Android -> router/middleware -> handler -> service -> repository -> PostgreSQL
                                             -> integration -> Firebase/AI
Hasil atau error diteruskan kembali menjadi response HTTP.
~~~

Aturan penempatan:

- Handler mengurus HTTP, input, dan response. Identitas pengguna diperoleh dari context hasil verifikasi middleware.
- Service menjalankan aturan bisnis dan mengatur satu operasi yang atomik. Perhitungan poin/otorisasi objek tidak ditaruh dalam router.
- Repository mengurus SQL, pemetaan hasil, dan akses transaksi; tidak menulis response HTTP.
- Integration menyembunyikan detail layanan eksternal yang digunakan fitur.
- Bootstrap menyambungkan dependency. Snapshot masih membuka pool di main; saat repository fitur pertama dibuat, pool itu diteruskan ke dependency yang memerlukannya tanpa membuat pool baru per request.
- Domain tidak bergantung pada router HTTP atau driver PostgreSQL. Entity tidak otomatis menjadi payload response; field internal dipilih secara eksplisit.
- Interface mengikuti kebutuhan pemanggil dan pengujian. Hindari interface besar untuk seluruh aplikasi atau interface satu-per-struct tanpa tujuan.
- Pool dan client eksternal dipakai ulang sesuai lifecycle aplikasi. Pemilik resource bertanggung jawab menutupnya.

### 6.3 Context, concurrency, dan pool

- Signal context mengatur lifecycle program; request context mengatur satu request. Penutupan browser tidak mematikan aplikasi Go.
- Goroutine pada startup menjalankan ListenAndServe agar fungsi utama dapat menunggu sinyal maupun hasil server.
- Channel serverErr membawa hasil ListenAndServe ketika method itu selesai; bukan seluruh error request.
- Context query memakai batas waktu yang sesuai. Context yang dibatalkan tidak secara otomatis membuktikan transaksi belum commit.
- Pinjaman manual melalui Acquire dikembalikan dengan Release; Rows ditutup, QueryRow dipanggil Scan, dan transaksi diakhiri Commit/Rollback.
- Pool.Close dipanggil saat shutdown, bukan setiap request. Penutupan pool dapat menunggu koneksi yang belum dikembalikan.
- Timeout Shutdown 10 detik pada snapshot hanya membatasi penghentian HTTP. Ini belum merupakan batas total waktu keluar proses jika resource lain tertahan.
- MaxConns, minimum/idle/lifetime, jumlah instance aplikasi, dan batas koneksi Supabase harus dinilai bersama. Tetapkan anggaran awal melalui FND-07; angka contoh bukan standar universal.
- Request klaim/redeem menunggu hasil transaksi yang dapat dipertanggungjawabkan. Pekerjaan penting tidak dilepas menjadi goroutine tanpa lifecycle, status, dan penanganan retry.
- Worker/queue baru dibuat jika kebutuhan AI, PDF, atau notifikasi yang sudah diputuskan memerlukannya.

## 7. Rencana data

### 7.1 Inventaris DBML awal

Daftar berikut mencakup 33 tabel dari DBML awal yang sudah diarsipkan di database/schema.dbml. Seluruhnya merupakan rancangan, bukan bukti sudah ada di Supabase. Lengkapi penandaan versi awal dan pemeriksaan schema remote melalui DATA-01 sebelum menghasilkan migration dari sumber tersebut.

| Area | Tabel awal | Catatan |
|---|---|---|
| Akun | accounts, user_settings | Firebase UID menghubungkan identitas dengan data aplikasi. |
| Mitra dan material | partners, partner_hours, waste_categories, partner_accepted_waste, partner_favorites | Sumber data, verifikasi mitra, serta kepemilikan lokasi menunggu Q-04. |
| Scan dan reuse | scans, reuse_ideas, scan_reuse_ideas | Scan AI tidak otomatis memberikan EcoPoints. |
| Konten | articles | Publikasi dan pengelola konten menunggu Q-09. |
| Setoran dan QR | point_rates, deposits, qr_claims | Perlu perubahan untuk multi-item dan penerbitan ulang token. |
| Saldo | wallets | Saldo materialized harus konsisten dengan point_ledger. |
| Reward dan ledger | reward_categories, rewards, redemptions, point_ledger | Reward sementara berupa simulasi. |
| Games | games, game_questions, game_choices, game_attempts, game_answers | Penilaian dan replay menunggu Q-07. |
| Misi dan badge | badges, missions, mission_progress, user_badges | Jenis misi kategori berbeda belum ada di enum awal. |
| Peringkat | leaderboard_periods, leaderboard_rows | Skor setoran dan XP dipisahkan. |
| Notifikasi dan dukungan | notifications, support_requests | Channel, preferensi, dan email kontak belum final. |
| Laporan | activity_reports | Detail metrik dan pembuatan PDF menunggu Q-14. |

### 7.2 Perubahan yang perlu dirancang

1. **Item setoran.** Tambahkan rancangan rincian setoran, misalnya tabel deposit_items: relasi ke header, kategori/material, berat, tarif snapshot, dan poin item sesuai pembulatan final. Nama dan field final ditetapkan pada DATA-03.
2. **Tarif.** Hubungkan tarif ke kategori/material dan cakupan mitra sesuai Q-02. Definisikan pemilihan tarif aktif, fallback, overlap periode, dan kondisi tidak ada tarif.
3. **Snapshot.** Tarif/biaya yang sudah digunakan transaksi disimpan agar perubahan katalog tidak mengubah nilai historis. Definisikan total header dan rekonsiliasinya dengan item.
4. **QR.** DBML awal memberi unique pada deposit_id di qr_claims. Pilih update token aktif atau relasi histori penerbitan. Keduanya harus menjamin token lama ditolak, satu token aktif, serta reset expires_at dari waktu penerbitan ulang.
5. **Status setoran.** Bedakan status transaksi, validitas token, pembatalan preview oleh pengguna, penolakan fisik, dan kedaluwarsa. QR kedaluwarsa belum tentu membuat setoran selamanya tidak bisa diterbitkan ulang; transisi final menunggu Q-08.
6. **Admin.** DBML awal hanya mempunyai role user dan mitra. Role/otorisasi admin tidak boleh dianggap sudah ada.
7. **Akun dan pengaturan.** Selaraskan status akun, onboarding sebelum login, preferensi notifikasi, dan privasi leaderboard dengan Q-10 sampai Q-12.
8. **Support.** Tentukan penyimpanan email kontak opsional dan lampiran sesuai Q-13.
9. **Gamifikasi.** Definisikan penyimpanan XP dan sumber peristiwa progres, klaim misi/badge satu kali, serta target kategori berbeda sesuai Q-07.
10. **Penghapusan.** Tentukan soft delete/retensi dan perilaku foreign key. Riwayat ledger tidak boleh hilang hanya karena penghapusan profil atau katalog.

### 7.3 Invariant yang harus dijaga

- Nilai berat dan tarif valid; perhitungan memakai representasi decimal atau unit integer yang disepakati, bukan pembulatan float yang tidak terkendali.
- Wallet tidak menjadi negatif akibat redemption. Ketersediaan stok terbatas tidak boleh menjadi negatif.
- Satu peristiwa pemberian/pemotongan poin memiliki satu hasil ledger yang sah, dilindungi constraint/idempotency key.
- Jenis sumber ledger sesuai referensinya: deposit, redemption, bonus misi, atau adjustment. Aturan adjustment dan otorisasinya perlu ditentukan.
- Saldo akhir dan total earned/spent konsisten dengan ledger, termasuk pembatalan/pengembalian bila disetujui.
- Kepemilikan akun, mitra, scan, voucher, dan laporan diverifikasi di server.
- Race antara klaim, reissue, expiry, dan pembatalan diselesaikan dalam transaksi dengan lock/conditional update yang tepat.
- Request berulang tidak membuat wallet/settings/profil, pemberian XP, atau voucher menjadi duplikat.
- Constraint, index, dan test menegakkan invariant yang sesuai. Validasi frontend saja tidak cukup.

## 8. Kontrak HTTP dan integrasi Android

### 8.1 Konvensi yang perlu difinalisasi

Kontrak final ditulis di docs/api sebelum frontend bergantung pada endpoint baru. Konvensi berikut adalah target rancangan, belum seluruhnya diterapkan:

- Tetapkan prefix versi, format response, format error, nama field, pagination, filter, sort, dan batas ukuran request.
- Gunakan UID dari ID token Firebase yang terverifikasi. Role/status serta kepemilikan objek diperiksa pada operasi yang relevan.
- Bedakan 400 input tidak valid, 401 identitas tidak valid, 403 akses ditolak, 404 tidak ditemukan, 409 konflik state, dan 500 kegagalan internal. Kode tepat per skenario ditetapkan dalam kontrak.
- Timestamp membawa timezone yang jelas; durasi QR memakai waktu server. Aturan periode bisnis menunggu Q-11/Q-14.
- Tentukan serialisasi decimal berat/tarif dan integer poin bersama Android.
- List endpoint mempunyai batas hasil. Query pencarian/filter dan validasi lokasi mempunyai aturan eksplisit.
- Retry mutasi sensitif memakai idempotency atau identitas bisnis yang stabil; status hasil dapat diambil ulang.
- Response tidak memuat password, service credential, connection string, stack trace, atau jawaban benar game sebelum waktunya.
- Setiap endpoint mencatat pihak yang berhak, input, output, error, efek data, dan skenario retry.
- CORS hanya dipasang jika ada client browser yang membutuhkannya; aplikasi Android native tidak memerlukan CORS untuk mengakses backend.

### 8.2 Peta endpoint usulan

GET /health dan GET /api/v1/auth/me sudah diimplementasikan. Route lainnya di bawah masih usulan dan belum dibekukan. Path, metode, dan kebutuhan endpoint dapat disesuaikan ketika kontrak ditulis.

| Area | Endpoint atau operasi usulan | Akses/keterangan |
|---|---|---|
| Liveness | GET /health | Sudah ada; hanya respons proses HTTP. |
| Identitas Firebase | GET /api/v1/auth/me | Sudah ada sejak 2026-11-17; ID token wajib, UID/email berasal dari token. Tidak membaca atau membuat akun PostgreSQL. Kontrak: docs/api/android-integration.md. |
| Readiness | GET /ready | Usulan; pemeriksaan dependency dengan timeout jika diperlukan deployment. |
| Akun | POST /accounts, GET /me, PATCH /me | Identitas Firebase; operasi pembuatan profil harus idempotent. |
| Pengaturan | GET/PATCH /me/settings | Pengguna sendiri; bentuk preferensi menunggu keputusan. |
| Mitra | GET /partners, GET /partners/{id} | Detail, area/radius, kategori diterima; kebijakan akses publik diputuskan. |
| Favorit | GET /me/favorites, PUT/DELETE /me/favorites/{partnerID} | Pengguna sendiri; pengulangan aman. |
| Estimasi | POST /point-estimates | Simulasi tarif/item; tidak menambah saldo. |
| Setoran | POST /deposits, GET /deposits/{id}, GET /partner/deposits | Mitra terkait; multi-item dan aturan status. |
| Penerbitan QR | POST /deposits/{id}/qr, POST /deposits/{id}/qr/reissue | Mitra pemilik; terikat state transaksi. |
| Preview dan klaim | POST /claims/preview, POST /claims | Token QR; klaim memerlukan identitas pengguna. |
| Status klaim | GET status klaim/setoran yang diotorisasi | Detail path ditetapkan dengan kebutuhan retry; hindari membocorkan token melalui URL/log. |
| Wallet | GET /me/wallet, GET /me/point-ledger | Pengguna sendiri; pagination dan filter waktu. |
| Reward | GET /rewards, GET /rewards/{id}, POST /redemptions | Katalog dan penukaran simulasi. |
| Koleksi voucher | GET /me/redemptions, GET /me/redemptions/{id} | Pengguna sendiri; aksi pemakaian menunggu Q-05. |
| Scan | POST /scans, GET /me/scans, GET /scans/{id} | Metadata upload, hasil/status; sync/async menunggu Q-06. |
| Koreksi dan reuse | Koreksi kategori scan; GET ide reuse terkait | Kepemilikan scan dan aturan koreksi ditentukan. |
| Artikel | GET /articles, GET /articles/{id} | Hanya konten yang boleh dipublikasikan. |
| Games | GET daftar/detail; buat attempt; kirim jawaban; ambil hasil | Penilaian server dan idempotency jawaban. |
| Misi dan badge | Ambil progres; klaim hadiah; daftar pencapaian | Lifecycle menunggu Q-07. |
| Leaderboard | GET /leaderboards | Jenis/periode, rank pengguna, privasi, tie-break. |
| Riwayat/beranda | GET /me/activities dan ringkasan beranda | Definisi metrik; gabungan sumber tanpa menggandakan poin. |
| Notifikasi | GET /me/notifications, tandai dibaca | Push merupakan keputusan tambahan. |
| Support | POST /support-requests | Laporan masalah/masukan dan lampiran tervalidasi. |
| Laporan | Buat/ambil laporan aktivitas dan unduh PDF | Kepemilikan, rentang, metrik, dan mode proses. |
| Admin | Operasi verifikasi, katalog, konten, dan tarif | Ditunda; path/role belum ditetapkan. |

Firebase menangani registrasi/login/password reset sesuai alur SDK yang dipilih tim Android. Backend mengurus verifikasi identitas dan profil aplikasi; tidak membuat sistem password kedua hanya untuk meniru layar login.

## 9. Backlog dan syarat selesai

Seluruh PIC belum ditetapkan untuk implementasi berjalan. Pembagian fitur pada PDF adalah referensi historis dan perlu dikonfirmasi tim, bukan penugasan otomatis oleh dokumen ini.

### M0. Sumber dan perencanaan

Dependency: tidak ada. Hasil: anggota tim dapat memahami posisi proyek dan memilih task berikutnya.

- [x] DOC-01 | SELESAI | Simpan referensi laporan dan UI. Bukti: file pada bagian 3 tersedia; batas export Figma dicatat.
- [x] DOC-02 | SELESAI | Buat rencana/tracker bersama ini berdasarkan audit lokal dan keputusan pemilik. Bukti: BUILD_PROCESS.md; verifikasi awal pada bagian 13.
- [ ] DOC-03 | MENUNGGU KEPUTUSAN | Konfirmasi Q-01, PIC fitur, scope wajib dan scope lanjutan. Selesai jika daftar fitur rilis dan penanggung jawab dicatat.
- [ ] DOC-04 | BERJALAN | Audit source tersedia di docs/FRONTEND_BACKEND_AUDIT.md; PRD final belum disepakati.  Susun PRD final bertahap dari sumber dan keputusan; rujuk backlog tanpa menggandakan tracker. Selesai jika user story, acceptance criteria, state gagal, dan batas scope fitur rilis disepakati.
- [ ] DOC-05 | BERJALAN | Kontrak dua route aktual ada di docs/api/android-integration.md; kebutuhan per fitur tercatat pada audit; kontrak seluruh fitur belum dibekukan.  Tetapkan satu sumber schema dan satu kontrak API berversi. Selesai jika anggota tim/AI dapat menemukan keduanya dari dokumen ini.

- [x] AUDIT-01 | SELESAI | Audit statis source FE/BE dan jawaban saat ini didokumentasikan pada docs/FRONTEND_BACKEND_AUDIT.md; inventaris 167 file pada docs/audit/source-inventory.json. Source aplikasi tidak berubah. Tes Go berbasis verifier pengganti lolos; runtime Android/layanan remote belum diperiksa. Pertanyaan yang belum dijawab tetap terbuka, bukan syarat agar pemeriksaan source dapat dicatat.
- [ ] AUDIT-02 | MENUNGGU KEPUTUSAN | Lengkapi handoff AI/game, alur pelengkapan/persetujuan mitra, formula tarif, dan lifecycle voucher sesuai ASK pada audit; perbarui kontrak sebelum batch fitur terkait.

- [x] GIT-01 | SELESAI | Baseline lokal db51b4b pada chore/backend-baseline menyimpan 145 perubahan file source/migration/tes/dokumen/bukti desain. Binary lokal diabaikan; pola credential yang diperiksa tidak ditemukan. Tes ulang gagal karena disk penuh dan tercatat pada GIT-03; bukan klaim release/test PASS.
- [x] GIT-02 | SELESAI | dev dibuat dari baseline db51b4b dan menjadi branch aktif; catatan penyelesaian disimpan melalui commit dokumentasi di dev. main tetap 071320d; tidak ada push atau merge main. Working tree diperiksa setelah commit dokumentasi.

- [x] GIT-03 | SELESAI | Run ulang 2026-11-20: seluruh suite PostgreSQL portable exit 0 tanpa SKIP; go build -p 1 -mod=readonly ./... exit 0. Cluster run yang berhasil dihentikan/dibersihkan. Direktori dari kegagalan lama tetap dipertahankan sebagaimana catatan 14.4. Ini gate melanjutkan fitur, bukan promosi main.

- [x] GIT-04 | SELESAI | Guard ruang untuk kedua runner tersedia pada chore/test-runner-preflight. Helper menguji kondisi cukup/kurang; runner portable/Docker pada ruang aktual menolak sebelum startup tanpa perubahan environment/cluster. Ini tidak menyelesaikan GIT-03 atau menggantikan full test.

### M1. Fondasi aplikasi

Dependency: kode scaffold yang sudah ada. Hasil: aplikasi dapat dimulai, didiagnosis, dan dihentikan dengan perilaku yang diketahui.

- [x] FND-01 | SELESAI | Implementasikan pembacaan HTTP_ADDR/default serta DATABASE_URL wajib. Bukti KODE: internal/config/config.go; kompilasi lolos. Ini hanya task implementasi; uji perilaku ada pada FND-05.
- [x] FND-02 | SELESAI | Implementasikan server/router dan GET /health. Bukti KODE: bootstrap/app.go, router.go, handler/health.go; kompilasi lolos.
- [x] FND-03 | SELESAI | Implementasikan NewPool dengan Ping, timeout startup, dan cleanup saat gagal. Bukti KODE: repository/postgres/connection.go; kompilasi lolos.
- [x] FND-04 | SELESAI | Pasang pemeriksaan error pool sebelum defer, signal context, goroutine/channel server, Shutdown, dan penutupan pool. Bukti KODE: cmd/app/main.go; build/vet lolos. Uji shutdown ada pada FND-06.
- [x] FND-05 | SELESAI | PIC: pemilik repo. Verifikasi startup: DATABASE_URL kosong, koneksi gagal, port bentrok, koneksi valid, /health, serta route tidak dikenal. Bukti LAPORAN: pemilik menyatakan rangkaian langkah yang diberikan selesai dan berhasil pada development lokal/Supabase. Output rinci tidak disalin dan pemeriksaan belum diulang AI; kredensial tetap lokal.
- [ ] FND-06 | ADA KODE | Verifikasi Ctrl+C/SIGTERM, request aktif, shutdown timeout, dan cleanup. Selesai jika urutan shutdown diamati dan batas HTTP vs total proses dijelaskan; gunakan fixture development yang tidak meninggalkan route percobaan.
- [ ] FND-07 | RENCANA | Tetapkan konfigurasi pool dan deadline query awal sesuai kuota Supabase serta jumlah proses/anggota tim. Selesai jika ada alasan batas, resource dilepas di semua jalur, dan uji pool kehabisan koneksi yang dapat dipulihkan.
- [ ] FND-08 | ADA KODE | Format JSON data/error diterapkan pada endpoint identitas dan error auth sejak 2026-11-17, dengan tes HTTP lolos. Validasi body, request ID/logging, recovery panic, dan batas body belum dikerjakan. Route/method error bawaan ServeMux belum memakai envelope JSON. Selesai bertahap ketika kebutuhan endpoint terpenuhi dan diuji.
- [ ] FND-09 | RENCANA | Rapikan scaffold lama dan artefak build setelah audit referensi. Selesai jika tidak ada dua lokasi aktif dengan tanggung jawab sama, binary lokal diabaikan Git, dan tidak ada kode pengguna yang hilang.
- [ ] FND-10 | RENCANA | Rapikan formatting, dependency metadata, dan petunjuk menjalankan aplikasi. Selesai jika gofmt/build/vet lolos, perubahan go mod tidy ditinjau, dan env.example hanya memuat contoh aman.
- [ ] DEV-01 | DITUNDA | Siapkan Docker untuk environment development tim setelah alur lokal stabil dan sebelum onboarding laptop anggota. Permintaan pemilik: dikerjakan nanti, saat ini tetap lokal. Target: Dockerfile dengan versi Go/runtime terkontrol, .dockerignore, Compose/env example bila diperlukan, perintah migration yang sama, dan credential lokal dipasang dari luar image. Selesai jika anggota lain dapat build/run mengikuti petunjuk, /health bekerja, dan Supabase/Firebase memakai konfigurasi development yang benar. Docker tidak otomatis menyalin credential atau mengisolasi database Supabase bersama; kebutuhan database lokal terpisah ditentukan saat task diaktifkan.

### M2. Schema, migration, dan seed

Dependency: DATA-02 dapat dimulai setelah arsip sumber tersedia; schema transaksi menunggu keputusan terkait.

- [ ] DATA-01 | BERJALAN | PIC: pemilik repo. Arsip database/schema.dbml tersedia: 33 tabel, 14 enum, dan 44 inline reference dengan target ditemukan. Bukti LAPORAN: query metadata public menghasilkan no rows returned, mencakup tabel dasar dan enum. Penandaan rancangan awal masih perlu ditambahkan. Pemeriksaan lokal bukan validasi parser DBML resmi; schema selain public tidak diaudit.
- [ ] DATA-02 | BERJALAN | PIC: pemilik repo. Goose CLI v3.29.0 terpasang, diperiksa melalui -version. File database/migrations/00001_create_accounts.sql ditemukan dengan Up/Down, enum akun, tiga tabel, constraint saldo nonnegatif, RLS, dan pencabutan grant anon/authenticated. Bukti LAPORAN: pemilik menyatakan penerapan berhasil dan ketiga tabel beserta rowsecurity terlihat. Perilaku re-run belum dicatat; pemilik sedang mempelajari konsep migration dan rollback. Selesai jika re-run aman dibuktikan dan strategi rollback/reset development dijelaskan. Referensi tool: https://github.com/pressly/goose.
- [ ] DATA-03 | MENUNGGU KEPUTUSAN | Tarif per kategori/scope (00005) serta deposits/deposit_items dengan snapshot tarif, idempotency key, dan status awal created (00006) tersedia. Status lanjutan, QR reissue/history, klaim, dan ledger belum dibuat. Dependency: Q-02/Q-03/Q-08/Q-16. Selesai jika contoh multi-material dan seluruh transisi utama dapat direpresentasikan tanpa kehilangan histori.
- [ ] DATA-04 | RENCANA | Tambahkan migration per fitur saat diperlukan. Selesai per migration jika diterapkan pada database development, efek data diperiksa, dan jalur upgrade dari versi sebelumnya jelas.
- [ ] DATA-05 | ADA KODE | cmd/seed-partners telah diuji melalui CLI pada PostgreSQL lokal: dua run tanpa duplikasi, empat kategori/satu lokasi/tujuh jam/tiga material, serta akun user biasa ditolak. Konten lain belum di-seed dan Supabase belum diisi.
- [ ] DATA-06 | RENCANA | Dokumentasikan relasi, constraint, retensi, serta batas data sensitif. Selesai jika schema, migration, dan kontrak fitur konsisten.

### M3. Firebase Auth, akun, dan media

Dependency: schema minimal akun; konfigurasi Firebase dan kontrak Android.

- [ ] AUTH-01 | ADA KODE | PIC: AI, izin pemilik 2026-11-17. NewAuthClient dipanggil saat startup dan diteruskan melalui bootstrap/router ke middleware. Build/vet lolos. Inisialisasi credential, kegagalan konfigurasi, dan verifikasi token Firebase nyata belum diuji; credential tidak dibaca AI. Kompilasi dan verifier pengganti bukan bukti integrasi Firebase runtime.
- [ ] AUTH-02 | ADA KODE | PIC: AI, 2026-11-17. Middleware Bearer memanggil VerifyIDToken, memasang identitas pada context, dan meneruskan request ke handler. GET /api/v1/auth/me terpasang. Tes HTTP memeriksa header hilang/salah, penolakan verifier, token nil/UID kosong, penerimaan claim dan context, serta /health publik. Semua tes lolos dengan verifier pengganti; token expired/project salah dan token valid nyata belum diuji. Revocation/disabled check belum diaktifkan. Kontrak dan batas: docs/api/android-integration.md.
- [x] AUTH-03 | SELESAI | Backend pendaftaran akun/settings/wallet dan detail usaha pending lolos PostgreSQL lokal nyata: retry, konflik input, 16 request paralel dengan satu created, rollback wallet/detail usaha, dan baca profil. Kontrak docs/api/accounts.md. Firebase/Android end-to-end tetap AUTH-01/02/06; Supabase belum dimigrasi oleh AI.
- [x] AUTH-04 | SELESAI | Backend GET/PATCH /api/v1/me tersedia. PATCH hanya full_name/phone dari caller, role/status/email/saldo tidak dapat diedit, inactive ditolak. HTTP dan PostgreSQL lokal PASS 2026-11-20: partial update/retry/Unicode/paralel/isolasi dan wallet utuh. Kontrak docs/api/accounts.md; Android/Firebase/Supabase end-to-end tetap task terpisah.
- [ ] AUTH-05 | MENUNGGU KEPUTUSAN | Lengkapi status akun, profil belum lengkap, onboarding, perpindahan peran, suspensi, dan penghapusan. Dependency: Q-04/Q-12. Selesai jika Android dan backend memakai transisi serta pesan yang sama.
- [ ] AUTH-06 | RENCANA | Integrasikan alur login/register/reset password Android dengan Firebase dan profil PostgreSQL. Selesai jika akun Firebase yang profil DB-nya belum dibuat dapat dipulihkan lewat retry yang aman.
- [ ] MEDIA-01 | MENUNGGU KEPUTUSAN | Tetapkan upload melalui Android SDK atau backend, akses file, jenis/ukuran, path kepemilikan, dan pembersihan orphan. Dependency: Q-13. Selesai jika satu pengguna tidak dapat mengubah/mengambil media privat pengguna lain.
- [ ] MEDIA-02 | RENCANA | Implementasikan media pertama sesuai kontrak, misalnya foto profil atau scan. Selesai jika upload berhasil/gagal dan path yang tidak sah ditangani; file disimpan di Storage, metadata di PostgreSQL.

Batas penyelesaian M3: satu pengguna dapat login, mempunyai profil aplikasi, dan membaca profilnya melalui backend yang memverifikasi identitas. Auth, role, status partner, dan ownership merupakan pemeriksaan yang berbeda.

### M4. Kategori, mitra, dan peta

Dependency: akun, data kategori/mitra, dan kebijakan akses.

- [ ] PARTNER-01 | ADA KODE | Daftar/detail/kategori tersedia. SQL dan command seed kini lolos PostgreSQL lokal: nama/area/material, owner suspended tersembunyi, material dan jam hasil seed. Kontrak docs/api/partners.md. Migration/seed Supabase dan integrasi Android belum dijalankan.
- [ ] PARTNER-02 | ADA KODE | Pencarian nama/area, material slug, limit/offset, validasi HTTP/service lolos; query material/area juga lolos PostgreSQL nyata. Radius/GPS/sort jarak tidak diimplementasikan; hanya pencarian manual dari kebutuhan FE sekarang.
- [x] PARTNER-03 | SELESAI | Backend favorit per caller tersedia dengan GET/PUT/DELETE. HTTP dan PostgreSQL lokal membuktikan retry/paralel aman, isolasi alice/bob, hapus target hidden, penolakan akun missing/suspended, serta filtering visibility. Integrasi tombol hati Android tetap PARTNER-05; Supabase belum dimigrasi.
- [ ] PARTNER-04 | MENUNGGU KEPUTUSAN | Implementasikan onboarding/verifikasi/pengelolaan lokasi mitra sesuai Q-04. Selesai jika mitra pending/rejected/suspended tidak dapat memakai operasi yang dilarang.
- [ ] PARTNER-05 | RENCANA | Integrasikan Android peta/detail/kontak/navigasi. Selesai jika izin lokasi ditolak tetap mempunyai jalur manual dan route navigasi memakai lokasi mitra yang dipilih.

Pemilihan provider peta, metode perhitungan jarak, dan kebutuhan PostGIS ditentukan ketika kebutuhan pencarian jelas. Backend tidak meminta tracking lokasi kontinu hanya untuk menampilkan mitra sekitar.

### M5. Estimasi, setoran, QR, wallet, dan ledger

Dependency: otorisasi akun/mitra, migration transaksi, Q-02/Q-03/Q-08/Q-16. Ini merupakan satu alur transaksi lintas beberapa file; masing-masing bagian tidak dianggap selesai hanya karena CRUD tersedia.

- [ ] DEPOSIT-01 | MENUNGGU KEPUTUSAN | Definisikan kontrak multi-item, tarif aktif, pembulatan, total, penerimaan/penolakan, dan aturan edit. Selesai jika contoh plastik+kertas+logam dihitung dengan hasil yang dapat diuji bersama tim.
- [x] DEPOSIT-02 | SELESAI | Backend estimator multi-item dan tarif demo lulus HTTP/service/PostgreSQL serta CLI seed; tidak mengubah wallet, tarif tidak tersedia menghasilkan 409. Kontrak docs/api/point-estimates.md. Server setoran memakai fungsi parse/hitung yang sama dan tes PostgreSQL membuktikan total setoran = hasil estimator (DEPOSIT-03).
- [x] DEPOSIT-03 | SELESAI | POST /api/v1/deposits dan GET /api/v1/deposits/{id} untuk mitra active dengan lokasi verified. Total dihitung server dari tarif dalam transaksi yang sama, snapshot item tersimpan, Idempotency-Key wajib (replay/konflik/8 request paralel → satu setoran), berat tidak sah dan material tidak diterima ditolak tanpa baris, mitra lain menerima 404, wallet tidak berubah. Status hanya created; edit/batal menunggu Q-08. Kontrak docs/api/deposits.md; bukti docs/test-results/deposits-2026-11-20.md.
- [x] QR-01 | SELESAI | POST /api/v1/deposits/{id}/qr: token acak 32 byte base64url, hanya SHA-256 disimpan, expiry 10 menit waktu server, warning bearer (D-18) di response. Token tidak dibawa di URL/log. Bukti docs/test-results/deposit-qr-claims-2026-11-20.md.
- [x] QR-02 | SELESAI | Reissue lewat endpoint yang sama: token lama revoked, token baru 10 menit, histori tersimpan, ditolak setelah klaim; race reissue vs klaim diuji 5 putaran.
- [x] QR-03 | SELESAI | POST /api/v1/deposit-claims/preview tanpa mutasi: mitra, item, berat, poin, expiry.
- [x] CLAIM-01 | SELESAI | POST /api/v1/deposit-claims: user active pertama (D-18) mendapat poin; claim, ledger, wallet, token, status satu transaksi; 8 klaim paralel → satu penerima, satu kredit.
- [x] CLAIM-02 | SELESAI | Retry pengklaim yang sama → 200 hasil sama tanpa kredit ulang; pengklaim lain → 409; expired/revoked/invalid dibedakan.
- [ ] WALLET-01 | RENCANA | Implementasikan saldo/ledger/riwayat setoran pengguna dan mitra dengan ownership, filter, pagination. Selesai jika angka riwayat merekonsiliasi saldo dan satu transaksi tidak tampil sebagai pemberian ganda.
- [ ] DEPOSIT-04 | RENCANA | Integrasikan seluruh alur Android mitra dan pengguna, termasuk state invalid/expired/used/cancelled/pending hasil. Selesai jika demo dua akun menyelesaikan setoran dan memverifikasi satu kredit ledger.
- [ ] DEPOSIT-05 | RENCANA | Jalankan skenario integritas pada bagian 11 terhadap PostgreSQL. Selesai jika hasil race, expiry boundary, reissue, retry, dan rollback dicatat bersama constraint yang menjaganya.

Status header setoran dan status token diuji sebagai dua hal berbeda. Mematikan QR lama tidak boleh menambah kredit baru atau mengubah setoran completed menjadi awaiting_scan.

### M6. Katalog reward dan voucher simulasi

Dependency: wallet/ledger yang benar; Q-05. D-08 tetap menjadi arah sementara sampai batas simulasi dikonfirmasi.

- [ ] REWARD-01 | RENCANA | Implementasikan katalog/detail kategori reward yang aktif. Selesai jika biaya, terms, stok/ketidaktersediaan ditampilkan konsisten; data contoh diberi label demo.
- [ ] REWARD-02 | RENCANA | Implementasikan penukaran atomik dengan idempotency, pemeriksaan saldo dan stok. Selesai jika request paralel tidak membuat saldo/stok negatif atau memotong dua kali.
- [ ] REWARD-03 | MENUNGGU KEPUTUSAN | Implementasikan voucher simulasi dan transisi requested/issued/used/failed/cancelled yang benar-benar dipakai. Selesai jika kode unik, kepemilikan, pemakaian ulang, kegagalan, dan refund yang disetujui ditangani.
- [ ] REWARD-04 | RENCANA | Implementasikan koleksi/detail voucher milik pengguna serta status hasil yang dapat diambil ulang. Selesai jika pengguna lain tidak memperoleh kode voucher privat.
- [ ] REWARD-05 | RENCANA | Integrasikan Android konfirmasi, sukses, saldo kurang, stok habis, retry, dan pembatalan. Selesai jika perubahan wallet sama dengan debit ledger dan bukti redemption.

Tidak ada janji pencairan DANA atau nilai uang nyata. Pemenuhan voucher eksternal merupakan scope baru yang memerlukan keputusan produk dan integrasi tersendiri.

### M7. Foto sampah, AI, dan panduan reuse

Dependency: media, kategori/mitra; Q-06 untuk identifikasi nyata dan Q-09 untuk konten panduan.

- [ ] SCAN-01 | MENUNGGU KEPUTUSAN | Tetapkan kontrak request/hasil AI, kategori, versi model, gagal/tidak dikenali, dan mode sync/async. Selesai jika batas kemampuan dan penanganan uncertainty dapat dijelaskan kepada pengguna.
- [ ] SCAN-02 | RENCANA | Implementasikan pencatatan scan, ownership, image path, status, dan error yang dapat diperiksa. Selesai jika upload/akses tidak sah ditolak dan retry tidak menghasilkan status tidak konsisten.
- [ ] SCAN-03 | MENUNGGU KEPUTUSAN | Integrasikan model/provider nyata dengan timeout serta batas resource. Selesai jika sampel uji yang disepakati menghasilkan hasil dan kondisi gagal terukur. Mock hanya untuk development/test dan dilabeli.
- [ ] SCAN-04 | RENCANA | Implementasikan panduan handling/reuse, alat/bahan, langkah, safety notes, dan mitra yang menerima kategori. Selesai jika konten berasal dari sumber yang ditentukan dan rekomendasi mengikuti kategori yang dipakai.
- [ ] SCAN-05 | RENCANA | Implementasikan koreksi kategori dan riwayat scan sesuai aturan yang disepakati. Selesai jika kategori deteksi awal dapat dibedakan dari koreksi dan koreksi tidak memberi poin setoran.
- [ ] SCAN-06 | RENCANA | Integrasikan Android kamera/galeri, hasil, retry, tidak dikenali, izin ditolak, dan loading. Selesai jika state frontend cocok dengan kontrak backend; scan tidak mengubah wallet.

Identifikasi dari foto tidak menggantikan pemeriksaan fisik mitra. Pengukuran kualitas model, confidence, dan panduan limbah khusus memerlukan sumber dan evaluasi yang disetujui; tidak ada target akurasi yang ditetapkan tanpa bukti.

### M8. Konten, gamifikasi, dan leaderboard

Dependency konten: Q-09. Dependency gamifikasi: Q-07/Q-11. Cakupan final rilis menunggu Q-01.

- [ ] CONTENT-01 | RENCANA | Implementasikan artikel publik, pencarian/filter, dan detail. Selesai jika hanya artikel published tampil, Markdown diproses aman di client, dan hasil kosong tersedia.
- [ ] GAME-01 | MENUNGGU KEPUTUSAN | Finalisasi Pilah Cepat/Eco Quiz, bank soal, XP, replay, dan penyelesaian attempt. Selesai jika aturan penilaian dan anti-duplikasi dapat dijadikan test.
- [ ] GAME-02 | RENCANA | Implementasikan daftar/detail game, attempt, jawaban, penilaian server, dan hasil. Selesai jika client tidak menentukan benar/XP sendiri dan mengirim ulang jawaban tidak menambah XP.
- [ ] MISSION-01 | MENUNGGU KEPUTUSAN | Finalisasi definisi misi, periode, sumber progres, hadiah, dan klaim. Selesai jika misi jumlah setoran, berat, scan, games, atau kategori berbeda mempunyai aturan yang jelas.
- [ ] MISSION-02 | RENCANA | Implementasikan progres, klaim satu kali, bonus ledger bila dipakai, dan badge. Selesai jika event/retry yang sama tidak menggandakan progres atau hadiah dan expiry ditangani.
- [ ] LEADER-01 | MENUNGGU KEPUTUSAN | Finalisasi dasar skor, periode/timezone, tie-break, dan privasi. Peringkat setoran di UI merujuk poin setoran terverifikasi, bukan saldo setelah belanja.
- [ ] LEADER-02 | RENCANA | Implementasikan leaderboard setoran dan game terpisah, pagination, serta peringkat pengguna. Selesai jika perubahan saldo karena redeem tidak salah menurunkan skor setoran.
- [ ] GAME-03 | RENCANA | Integrasikan Android game/misi/badge/leaderboard dan state akhir. Selesai jika hasil XP, hadiah, dan peringkat dapat ditelusuri ke peristiwa server yang sah.

### M9. Beranda, riwayat, pengaturan, dukungan, dan laporan

Dependency: fitur sumber data yang digunakan. Bagian ini mengagregasi hasil fitur lain tanpa membuat ledger kedua.

- [ ] HOME-01 | RENCANA | Implementasikan ringkasan beranda/profil: saldo, jumlah setoran, misi, mitra sekitar, dan pencapaian yang masuk scope. Selesai jika metrik mempunyai definisi periode dan sesuai data sumber.
- [ ] HISTORY-01 | RENCANA | Implementasikan riwayat gabungan scan/setoran/redeem dengan filter jenis/waktu serta detail ownership. Selesai jika pagination stabil, status benar, dan jumlah saldo tidak dihitung dua kali.
- [ ] SETTINGS-01 | MENUNGGU KEPUTUSAN | Implementasikan onboarding, preferensi notifikasi, privasi leaderboard, dan kebutuhan akun sesuai Q-10/Q-12. Selesai jika pilihan tersimpan dan dipatuhi fitur terkait.
- [ ] NOTIFY-01 | MENUNGGU KEPUTUSAN | Definisikan kejadian notifikasi, channel, retry, dan status dibaca. Selesai jika kebutuhan push/in-app dibedakan dan tidak membocorkan data privat.
- [ ] NOTIFY-02 | RENCANA | Implementasikan channel yang disetujui dan daftar notifikasi. Selesai jika pengiriman ulang tidak menghasilkan spam/duplikasi tidak terkendali dan preferensi dipatuhi.
- [ ] SUPPORT-01 | MENUNGGU KEPUTUSAN | Implementasikan bantuan, laporan masalah, dan masukan beserta validasi/lampiran/email kontak. Selesai jika pengiriman berhasil/gagal, batas data, kepemilikan, serta cara tim menindaklanjuti dijelaskan.
- [ ] REPORT-01 | MENUNGGU KEPUTUSAN | Finalisasi periode, total berat/item, jumlah setoran, poin, grafik, dan rincian kategori menurut Q-14. Selesai jika contoh laporan dapat dihitung ulang dari transaksi sumber.
- [ ] REPORT-02 | RENCANA | Implementasikan data laporan dan PDF yang hanya dapat diakses pemilik. Selesai jika batas rentang, status pembuatan, kegagalan, dan izin unduh diuji; async hanya bila diperlukan.
- [ ] UI-01 | RENCANA | Cocokkan kontrak dengan 42 layar sumber dan daftar state tambahan. Selesai jika setiap layar/aksi dalam scope memiliki sumber data atau alasan client-only; perbedaan navigasi dan angka mockup dicatat/diselesaikan.

### M10. Pengelolaan admin

Dependency: Q-04/Q-09 dan keputusan scope tim. Ditunda tidak berarti proses pengelolaan data boleh diabaikan saat demo.

- [ ] ADMIN-01 | DITUNDA | Putuskan role/permissions, interface operasional, audit, dan kebutuhan pengguna admin. Selesai jika verifikasi mitra serta pengelolaan tarif/konten/reward mempunyai penanggung jawab.
- [ ] ADMIN-02 | RENCANA | Implementasikan operasi admin yang disetujui atau prosedur development sementara yang terdokumentasi. Selesai jika akun biasa tidak dapat mengubah role/status/ledger lewat endpoint atau payload pengguna.
- [ ] ADMIN-03 | RENCANA | Integrasikan audit untuk tindakan sensitif yang masuk scope. Selesai jika perubahan dapat ditelusuri tanpa merekam credential/token mentah.

### M11. Integrasi, kualitas, dan rilis

Dependency: fitur yang dipilih pada Q-01, data demo, dan Q-15 untuk deployment.

- [ ] QA-01 | RENCANA | Jalankan unit test aturan bisnis dan integration test PostgreSQL yang relevan. Selesai jika skenario prioritas bagian 11 lulus pada fitur yang dirilis.
- [ ] QA-02 | RENCANA | Uji kontrak Android/backend pada minimal satu perangkat/emulator dan dua peran. Selesai jika alur utama serta kondisi gagal dalam scope berhasil direproduksi.
- [ ] QA-03 | RENCANA | Periksa ownership, role/status, input, SQL parameterization, media, redaksi log, dan batas request. Selesai jika skenario penyalahgunaan yang relevan ditolak dan bukti dicatat.
- [ ] QA-04 | RENCANA | Periksa query lambat, pagination, index, pool, timeout, dan kebocoran resource pada beban demo yang disepakati. Target beban/latency belum ditentukan; jangan mengklaim kapasitas tanpa pengukuran.
- [ ] CI-01 | RENCANA | Tambahkan workflow build/vet/test dan pemeriksaan format sesuai kebutuhan repo. Selesai jika berjalan pada PR/push yang disepakati dan test integrasi menggunakan database test terpisah.
- [ ] RELEASE-01 | MENUNGGU KEPUTUSAN | Pilih hosting Go, akses Supabase, env, port, TLS, dan URL Android. Selesai jika prosedur deploy dan rollback terdokumentasi; VPS penelitian diperiksa sebelum konfigurasi jaringan diubah.
- [ ] RELEASE-02 | RENCANA | Siapkan migration release, data demo, backup dan latihan restore yang tersedia pada layanan terpilih. Selesai jika pemulihan dicoba pada database terpisah dan credential tidak berada di repo.
- [ ] RELEASE-03 | RENCANA | Jalankan smoke test deployment, observasi log, /health/readiness sesuai kebutuhan, serta alur lintas akun. Selesai jika backend tetap berjalan setelah sesi SSH/terminal pengembang ditutup.
- [ ] RELEASE-04 | RENCANA | Siapkan petunjuk menjalankan, API contract final, laporan tugas, video/demo, dan daftar keterbatasan. Selesai jika anggota lain dapat menjalankan proyek mengikuti dokumen serta scope yang belum selesai disebutkan.
- [ ] RELEASE-05 | RENCANA | Bekukan versi demo setelah pemeriksaan penerimaan tim. Selesai jika keputusan terbuka yang memengaruhi demo ditutup atau ditunda secara eksplisit dan tag/commit rilis dicatat.

## 10. Urutan pengerjaan dan pembagian tim

Usulan urutan hasil yang dapat diperiksa:

| Gelombang | Hasil | Syarat mulai |
|---|---|---|
| A | Fondasi dapat dimulai/dihentikan dan schema akun tersedia. | FND-05/FND-06 serta DATA-01/DATA-02. |
| B | Login Firebase dan profil sendiri berjalan melalui Handler-Service-Repository. | Migration minimal akun dan konfigurasi Firebase. |
| C | Daftar/detail mitra, kategori, dan favorit berjalan; aturan setoran difinalisasi. | Data development dan keputusan akses. |
| D | Setoran multi-item, QR, klaim, wallet, dan ledger selesai sebagai satu alur. | Q-02/Q-03/Q-08/Q-16 serta izin mitra. |
| E | Reward simulasi dan integrasi foto/AI sesuai keputusan. | Wallet benar; Q-05/Q-06. Keduanya bisa dikerjakan terpisah. |
| F | Konten/gamifikasi/riwayat/laporan sesuai scope; integrasi dan rilis. | Aturan masing-masing dan kapasitas yang disepakati. |

Gelombang bukan komitmen minggu kalender. Jadwal rinci dibuat setelah Q-01 dijawab. Dependensi produk, bukan banyaknya folder, menentukan urutan.

Referensi pembagian dari PDF: Fahry menangani Foto Sampah/Peta Mitra, Ghazy Scan QR, Aditya Generate QR, dan Nafis Reward/Tukar Poin. Konfirmasi kembali kepemilikan backend/Android serta siapa yang merawat fondasi bersama. Setelah disepakati, PIC dicatat pada task yang diambil.

Aturan kolaborasi:

- Satu alur transaksi membutuhkan kontrak dan invariant bersama. PIC Generate QR dan Scan QR menyepakati schema/token/status sebelum implementasi terpisah.
- Wallet/ledger dan migration yang sama mempunyai koordinasi perubahan. Hindari dua migration dengan tujuan/versi bertabrakan.
- Tiap perubahan menyebut task ID, hasil, verifikasi, serta keputusan yang terpengaruh.
- Perubahan schema/API yang dipakai anggota lain diberi catatan kompatibilitas dan langkah penyesuaian.
- Temuan baru menambah/memperbaiki task yang relevan; tidak menggandakan roadmap pada file lain.
- Penambahan dependency dipilih setelah kebutuhan nyata diketahui. ORM, router tambahan, dan queue belum menjadi keputusan otomatis. Docker development tim sudah masuk rencana DEV-01, dengan implementasi ditunda atas permintaan pemilik.

## 11. Matriks verifikasi

Ini merupakan target pengujian, bukan hasil tes yang sudah lulus.

| Kelompok | Skenario prioritas | Hasil yang harus dibuktikan |
|---|---|---|
| Config/startup | Env wajib kosong, URL salah, database tidak dapat dicapai, port bentrok. | Gagal jelas; server tidak mengaku siap setelah kegagalan dependency wajib; log aman. |
| Shutdown | Idle, request aktif, deadline habis, server gagal listen, koneksi DB dipinjam. | Lifecycle dan cleanup jelas; tidak menganggap timeout HTTP membatasi seluruh cleanup. |
| Pool/context | Batas pool tercapai, query timeout, Rows selesai lebih awal, Acquire/transaction gagal. | Pinjaman dilepas, penantian dapat dibatalkan, request berikutnya bisa dilayani. |
| Auth | Tidak ada token, rusak, expired, project salah, token valid, akun suspended/deleted. | Identitas/status diperiksa; client tidak dapat memilih UID/role sendiri. |
| Akun | Retry pembuatan profil dan dua request paralel UID sama. | Satu akun, settings, dan wallet; tidak ada saldo awal ganda. |
| Mitra | Pending/suspended membuat transaksi; mitra A mengakses transaksi B. | Izin ditolak sesuai aturan final. |
| Estimasi/setoran | Banyak kategori, berat nol/negatif, kategori tidak diterima, tarif hilang/overlap, batas pembulatan. | Nilai server konsisten dengan aturan yang disepakati dan constraint. |
| QR expiry | Sebelum expired_at, tepat expired_at, sesudahnya. | Klaim hanya diterima ketika masih berlaku dan syarat lain terpenuhi. |
| QR reissue | Reissue lalu pakai token lama; reissue bersamaan klaim. | Satu hasil/transisi sah; waktu token baru dihitung dari reissue. |
| Preview | Preview berulang, batal, QR kedaluwarsa setelah preview. | Preview tidak memutasi saldo; konfirmasi memvalidasi ulang. |
| Klaim | Pengguna sama retry, dua pengguna bersamaan, request key berbeda untuk deposit sama. | Satu kredit/penerima; constraint bisnis tetap bekerja selain idempotency key request. |
| Klaim hasil tidak diketahui | Response terputus setelah commit. | Status dapat diambil ulang; retry tidak memberi poin lagi. |
| Atomicity | Kegagalan di antara update deposit/token/ledger/wallet. | Seluruh perubahan commit bersama atau rollback; rekonsiliasi tidak berselisih. |
| Reward | Saldo kurang, stok terakhir diperebutkan, harga berubah setelah preview, retry. | Saldo/stok valid, snapshot benar, satu redemption/debit sah. |
| Voucher demo | Akses akun lain, pemakaian dua kali, failed/cancelled/refund bila disetujui. | Ownership dan lifecycle sesuai Q-05. |
| AI/media | File rusak/terlalu besar, file milik orang lain, provider timeout, kategori tak dikenali. | Penolakan/status jelas; mock tidak dianggap inference nyata; saldo tidak berubah. |
| Game/misi | Jawaban ulang, choice dari soal lain, attempt milik orang lain, replay dan klaim bonus ulang. | Server menilai dan memberi XP/hadiah sesuai aturan satu kali yang disetujui. |
| Leaderboard | Pergantian periode, skor seri, redeem poin, privasi. | Skor/peringkat mengikuti definisi, terpisah dari saldo belanja. |
| Riwayat/laporan | Filter, batas periode, banyak data, PDF milik orang lain. | Hasil dapat dihitung ulang, dipaginasi/dibatasi, serta akses terlindungi. |
| Android | Izin ditolak, offline, retry, kembali halaman, token expired. | State yang disepakati ditampilkan tanpa mutasi ganda. |

Unit test ditempatkan dekat package yang diuji; integration test menggunakan PostgreSQL development/test yang terisolasi. Pengujian race/atomicity tidak diganti sepenuhnya oleh mock. Jalankan race detector jika toolchain dan lingkungan mendukungnya; catat jika belum tersedia.

Setiap fitur selesai mencakup jalur berhasil, validasi, ownership/role yang relevan, kegagalan, dan retry yang berisiko. Tidak perlu menulis test untuk setiap getter, placeholder, atau nama folder.

## 12. Panduan development dan operasi

### 12.1 Konfigurasi

Pada snapshot, aplikasi membaca HTTP_ADDR dan DATABASE_URL dari environment proses. File .env/.env.example belum dimuat otomatis. env.example adalah contoh pengaturan; nilainya tidak membuktikan koneksi aktif.

Untuk Supabase, ambil PostgreSQL connection string dari tombol Connect. Pilihan session pooler digunakan pada setup laptop agar dapat dijangkau melalui IPv4. Pertahankan host/username dari dashboard, percent-encode karakter password yang membutuhkan encoding, dan gunakan konfigurasi SSL yang benar. API key/project URL Supabase tidak menggantikan DATABASE_URL untuk pgxpool.

Referensi operasional: [koneksi PostgreSQL Supabase](https://supabase.com/docs/guides/database/connecting-to-postgres), [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool), [HTTP Shutdown](https://pkg.go.dev/net/http#Server.Shutdown), dan [NotifyContext](https://pkg.go.dev/os/signal#NotifyContext). Verifikasi ulang dokumentasi layanan saat mengubah setup; kuota gratis/harga tidak dijadikan konstanta rencana.

Credential Firebase, password database, private token QR, kode voucher privat, dan data pribadi tidak disimpan di task/log/dokumen. Gunakan placeholder pada contoh dan env lokal/secret deployment untuk nilai nyata. Pemilihan skema credential Firebase ditambahkan setelah AUTH-01, bukan ditebak dari nama file.

### 12.2 Perintah pemeriksaan

Jalankan dari root repo. Pembaca AI mengikuti instruksi lingkungan yang berlaku; bila RTK tersedia, gunakan wrapper RTK untuk output ringkas. Perintah native berikut menjelaskan operasi dasarnya.

~~~powershell
# Formatting check tanpa menulis file
gofmt -l ./cmd ./internal ./domain

# Pemeriksaan statis
go vet -mod=readonly ./...

# Tests yang tersedia; "no test files" bukan bukti fitur diuji
go test -mod=readonly ./...

# Build ke folder output yang sudah diabaikan Git
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -mod=readonly -o ./bin/ecosnap.exe ./cmd/app

# Jalankan setelah env lokal diatur; Ctrl+C untuk shutdown
./bin/ecosnap.exe
~~~

Build ke bin mencegah binary baru muncul di root. Jangan menghapus app.exe atau perubahan pengguna otomatis; perapian artefak dibahas pada FND-09. Migration/seed mempunyai perintah tambahan setelah alatnya dipilih dan didokumentasikan.

Tests integrasi menyatakan database target yang aman dan cara menyiapkannya. Pengujian/reset database tidak diarahkan ke project bersama atau data nyata tanpa memastikan scope. Export credential, penggantian firewall, dan perubahan VPS bukan bagian implisit dari pemeriksaan build.

### 12.3 Target deployment

- Backend lokal masih memakai 127.0.0.1:8080 secara default. Android pada perangkat lain memerlukan alamat yang dapat dijangkau; localhost perangkat Android bukan localhost laptop.
- Supabase menyimpan PostgreSQL, bukan otomatis menjalankan binary Go proyek ini.
- VPS bekas penelitian dapat dievaluasi berdasarkan OS, RAM, disk, layanan aktif, dan routing/firewall. Keberadaan SDN/FRRouting/Ansible tidak membuktikan VPS cocok atau tidak cocok.
- Instalasi PostgreSQL lokal, Docker, ataupun PostgreSQL di VPS tidak diwajibkan saat memakai database Supabase.
- Keputusan Docker, proses supervisor, reverse proxy, TLS, domain, dan pemisahan environment dicatat pada RELEASE-01.
- Ketersediaan layanan gratis, biaya Firebase Storage, serta kebutuhan billing/kuota provider AI diperiksa sebelum integrasi/deployment; kata "gratis" tidak dianggap jaminan semua bagian aplikasi tanpa biaya.
- Backup, restore, migration, credential rotation, dan pemantauan error mempunyai prosedur yang dapat dijalankan tim, sesuai kapasitas layanan yang benar-benar tersedia.

## 13. Catatan verifikasi dan pembaruan

### 13.1 Audit awal

| Pemeriksaan | Hasil | Batas |
|---|---|---|
| Inventaris kode | Enam file Go aplikasi ditemukan: main, config, bootstrap, router, health, connection. | Folder lain yang diperiksa masih placeholder/kosong. |
| Build | Lolos: `rtk go build -mod=readonly -o <TEMP>/ecosnap-build-process/app-audit.exe ./cmd/app`. | Artefak audit berada di luar repo; bukan pengujian koneksi atau deployment. |
| Vet | Lolos: `rtk go vet -mod=readonly ./...`. | Bukan Staticcheck, test transaksi, atau jaminan tidak ada bug. |
| Supabase runtime | LAPORAN pemilik bahwa setup/koneksi selesai. | Audit dokumen tidak membuka credential, memanggil database remote, atau mengulang proses shutdown. |
| Skenario transaksi | Belum tersedia implementasi/test. | Tidak ditandai selesai. |

Tanggal audit mengikuti snapshot di awal dokumen. Versi Go/dependency yang berlaku diambil langsung dari go.mod/go.sum dan toolchain, bukan disalin sebagai daftar yang harus dipelihara di dokumen ini.

### 13.2 Definition of Done per task

Sebelum mengubah [ ] menjadi [x]:

1. Hasil sesuai kriteria task dan keputusan yang berlaku.
2. Kode/schema/kontrak yang diperlukan tersedia, bukan hanya folder atau response hardcoded untuk fitur yang seharusnya memakai data.
3. Verifikasi relevan dilakukan dan hasilnya dicatat; bedakan KODE, CEK, dan LAPORAN.
4. Jalur error, ownership, resource cleanup, dan idempotency diuji bila relevan terhadap task.
5. Dokumentasi API/migration atau petunjuk menjalankan yang terpengaruh diperbarui.
6. Batas yang belum diselesaikan mempunyai task/Q tersendiri, bukan disembunyikan di balik status selesai.

Task implementasi fondasi FND-01 sampai FND-04 sengaja dipisahkan dari task verifikasi runtime FND-05/FND-06. Menyelesaikan satu tidak otomatis menyelesaikan lainnya.

### 13.3 Format serah terima

Gunakan format berikut pada setiap checkpoint; tulis langsung di catatan terbaru atau tautkan PR/commit yang memuatnya.

~~~text
Tanggal:
Task ID:
PIC:
Status sebelum -> sesudah:
Hasil/perubahan:
Bukti file/commit:
Perintah dan hasil verifikasi:
Kondisi yang belum diuji:
Keputusan baru atau Q terkait:
Langkah berikutnya:
~~~

### 13.4 Riwayat checkpoint

| Tanggal | Task | Ringkasan | Langkah berikut |
|---|---|---|---|
| 2026-11-17 | AUDIT-01/AUDIT-02, DOC-04/DOC-05 | Pemilik meminta audit mendalam tanpa source baru. Inventaris hash 167 file, 26 kelompok layar/aksi, 14 gap schema/aturan, dan pertanyaan terbuka disimpan. Scope seluruh placeholder, seed mitra, pending tanpa QR, keputusan akhir satu akun satu tempat, tarif dummy editable, voucher simulasi dikonfirmasi. Tes Go dengan verifier pengganti lolos. | Handoff AI/game boleh ditunda; tetapkan pelengkapan/persetujuan mitra; perbarui keputusan/kontrak; implementasi tetap ditahan sesuai instruksi pemilik. |
| 2026-11-17 | AUTH-01/AUTH-02, kontrak Android | Pemilik mengizinkan implementasi AI setelah penjelasan batch. Source Android diaudit: auth masih navigasi, data aplikasi lokal, QR demo dan penilaian game lokal. Firebase dipasang ke startup/router; GET /api/v1/auth/me, JSON error, serta tes verifier pengganti ditambahkan. go test -mod=readonly ./..., go vet -mod=readonly ./..., dan go build -mod=readonly ./... melalui RTK lolos. Contoh env diganti placeholder. Tidak ada credential dibaca, database remote diubah, atau source Android diubah. | Uji Firebase nyata bersama Android; implementasikan profil idempotent/settings/wallet dan GET /api/v1/me. Finalisasi alur kelengkapan alamat mitra. Panduan: docs/api/android-integration.md. |
| 2026-11-03 | DOC-01/DOC-02, FND-01 sampai FND-04 | Rencana bersama dibuat; sumber dan kode diaudit; build/vet lolos; implementasi fondasi dipisahkan dari verifikasi runtime. | Konfirmasi pemahaman lifecycle/pool, tutup FND-05/FND-06, arsipkan schema pada DATA-01. |
| 2026-11-03 | FND-05 | Task mulai dikerjakan pemilik repo. Pembacaan kode memastikan shutdown sudah terpasang; belum ada hasil pengujian runtime baru. | Jalankan skenario startup/HTTP, catat hasil, lalu lanjutkan FND-06 dan DATA-01. |
| 2026-11-03 | FND-05/DATA-01 | Pemilik melaporkan rangkaian startup/HTTP selesai dan berhasil. FND-05 ditutup berdasarkan LAPORAN; pemeriksaan shutdown lanjutan tetap terbuka. DATA-01 mulai dikerjakan. | Pemilik menyimpan DBML sumber dan menjalankan query metadata tabel/enum public; belum ada perubahan database. |
| 2026-11-03 | DATA-01 | File DBML ditemukan. Pemeriksaan blok, jumlah tabel/enum, dan target 44 inline reference lolos. Belum ada deposit_items; unique deposit_id pada qr_claims tetap mengikuti rancangan awal. | Tandai file sebagai rancangan awal dan catat hasil query metadata Supabase sebelum migration. |
| 2026-11-03 | DATA-01/DATA-02 | Pemilik melaporkan query tabel/enum public menghasilkan no rows returned. Langkah migration akun pertama disiapkan dengan Goose; AI tidak membuat file SQL atau mengubah database. | Pemilik menambahkan penandaan DBML awal, memasang Goose, membuat file migration, dan memeriksa hasil penerapan. |
| 2026-11-03 | DATA-02 | Goose v3.29.0 dan file migration diperiksa lokal. Pemilik melaporkan tabel akun/settings/wallet serta RLS sudah terlihat setelah penerapan. | Jelaskan migration, Up/Down, catatan versi, dan perbedaan dengan query data; lanjutkan setelah pemilik memahami. Re-run belum dilaporkan. |
| 2026-11-04 | AUTH-01 | Pemilik menyatakan sudah memahami migration dan meminta langkah berikutnya. Kode masih mempunyai satu endpoint health; folder Firebase masih placeholder. Setup Firebase mulai diarahkan; tidak ada credential dibaca atau implementasi ditulis AI. | Pemilik menyiapkan project/provider/credential lokal, lalu memasang Admin SDK dan membuat client pada langkah berikutnya. |
| 2026-11-04 | AUTH-01/DEV-01 | Firebase Admin SDK v4.22.0 ditemukan; pemilik melaporkan setup selesai. Pemilik meminta Docker untuk menyamakan environment tim, tetapi dikerjakan nanti. | Pemilik membuat client Firebase Auth; Docker dicatat DITUNDA. File implementasi/credential/Docker tidak dibuat atau dibaca AI. |
| 2026-11-04 | AUTH-01/AUTH-02 | Constructor Firebase ditemukan dan build seluruh package lolos. Error build yang dilaporkan pemilik memakai ./.. atau ./, sedangkan sumber main berada di cmd/app. | Jelaskan pointer dan pola build; pemilik menyiapkan middleware token, lalu client dirakit ke startup/router. |

Jika dokumen ini bertambah terlalu panjang untuk satu sesi, baca bagian awal dan bagian task yang dikerjakan. Detail PRD, schema, kontrak API, dan bukti gambar tetap berada pada sumber masing-masing; BUILD_PROCESS.md menyimpan urutan, keputusan, status, dan cara memverifikasinya.


### Checkpoint batch akun setelah audit

Pemilik mengizinkan melanjutkan kode dengan penjelasan lokasi/tujuan terlebih dahulu serta nama file yang spesifik. Implementasi POST /api/v1/accounts dan GET /api/v1/me tersedia; kontrak docs/api/accounts.md. Migration tambahan menyimpan pendaftaran usaha pending. Source Android tidak berubah, database remote tidak dimutasi. AUTH-03/04 berstatus ADA KODE; kelengkapan/persetujuan mitra dan pengujian PostgreSQL nyata tetap terbuka. Batas pengujian tercatat pada kontrak. Tahap audit sebelumnya merupakan snapshot historis, bukan status aktif.

Verifikasi batch akun: `go test -mod=readonly ./...` melalui RTK: 15 passed in 11 packages; `go vet -mod=readonly ./...` exit 0; `go build -mod=readonly ./...` exit 0; `gofmt -l` pada sumber terkait kosong. Tes storage pengganti tidak memverifikasi SQL/transaction PostgreSQL.

### Checkpoint pengujian repository PostgreSQL

Pemilik meminta step berikutnya setelah penjelasan akun. Interface yang diuji: AccountRepository.Register/Profile. Ditambahkan integration test dengan database acak per run, sembilan skenario termasuk retry paralel, fault injection wallet/detail usaha, dan SQL Up/Down. Runner scripts/test-accounts-postgres.ps1 membuat serta membersihkan container miliknya; syntax PowerShell diperiksa tanpa parse error. Tidak ada dependency Go baru atau migration remote diterapkan.

Bukti eksekusi: `rtk bypass go test -p 1 -mod=readonly -count=1 -v ./internal/repository/postgres -run '^TestAccountRepositoryIntegration$'` menampilkan SKIP karena environment database tes tidak tersedia. Docker info gagal pada pipe dockerDesktopLinuxEngine. Kode test berhasil dikompilasi setelah membatasi resource Go. Langkah eksternal berikutnya: aktifkan Docker Engine, jalankan runner, lalu catat hasil masing-masing subtest. Jangan menandai AUTH-03 selesai dari hasil skip.

Verifikasi tambahan batch ini: `go test -p 1 -mod=readonly ./...` melalui RTK: 15 passed in 11 packages; suite PostgreSQL tetap SKIP sebagaimana output verbose di atas. `go vet -p 1 -mod=readonly ./...` exit 0. Kedua perintah dijalankan dengan batas resource sementara pada proses.

### Checkpoint kategori dan mitra

Pemilik meminta melanjutkan fitur sambil menunda verifikasi database akun. Batch BE-02 menyediakan tiga route baca, migration 00003, DTO sesuai peta FE, dan seed development eksplisit. Akun tetap satu lokasi. Migration/seed tidak dijalankan ke database remote. Full go test awal gagal karena disk C penuh; cache build Go dibersihkan dan file test yang sempat kosong diperbaiki. Kegagalan tersebut bukan hasil tes aturan bisnis. Hasil verifikasi lanjutan harus dibaca terpisah dari SQL/seed yang belum diuji.

Verifikasi batch mitra: tes service 2 passed; tes HTTP 19 passed. Seluruh package melalui `go test -p 1 -ldflags="-s -w" -mod=readonly ./...` menghasilkan 21 passed in 12 packages; suite PostgreSQL tetap SKIP tanpa environment tes. `go build -p 1 -mod=readonly ./...` exit 0. Binary tes memakai simbol debug yang di-strip untuk menekan kebutuhan disk. Pemeriksaan formatting dan local link dokumen lolos. Migration/seed dan SQL mitra pada PostgreSQL nyata tetap belum dijalankan.
`go vet -p 1 -mod=readonly ./...` juga selesai dengan exit 0 pada batch mitra.

### Checkpoint favorit dan PostgreSQL nyata

Hasil full suite portable PostgreSQL: exit 0, semua tes PASS, tanpa SKIP. Interface yang diuji: HTTP route, AccountRepository.Register/Profile, FavoriteRepository.Set/List, PartnerRepository.List/Detail/Categories, dan command seed-partners. Ada 9 skenario akun, 4 kelompok skenario favorit/query mitra, serta uji CLI seed (dua run idempotent dan role user ditolak). SQL migration 00001-00004 diuji Up/Down dan reapply dengan reset pool sesudah destructive enum recreation. Tidak menguji version tracking Goose, credential Firebase nyata, atau Supabase runtime.

Docker masih tidak aktif. Alternatif runner portable memakai arsip Windows EDB yang dirujuk postgresql.org; runtime di LOCALAPPDATA/EcoSnapTestPostgres, bukan dependency repo atau service Windows. Komponen arsip diverifikasi ukuran dan CRC ZIP. Cluster sementara dihentikan/dibersihkan melalui finally. Upaya unduhan penuh dibatalkan dan diganti runtime minimum. Runner Docker juga diperbarui untuk seluruh package. Perbaikan kegagalan fixture dijelaskan pada docs/test-results/postgres-favorites.md.

Pemeriksaan akhir batch favorit: PostgreSQL 17.12 terkonfirmasi; full suite diulang setelah penyesuaian tes HTTP dan seluruhnya PASS tanpa SKIP. Vet/build exit 0. Formatting dan local link dokumen lolos. Migration Supabase/credential Firebase/Android tetap belum diuji end-to-end.

### Checkpoint 2026-11-20: tes ulang dan AUTH-04

Pemilik meminta tes dahulu setelah ruang penyimpanan tersedia. Baseline suite lengkap PASS pada PostgreSQL portable, lalu implementasi edit profil dijelaskan dan ditambahkan di account_profile_update.go. Tes target awal merah karena route belum tersedia; setelah implementasi tes HTTP serta repository nyata PASS. Seluruh suite lama tetap PASS tanpa SKIP, server tes berhenti dan cluster dibersihkan. Tidak ada migration baru, mutation Supabase, atau perubahan Android. Laporan rinci: docs/test-results/profile-update-2026-11-20.md. Prioritas berikutnya adalah kontrak tarif/estimasi multi-item dan transaksi/QR/ledger, dengan keputusan bisnis terbuka tetap dicatat.

Verifikasi akhir 2026-11-20: `go vet -p 1 -mod=readonly ./...` dan `go build -p 1 -mod=readonly ./...` exit 0. Gofmt dan git diff --check tidak melaporkan masalah. Direktori cluster tes tidak menyisakan cluster aktif.

## 14. Strategi branch dan baseline Git

Pemilik meminta strategi branch dicatat dahulu, kemudian pekerjaan dimulai dari langkah pertama. Branch dibagi menurut hasil fitur/sistem bisnis yang dapat diuji. Satu branch dapat mencakup entity, migration, repository, service, handler, tes, dan dokumentasi. Tidak membuat branch terpisah untuk layer repository/service/handler.

### 14.1 Branch tetap dan pola kerja

- main: target versi stabil/demo. Saat awal pengaturan, main lokal masih commit scaffold 071320d. Belum ada promosi baseline ke main.
- dev: titik integrasi bersama, dimulai dari snapshot baseline implementasi sekarang. Hasil tes checkpoint terakhir harus dibaca; promosi ke main memerlukan verifikasi ulang yang lolos.
- Feature/chore branch: berumur sesuai pekerjaan; setelah PR digabung dan tidak dipakai lagi, branch dapat dihapus. Daftar di bawah adalah backlog nama, bukan instruksi membuat semua branch sekaligus.
- Feature branch berikutnya dibuat dari dev terbaru. Setiap perubahan membawa kontrak/migration/tes/dokumentasi terkait dan bukti pengujian sebelum PR ke dev.
- Promosi dev ke main dilakukan setelah fitur gabungan diverifikasi. Pembuatan PR/push/proteksi branch GitHub belum dikerjakan pada tahap lokal ini.
- Jangan menyimpan fitur selamanya di branch terpisah. Jangan menyimpulkan isi/kelengkapan fitur frontend hanya dari nama branch.

### 14.2 Rancangan branch

| Branch | Hasil/cakupan |
|---|---|
| chore/backend-baseline | Snapshot source dan bukti kerja yang sudah tersedia sekarang, termasuk fitur yang sebelumnya dibuat pada working tree main. |
| feat/accounts-profile | Perubahan berikutnya pada akun/profil/settings/status; akun yang sudah ada diwarisi dari baseline, bukan dibuat ulang. |
| feat/partner-management | Direktori/detail/favorit serta pelengkapan dan pengelolaan satu lokasi per akun. |
| feat/deposit-estimation | Tarif material, presisi/pembulatan, kalkulator estimasi multi-material tanpa mutasi saldo. |
| feat/deposit-qr-claims | Setoran, issue/reissue QR, klaim, ledger, dan mutasi wallet yang konsisten. |
| feat/reward-redemption | Katalog/status/stock, penukaran atomik, koleksi voucher simulasi, lifecycle pemakaian/refund sesuai keputusan. |
| feat/waste-analysis | Media foto, kontrak provider AI, hasil identifikasi, dan panduan/reuse. |
| feat/articles | Konten terbit, daftar/detail, kategori, dan pencarian artikel. |
| feat/games-xp | Attempt/soal/jawaban, penilaian server, hasil, dan replay policy XP. |
| feat/missions-badges | Progres misi, event sumber, klaim hadiah satu kali, dan pencapaian. |
| feat/leaderboards | Ranking setoran/XP, periode, tie-break, dan privasi. |
| feat/activity-reports | Ringkasan home, riwayat aktivitas, statistik, dan PDF. Pecah per hasil jika terlalu besar. |
| feat/notifications-support | Inbox/preferensi notifikasi serta laporan masalah/masukan; pecah lagi jika dikerjakan terpisah. |
| chore/deployment | Docker aplikasi, CI, hosting, konfigurasi, dan petunjuk menjalankan tim ketika task tersebut diaktifkan. |

Branch besar deposit-qr-claims dapat dipecah ketika kontraknya sudah jelas menjadi feat/deposit-creation, feat/qr-issuance, dan feat/qr-claim-wallet. Klaim dan mutasi saldo/ledger tetap satu operasi atomik; pemecahan branch tidak memecah transaksi bisnis tersebut.

Scope admin belum dibekukan. Pemeriksaan awal ref lokal frontend menunjukkan 12-feat/admin hanya menambahkan folder placeholder dari 11-feat/points; perlu pemeriksaan branch frontend lebih lanjut sebelum menentukan backend admin. Ini bukan bukti admin selesai atau alasan menghapusnya dari scope.

### 14.3 Urutan awal

1. Catat rencana ini dan periksa perubahan working tree yang sudah ada.
2. Abaikan binary lokal dan file credential; .env.example hanya placeholder. Source, migration, kontrak, dan bukti desain yang memang sudah diarsipkan ikut baseline.
3. Buat chore/backend-baseline tanpa membuang perubahan, jalankan pemeriksaan, lalu commit baseline lokal.
4. Buat dev dari baseline dan catat referensi commit. Jangan merge ke main atau push otomatis.
5. Pekerjaan fitur baru pertama yang direncanakan adalah feat/deposit-estimation. Nilai tarif contoh boleh ditentukan sementara; formula dan presisi harus ditulis dalam kontrak sebelum implementasi.

Checklist pelaksanaan hanya berada pada GIT-01/GIT-02 di backlog. Status fitur lama tetap berdasarkan task dan tes yang sudah ada; membuat baseline tidak membuat seluruh aplikasi selesai.

### 14.4 Pemeriksaan penyimpanan baseline 2026-11-20

Strategi branch dicatat terlebih dahulu; chore/backend-baseline dibuat dari main 071320d tanpa membuang working tree. Kandidat baseline mencakup source/migration/tes/kontrak serta bukti desain lokal yang memang telah diarsipkan. app.exe diabaikan lewat .gitignore. Pemeriksaan pola private key/token pada kandidat dan index tidak menemukan pola yang dicari; ini bukan jaminan audit secret menyeluruh. Whitespace pada metadata Markdown/EOF catatan Figma dirapikan agar diff --check bersih.

Pemeriksaan kali ini TIDAK semuanya lolos: go vet berhasil, tetapi build gagal menulis binary sementara karena disk penuh. Full suite PostgreSQL lolos HTTP/service serta sebagian skenario akun, lalu gagal checkpoint/cleanup dan query berikutnya akibat No space left on device. Hasil full PASS pada checkpoint profil/favorit sebelumnya tetap riwayat; tidak diganti menjadi klaim PASS pada run baseline ini.

Cache build Go yang dapat dibuat ulang dibersihkan. Server cluster tes 7948d896d9b7408a92c78b1b5a17ba36 dihentikan secara immediate setelah fast shutdown gagal; ini hanya cluster disposable lokal. Pembersihan direktori cluster setelah status berhenti ditolak automatic approval review dengan alasan blocked by policy. Direktori dipertahankan, tidak dicoba dihapus melalui cara lain. Supabase/source/credential tidak dihapus.

Baseline ini menyimpan pekerjaan agar tidak hilang; bukan promosi release/main atau bukti environment runtime siap. Verifikasi ulang penuh ditahan sampai ruang penyimpanan tersedia. Jangan mulai fitur baru atau menandai tes baseline PASS berdasarkan hasil parsial ini.

### 14.5 Hasil langkah pertama

- chore/backend-baseline menunjuk commit db51b4b (chore: capture backend implementation baseline).
- dev dibuat dari db51b4b; commit berikutnya hanya mencatat hasil pengaturan Git ini.
- main tetap pada 071320d, sehingga baseline belum dipromosikan sebagai release.
- Seluruh rencana nama branch telah dicatat; feature branch lain belum dibuat.
- Tidak ada push ke remote. Lanjutkan dari dev; GIT-03 (ruang dan tes ulang) masih terbuka sebelum feat/deposit-estimation.

### 14.6 Pemeriksaan ulang baseline dan guard ruang

Permintaan pemilik: periksa apakah baseline perlu dibuat lagi dan lanjut jika siap. Hasil: baseline Git masih lengkap dan tidak perlu direkreasi. git merge-base --is-ancestor chore/backend-baseline dev exit 0; git diff --name-only chore/backend-baseline..dev hanya BUILD_PROCESS.md.

Ruang awal sekitar 413.106.176 byte. Tidak dilakukan penghapusan cache/folder atau pengulangan tes yang menulis database pada kondisi ini. Sebagai perbaikan prasyarat, chore/test-runner-preflight menambahkan helper ruang dan skrip uji tanpa alokasi besar. Kedua runner memeriksa drive repo/TEMP/cache Go sebelum melakukan operasi startup. GIT-03 tetap terbuka sampai full suite dan build berhasil dengan ruang memadai. Tidak ada feature branch deposit-estimation, merge dev/main, atau push pada tahap ini.

### Pemeriksaan ulang setelah ruang tersedia

Baseline db51b4b tetap dipakai dan tidak direkreasi. Pada awal run ini C menyediakan sekitar 6.920.720.384 byte sehingga preflight runner lolos. Full suite scripts/test-postgres-portable.ps1 selesai exit 0 tanpa SKIP: HTTP/service, profil, sembilan skenario repository akun, empat kelompok favorit/query mitra, serta CLI seed mitra. Server berhenti dan cluster run ini dibersihkan. go build -p 1 -mod=readonly ./... juga exit 0. Tes service tambahan tanpa VCS stamping/vet bukan pengganti full suite. Pemilik mengonfirmasi floor setelah penjumlahan total pada D-17; nilai tarif awal tetap contoh editable. Tidak ada source aplikasi yang diubah sebelum hasil baseline tersedia. Pengujian Firebase nyata/Supabase/Android end-to-end belum dilakukan.

### Checkpoint estimator multi-material 2026-11-20

chore/test-runner-preflight digabung fast-forward ke dev pada 937a9b7; feat/deposit-estimation dibuat dari commit tersebut. Batch dijelaskan sebelum perubahan: DTO entity, service hitung integer, repository pemilihan tarif, handler/route/bootstrap, migration 00005 dan CLI/file seed demo. Floor dilakukan sekali setelah penjumlahan semua hasil item sesuai jawaban pemilik. Tarif/batas demo tidak dinyatakan sebagai keputusan bisnis produksi.

Tes awal service/HTTP merah karena interface/route belum tersedia, lalu hijau setelah implementasi. Run PostgreSQL pertama menemukan fixture mitra belum mengisi kolom lokasi wajib; fixture diperbaiki. Full suite berikutnya exit 0 tanpa SKIP, termasuk tes lama, estimator global/lokasi/fallback/versi, kategori ditolak, owner suspended, wallet tetap, serta CLI seed dua kali dan penolakan penimpaan harga. Migration 00001-00005 Up/Down/reapply lulus. Server cluster disposable berhenti dan cluster run ini dibersihkan. Rincian: docs/test-results/point-estimates-2026-11-20.md.

go vet -p 1 -mod=readonly ./... dan go build -p 1 -mod=readonly ./... exit 0. gofmt -l pada semua file Go yang berubah kosong; git diff --check lolos. Android/Supabase/Firebase token nyata belum diuji pada batch ini. Folder features/deposit pada worktree FE tetap .gitkeep; desain kalkulator dalam arsip tetap acuan dan kebutuhan item/DTO dicatat pada kontrak. Tidak ada QR/setoran/ledger/saldo yang dibuat. DEPOSIT-02 tetap ADA KODE karena reuse pada server setoran masih menunggu DEPOSIT-03. Langkah berikutnya adalah kontrak dan migration setoran multi-item, snapshot tarif, otorisasi mitra, kemudian QR 10 menit/reissue/claim atomik sesuai backlog. Tidak ada merge main atau push.

### Checkpoint setoran multi-item 2026-11-20

Pemilik meminta langsung menerapkan DEPOSIT-03 setelah rencana dijelaskan. feat/deposit-creation dibuat dari e41b1b1. Estimator direfaktor menjadi parseEstimateItems/priceEstimate dan selectRatesTx agar setoran memakai aturan validasi, pemilihan tarif, dan pembulatan yang sama. Migration 00006 menambah deposits/deposit_items (snapshot, CHECK hasil persis, unique partner+key). Keputusan sementara yang tidak mengunci Q-08/Q-16: status hanya created, tanpa endpoint edit, tanpa penerima setoran, detail hanya untuk mitra pemilik. Idempotency-Key wajib karena retry Android tidak boleh membuat setoran ganda.

Full suite PostgreSQL portable exit 0 tanpa SKIP termasuk TestDepositIntegration (6 skenario) dan TestDepositHTTP; migration 00001-00006 Up/Down/reapply lulus; go vet exit 0. Supabase/Firebase nyata/Android belum diuji. Langkah berikutnya: putuskan Q-08 (perubahan/pembatalan/penolakan fisik) dan Q-16 (QR bearer atau terikat pengguna), lalu QR-01 penerbitan token 10 menit.

### Checkpoint QR bearer dan klaim 2026-11-20

Pemilik memutuskan D-18 (QR bearer, pengklaim pertama, cukup warning) dan D-19 (setoran terkunci setelah QR terbit; koreksi lewat banding Q-17). Enum deposit_status pada 00006 diperluas (belum pernah diterapkan ke Supabase); 00007 menambah deposit_qr_tokens, deposit_claims, point_ledger. Full suite PostgreSQL portable exit 0, 0 SKIP, termasuk TestDepositQRClaimIntegration (8 skenario) dan TestDepositQRHTTP. Belum: Android scan, Firebase/Supabase nyata, WALLET-01 riwayat, banding. Langkah berikut: WALLET-01 (saldo/riwayat ledger) atau REWARD-01/02.
