# EcoSnap - Kelengkapan UI untuk bahan PRD

Status: temuan awal berdasarkan metadata tiga page Figma dan 10 gambar yang dikirim pengguna. Daftar ini belum menetapkan perubahan desain atau scope rilis.

## Batas pemeriksaan

- File Figma berhasil diakses melalui konektor. Struktur Hi Fi, WireFrame, dan Design System terbaca.
- Inventaris Hi Fi memuat 42 layar utama dan 5 frame tambahan. Seluruh 42 layar utama memiliki padanan visual pada 10 gambar pengguna. Nama, node ID, dan link referensinya tersedia dalam [inventaris](README.md).
- Pemeriksaan ini mencocokkan kelompok layar dan isi visual. Prototype interactions, seluruh properti komponen, versi revisi, serta identitas piksel antara screenshot dan Figma belum diperiksa.
- Satu frame tambahan, pop-up serbaguna, berhasil diekspor langsung. Export frame berikutnya ditolak karena batas panggilan Figma MCP plan Starter. Export seluruh frame, raw images, dan SVG Figma belum selesai.
- Sebutan **belum terlihat** berarti belum ditemukan dalam inventaris atau gambar yang diperiksa. Status tersebut perlu dikonfirmasi jika tim memiliki rancangan lain di luar bukti ini.

## Cakupan yang sudah tergambar

| Kelompok | Bukti yang terlihat | Gambar pengguna |
|---|---|---|
| Onboarding | Splash, tiga pengenalan fitur, skip, dan tombol mulai. | 1 |
| Authentication | Login email/password, registrasi pengguna, registrasi mitra, permintaan reset, cek email, dan password baru. | 2 |
| Beranda dan mitra | Saldo, ringkasan aktivitas, misi, rekomendasi mitra, pencarian/filter peta, favorit, detail lokasi, kontak, dan navigasi. | 3 |
| Scan sampah dan pemanfaatan kembali | Kamera/galeri, hasil material, panduan penanganan, ide reuse, alat/bahan, langkah, peringatan penggunaan alat, dan pencarian mitra. | 3-4 |
| Klaim setoran | Kamera scan QR serta bukti setoran berhasil yang menampilkan poin, berat, mitra, ID transaksi, tanggal, dan status. | 4 |
| Reward | Katalog, biaya, kondisi saldo kurang, ketentuan reward, dan tombol konfirmasi penukaran. | 5 |
| Games | Daftar games, detail Pilah Cepat, pertanyaan, feedback benar/salah, XP, dan hasil akhir. Eco Quiz terlihat sebagai pilihan katalog. | 5-6 |
| Konten dan peringkat | Kalkulator pengguna, daftar/detail artikel, serta leaderboard setoran dan games. | 7 |
| Akun dan aktivitas | Daftar/detail riwayat, profil, pengaturan notifikasi, bantuan, laporan aktivitas dan unduh PDF, laporan masalah, serta masukan. | 8-9 |
| Mitra | Beranda mitra, input berat, estimasi poin, generate QR, countdown, tombol buat ulang, dan status menunggu pengguna. | 10 |

## Alur yang belum lengkap dalam bukti

Urutan berikut merupakan usulan prioritas pembahasan, bukan perubahan scope yang sudah disetujui.

| Prioritas | Bagian | Hal yang belum terlihat atau belum jelas |
|---|---|---|
| P1 | Klaim QR | Layar preview detail sebelum klaim, konfirmasi/batal, QR tidak valid/kedaluwarsa/sudah dipakai, serta pemeriksaan status ketika hasil request belum diketahui. PDF sudah menjelaskan kebutuhan beberapa tahap ini. |
| P1 | Penukaran reward | Hasil berhasil/gagal/menunggu, koleksi Reward Saya, detail voucher yang dimiliki, cara memakai voucher, serta penanganan request berulang. Tombol konfirmasi tersedia, tetapi pemetaan dialog dan kelanjutannya belum terlihat. |
| P1 | Registrasi dan kelayakan mitra | Form nama/kategori usaha tersedia; pengisian alamat, koordinat, jam operasional, material diterima, serta layar pending/rejected/suspended belum terlihat. Mekanisme verifikasi mitra belum diketahui. |
| P1 | Data setoran | Input mitra yang terlihat hanya berat. Pemilihan jenis/material, satu atau beberapa jenis per setoran, penerimaan/penolakan fisik, serta pemeriksaan detail sebelum menerbitkan QR perlu dijelaskan. |
| P1 | Kondisi sistem | Belum terlihat state khusus loading, kosong, offline, layanan AI gagal, izin kamera/lokasi ditolak, serta akun dibatasi pada alur masing-masing. Tidak semuanya harus menjadi halaman baru, tetapi perilakunya perlu ditentukan. |
| P2 | Gamifikasi | Detail misi, pemenuhan dan klaim hadiah, pencapaian/badge, serta gameplay Eco Quiz belum terlihat sebagai layar khusus. |
| P2 | Akun | Tujuan Edit Profil dan Privasi & Keamanan, pengaturan visibilitas leaderboard, serta daftar/detail notifikasi belum terlihat sebagai layar khusus. |
| P2 | Mitra | Riwayat setoran milik mitra dan status transaksi setelah pengguna mengklaim QR belum terlihat sebagai layar khusus. |
| P2 | Bantuan dan laporan | Konfirmasi kirim, gagal kirim, validasi form, serta loading/gagal saat membuat dan mengunduh PDF perlu ditentukan. |

