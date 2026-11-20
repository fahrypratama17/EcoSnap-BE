# EcoSnap - Catatan bukti untuk PRD

Status: analisis awal. Bukti tambahan dari pemilik proyek masih akan diterima. Dokumen ini belum menetapkan PRD final, prioritas rilis, atau keputusan implementasi.

## Sumber dan batas interpretasi

1. **PDF UTP resubmission, 33 halaman.** Teks dan gambar tersedia dalam [ekstraksi Markdown](utp-resubmission.extracted.md). Nomor halaman di catatan ini mengikuti urutan halaman fisik PDF, bukan daftar isi.
2. **DBML yang dikirim dalam percakapan.** Dipakai hanya untuk perbandingan awal. Schema tersebut belum disimpan sebagai file sumber di repo pada saat catatan ini dibuat.
3. **Konfirmasi stack dalam percakapan.** Pemilik proyek mengonfirmasi Firebase dan PostgreSQL. Backend Go sudah menjadi konteks pengerjaan. Firebase Auth untuk login dan Firebase Storage untuk media tercantum dalam catatan Project DBML; PDF belum menjelaskan integrasinya.
4. **Figma dan 10 gambar UI dari pengguna.** Inventaris tiga page, salinan gambar, status export, dan pemetaan 42 layar Hi Fi tersedia dalam [referensi Figma](figma/README.md). Temuan kelengkapan dan konsistensi disimpan dalam [catatan UI](figma/ui-completeness-notes.md). Batas panggilan Figma MCP Starter menghentikan export massal; metadata berhasil dibaca dan satu pop-up berhasil diekspor langsung.

Label **tertulis dalam PDF** menunjukkan kebutuhan yang disebut sumber. Label **perlu keputusan** menunjukkan hal yang belum cukup jelas. Label **usulan** menunjukkan analisis yang masih perlu dibahas.

## Ringkasan produk dari PDF

EcoSnap adalah aplikasi Android berbasis Kotlin untuk membantu pengguna mengidentifikasi sampah dari foto, memperoleh panduan penanganan dan pemanfaatan kembali, menemukan mitra penerima sampah, serta memperoleh EcoPoints dari setoran yang diverifikasi. EcoPoints dapat ditukarkan dengan reward. Sumber: halaman 3-6 dan 27-29.

Dua aktor utama adalah pengguna umum dan mitra/bank sampah. Pengguna mengidentifikasi sampah, mencari mitra, mengklaim transaksi, dan menukar poin. Mitra memeriksa sampah secara fisik, mencatat jenis serta berat aktual, menghitung poin melalui sistem, dan menerbitkan QR transaksi. Sumber: halaman 5-6.

## Fungsi utama yang tertulis dalam PDF

| Fungsi | Perilaku yang disebut sumber | Halaman |
|---|---|---|
| Foto sampah | Kamera atau galeri, identifikasi AI, hasil kategori/material, panduan penanganan, ide pemanfaatan kembali, dan rekomendasi mitra. Identifikasi gagal menyediakan jalur pengambilan foto ulang. | 4, 12-13, 17-18, 27 |
| Peta mitra | Lokasi perangkat atau lokasi manual, pencarian dalam jangkauan tertentu, filter jenis sampah, detail mitra, favorit, kontak, dan akses navigasi. | 6, 10-12, 18, 27-28 |
| Scan QR dan klaim poin | Pemindaian, validasi, preview transaksi, konfirmasi, validasi ulang, pemberian poin, dan riwayat. Hasil yang belum diketahui ditangani melalui pemeriksaan status. | 7-10, 18, 28 |
| Reward dan tukar poin | Katalog, biaya dan ketersediaan reward, pemeriksaan saldo, konfirmasi, validasi server, pencatatan penukaran, serta bukti/status hasil. | 13-14, 18, 28 |
| Kalkulasi poin dan generate QR | Mitra mencatat jenis dan berat hasil pemeriksaan fisik, sistem menghitung poin, mitra memeriksa hasil, kemudian sistem menerbitkan QR. | 15-16, 18-19, 28-29 |

Kelima fungsi tersebut disebut sebagai fungsi utama pada bagian 2.4. Status wajib untuk penilaian dan prioritas rilis masing-masing fungsi masih perlu dipastikan dari bukti tugas atau kesepakatan tim.

Fitur lain juga disebut atau ditampilkan: onboarding, registrasi dua peran, login, pemulihan password, profil, pengaturan, notifikasi, riwayat, laporan aktivitas, artikel, misi mingguan, badge/pencapaian, mini games, leaderboard setoran dan games, bantuan, laporan masalah, serta masukan. Sumber: halaman 5-6, 17, 20-28. Rincian perilaku dan prioritas fitur-fitur ini belum setara dengan lima fungsi utama.