Frame `200:184` memuat enam variasi pop-up generik: tiga warna, masing-masing dengan satu atau dua tombol. Keberadaannya menyediakan bahan dialog, tetapi belum menjelaskan pesan, aksi, dan perpindahan state per fitur. [Lihat export](hi-fi/popup-serbaguna-200-184.png).

## Catatan angka dan konsistensi desain

1. **Durasi QR:** Gambar 10 menampilkan countdown `04:59`. PDF menetapkan umur kurang dari 10 menit. Belum diketahui apakah angka UI menunjukkan sisa waktu atau durasi awal berbeda.
2. **Tarif:** Gambar 7 memberi contoh `100 poin/kg`; gambar 10 memakai contoh 2,50 kg untuk 250 poin. Angka ini merupakan data contoh sampai aturan tarif disepakati. PDF menyebut tarif berdasarkan jenis sampah, sementara UI mitra belum memperlihatkan pemilihan jenis dan DBML percakapan belum menghubungkan tarif/setoran ke kategori.
3. **Kalkulator pengguna:** Gambar 7 menampilkan input 2,50 kg dan 250 poin, tetapi keterangan kecil menyebut 2,6 kg. Nilai contoh perlu diselaraskan.
4. **Riwayat:** Gambar 8 memberi label Poin Keluar pada angka positif dan negatif. Contoh penukaran menyebut 200 EcoPoints dalam deskripsi, sementara angka di sisi kanan adalah -250. Data mockup tidak digunakan sebagai aturan ledger.
5. **Kategori misi:** Beranda mencontohkan target jumlah setoran; halaman Games mencontohkan setoran tiga jenis berbeda. Keduanya bisa menjadi misi terpisah, tetapi DBML yang dikirim belum mempunyai mission type untuk jumlah kategori berbeda.
6. **Perbedaan EcoPoints dan XP:** Gambar 5 menyatakan XP mini games untuk pencapaian/leaderboard Games, terpisah dari saldo EcoPoints setoran. Misi pada halaman yang sama mencontohkan hadiah +50 EcoPoints. Sumber bonus yang boleh masuk ke saldo dan dasar leaderboard perlu ditetapkan.
7. **Leaderboard setoran:** Gambar 7 menjelaskan skor dari poin transaksi setoran terverifikasi selama periode terpilih. Saldo tersisa setelah penukaran tidak disebut sebagai dasar peringkat. Kebutuhan ini perlu dibedakan dalam PRD.
8. **Mini game:** Contoh Pilah Cepat mempunyai lima soal dan 10 XP per jawaban benar, maksimal 50 XP. Hasil akhir menampilkan 2/5 dan +20 XP, tetapi tombol masih bertuliskan Soal Berikutnya. Tentukan aksi setelah permainan selesai dan aturan bermain ulang sebelum menetapkan perolehan XP.
9. **Reward demo:** Gambar 5 secara eksplisit menyatakan contoh katalog belum terhubung ke penyedia voucher atau pemenuhan sungguhan. Tampilan saldo e-wallet belum membuktikan adanya integrasi pembayaran atau pencairan.
10. **Navigasi:** Indikator Games terlihat aktif pada beberapa halaman di luar Games, seperti detail mitra, hasil scan, dan detail reward. Navigasi mitra juga memakai label QR dan Scan pada layar berbeda. Route dan tab aktif perlu diselaraskan.
11. **Registrasi mitra:** Judul masih Buat akun Pengguna ketika pilihan Mitra aktif. Judul dan penjelasan perlu mengikuti peran yang dipilih.
12. **Pengaturan notifikasi:** UI memperlihatkan notifikasi transaksi dan pengingat misi mingguan, sementara DBML hanya mempunyai satu `notification_enabled`. Tentukan apakah preferensi berlaku bersama atau dapat diatur terpisah.
13. **Kontak dukungan:** Form laporan/masukan memperbolehkan email kontak opsional. DBML `support_requests` belum menyediakan field email kontak terpisah. Tentukan apakah respons memakai email akun atau nilai yang diisi pengguna.
14. **Onboarding:** Layar muncul sebelum login. DBML menyimpan `onboarding_completed` per akun. Perilaku untuk instalasi baru, pengguna yang belum login, dan perpindahan perangkat masih perlu dijelaskan.

## Status penyimpanan

- Sepuluh PNG gabungan pengguna disalin tanpa perubahan ke `user-provided/`; ukuran, dimensi, dan checksum tersimpan dalam `user-image-manifest.json`.
- Inventaris node tiga page tersimpan dalam `node-inventory.json`.
- Export langsung yang berhasil tersimpan di `hi-fi/popup-serbaguna-200-184.png` dan dicatat dalam `download-manifest.json`.
- Gambar PDF tetap tersedia pada direktori sumber ekstraksi sebelumnya. Gambar tersebut diberi asal PDF dan tidak dinyatakan sebagai export Figma terbaru.
- PRD final, schema, kode backend, dan desain Figma belum diubah oleh pemeriksaan ini.