## Aturan bisnis yang tertulis dalam PDF

| ID | Aturan | Halaman |
|---|---|---|
| BR-01 | EcoPoints dari setoran dan XP mini games dicatat terpisah. EcoPoints digunakan untuk penukaran reward. | 6 |
| BR-02 | Mitra memeriksa kondisi fisik sampah dan menerima atau menolak setoran. Hasil AI menjadi referensi awal dan dapat dikoreksi berdasarkan pemeriksaan fisik. | 6 |
| BR-03 | Poin dihitung dari berat sampah dalam kilogram dikalikan poin per kilogram. Tarif disebut bergantung pada jenis sampah dan ketentuan yang tersimpan dalam sistem. | 19 |
| BR-04 | QR merujuk transaksi dalam sistem. Nilai poin tidak langsung dipercaya dari isi QR. | 8 |
| BR-05 | Klaim memerlukan transaksi sah, setoran disetujui, QR belum digunakan, dan umur QR kurang dari 10 menit. Tepat 10 menit sudah dianggap kedaluwarsa. | 8 |
| BR-06 | Preview transaksi belum memakai QR dan belum menambah poin. | 8 |
| BR-07 | Pembatalan konfirmasi klaim tidak memakai QR dan tidak mengubah saldo. QR tetap dapat diklaim selama masih memenuhi syarat. | 9 |
| BR-08 | Saat konfirmasi, sistem memvalidasi ulang sesi dan transaksi, mengikat penerima, memakai QR, menambah poin, serta mencatat riwayat sebagai satu transaksi. | 9 |
| BR-09 | Pengguna pertama yang berhasil menyimpan klaim menjadi penerima poin. Jika hasil request belum diketahui, aplikasi memeriksa status sebelum menentukan langkah berikutnya. | 9-10 |
| BR-10 | Mitra dapat membuat ulang QR transaksi yang belum diklaim dengan menonaktifkan QR sebelumnya. | 6 |
| BR-11 | Pembatalan konfirmasi penukaran reward tidak mengubah saldo. Server memeriksa ulang identitas, saldo aktual, status/ketersediaan reward, dan permintaan transaksi. | 13 |
| BR-12 | Pengurangan poin dan pencatatan penukaran dilakukan secara atomik. Identitas transaksi unik digunakan untuk mencegah pemotongan poin berulang. | 13-14 |
| BR-13 | Penolakan izin lokasi tetap menyediakan pencarian lokasi manual. Hasil pencarian kosong menyediakan perubahan filter atau lokasi. | 11-12, 27-28 |

## Inventaris visual

Gambar sumber dipertahankan sebagai file tersendiri dan ditautkan pada setiap halaman ekstraksi. Gambar memerlukan pembacaan visual; text layer tidak mencakup seluruh teks di dalam diagram atau mockup.

| Halaman | Isi visual |
|---|---|
| 5 | Use case keseluruhan aplikasi dengan dua aktor. |
| 7-9 | Flowchart akses kamera, validasi QR, konfirmasi, dan hasil klaim. |
| 11-12 | Flowchart peta mitra dan foto sampah. |
| 14-16 | Flowchart reward, perhitungan poin, dan generate QR. |
| 20 | Splash screen dan tiga layar onboarding. |
| 21 | Login, registrasi pengguna/mitra, dan pemulihan password. |
| 22 | Beranda, peta, detail mitra, kamera, hasil identifikasi, dan scan QR. |
| 23 | Hasil klaim, katalog/detail reward, serta halaman games. |
| 24 | Permainan pemilahan, hasil permainan, kalkulator, dan artikel. |
| 25 | Detail artikel, leaderboard, riwayat, dan profil. |
| 26 | Pengaturan, bantuan, laporan aktivitas, laporan masalah, dan masukan. |
| 27 | Beranda mitra, input berat, dan generate QR. |

## Ketidaksesuaian dan keputusan terbuka

| ID | Temuan | Dampak terhadap PRD atau schema |
|---|---|---|
| GAP-01 | PDF mencatat jenis sampah dan tarif berdasarkan jenis. DBML percakapan belum menyediakan kategori pada `deposits` maupun `point_rates`. | Perlu menentukan satu atau beberapa jenis sampah per setoran, pencatatan detail material, serta cakupan tarif per kategori dan mitra. |
| GAP-02 | Hasil perkalian berat dan tarif dapat berupa pecahan, sedangkan poin dalam DBML memakai integer. PDF belum menetapkan pembulatan. | Tetapkan aturan pembulatan, ketelitian input, dan kapan pembulatan dilakukan. |
| GAP-03 | PDF mengizinkan QR dibuat ulang. `qr_claims.deposit_id` pada DBML bersifat unique. | Model satu row yang diperbarui bisa menyimpan token aktif; riwayat penerbitan memerlukan model tambahan atau perubahan relasi. Tentukan kebutuhan audit dan awal masa berlaku QR pengganti. |
| GAP-04 | Narasi halaman 8 menetapkan batas 10 menit. Mockup generate QR halaman 27 menampilkan countdown `04:59`. | Konfirmasi apakah mockup menunjukkan sisa waktu dari durasi 10 menit atau memakai durasi awal berbeda. Nilai UI contoh belum menjadi keputusan perubahan aturan. |
| GAP-05 | PDF menyebut mitra menyetujui/menolak setoran, tetapi DBML belum mempunyai status atau atribut persetujuan/penolakan yang eksplisit. | Tentukan apakah data transaksi baru dibuat setelah setoran diterima, atau penolakan juga perlu dicatat. |
| GAP-06 | Registrasi mitra disebut dalam PDF; DBML memiliki status partner pending, verified, rejected, dan suspended. Aktor yang memverifikasi mitra belum dijelaskan. | Tentukan siapa yang menyetujui mitra dan tindakan yang diizinkan sebelum verifikasi. |
| GAP-07 | Preview, klaim, pembatalan, hasil belum diketahui, dan retry sudah dibahas untuk QR. Mekanisme penyelesaian reward ketika hasil belum diketahui belum sedetail QR. | Lengkapi acceptance criteria penukaran untuk request berulang, timeout, stok berubah, dan pemenuhan reward gagal. |
| GAP-08 | Reward dicontohkan sebagai voucher pada mockup. Sumber voucher, proses penerbitan, pemakaian, dan pengembalian poin belum dijelaskan lengkap. | Tentukan cakupan demo dan integrasi nyata, penanggung jawab pemenuhan, serta aturan gagal/batal. |
| GAP-09 | AI disebut sebagai pemroses identifikasi, tetapi model/provider, lokasi pemrosesan, ambang hasil, dan batas kategori belum ditentukan dalam PDF. | Tetapkan kontrak hasil AI, penanganan tidak dikenali, koreksi pengguna, dan kegagalan layanan. |
| GAP-10 | Beberapa fitur gamifikasi, laporan, notifikasi, dan konten disebut atau muncul di mockup tanpa aturan lengkap. | Tetapkan scope wajib, fitur lanjutan, sumber konten, perhitungan skor, periode, dan mekanisme klaim hadiah. |
| GAP-11 | Firebase Auth, Firebase Storage, dan PostgreSQL disebut dalam konteks percakapan/DBML, tetapi integrasi teknisnya belum dijelaskan dalam PDF. | Catat sebagai keputusan teknis terpisah; tentukan pembuatan profil setelah login, pemeriksaan role/status, serta akses media. |

## Bagian template yang perlu dipisahkan

- Halaman 1 masih memuat placeholder nomor kelompok.
- Daftar isi halaman 2 belum mengikuti nomor halaman fisik isi dokumen.
- Subbagian konfirmasi klaim halaman 9 kembali memakai nomor `2.2.1.1`.
- Halaman 19 masih memakai istilah CRUD Inventaris/Transaksi Kasir dan label Screen Flow `(Berlima)`, sementara daftar anggota memuat empat orang.
- Urutan fungsi Generate QR dan Reward dalam pembagian tugas halaman 19 tidak konsisten dengan daftar fungsi pada halaman 18-19. Kepemilikan fitur dapat dikenali dari nama fitur, tetapi nomor fungsi perlu dibetulkan.
- Halaman 30-33 berisi instruksi dan contoh template aplikasi gudang, seperti prediksi restock, SmartGudang, contoh repository, dan placeholder video. Bagian tersebut tidak membuktikan implementasi atau pengujian EcoSnap.

## Usulan urutan penyusunan PRD

1. Tambahkan bukti tugas, rubrik, desain terbaru, dan keputusan tim, lalu catat sumber setiap requirement.
2. Pastikan scope untuk rilis tugas, dua peran pengguna, dan lima fungsi utama. Fitur tambahan perlu diberi prioritas secara eksplisit.
3. Putuskan aturan material/tarif, transaksi setoran, QR, dan reward sebelum menyesuaikan schema.
4. Tulis user story, alur utama, kondisi gagal, dan acceptance criteria yang dapat diuji untuk setiap fungsi.
5. Cocokkan PRD dengan desain layar dan ERD, lalu susun implementasi backend secara bertahap.

Belum ada keputusan baru mengenai scope, batas performa, provider AI, durasi proyek, atau deadline dalam catatan ini.
