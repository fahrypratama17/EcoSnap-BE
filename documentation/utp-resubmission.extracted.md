# EcoSnap - Ekstraksi laporan UTP resubmission

- Sumber: `Kelompok 8_Muhamad Fahry Pratama Putra_245150207111046_Resubmission_UTP (1).pdf`.
- Jumlah halaman: 33.
- SHA-256 PDF: `2d07808897cab2762ddd848b9215ff42eb8a2b9d17a839915313827964459f1e`.
- Metode: ekstraksi text layer per halaman dengan PyMuPDF; gambar dokumen diekstrak pada resolusi aslinya.
- Teks sumber dipertahankan, termasuk contoh template, salah ketik, dan penomoran yang belum konsisten. Karakter zero-width dihapus untuk memudahkan pencarian.
- Blok teks mempertahankan tata letak hasil ekstraksi, bukan format tabel semantik. Gambar belum ditranskripsikan dengan OCR; isi diagram dan mockup tersedia melalui gambar di setiap halaman.
- Header/footer bergambar yang berulang tidak disalin. Gambar cover, diagram, dan mockup disertakan.
- Dokumen ini merupakan referensi sumber untuk penyusunan PRD. Isi dokumen belum seluruhnya menjadi requirement yang disepakati.

## Halaman 01

```text
TEMPLATE LAPORAN PROJECT AKHIusR PRAKTIKUM PENGEMBANGAN
              APLIKASI PERANGKAT BERGERAK TIF - C

                               EcoSnap





                          Dosen Pengampu:
                             Fais Al Huda, S.Kom., M.Kom.



                 Disusun Oleh: Kelompok [Nomor Kelompok]

1. Muhamad Fahry Pratama Putra - 245150207111046 - Foto Sampah dan Peta Mitra
              2. Muhammad Ghazy Humaidi - 245150200111071 - Scan QR
                    3. Aditya Akbar - 245150200111028 - Generate QR
           4. Nafis Naufal Rahman - 245150200111008 - Reward / Tukar Poin



                               Asisten Praktikum:

                                    1. Johan Arizona
                                    2. Rahma Aliyyah



              PROGRAM STUDI TEKNIK INFORMATIKA
                 DEPARTEMEN TEKNIK INFORMATIKA
                       FAKULTAS ILMU KOMPUTER
                        UNIVERSITAS BRAWIJAYA
                                2026
```

![Gambar 1 pada halaman 1](assets/utp-resubmission/page-01-figure-01.png)

![Gambar 2 pada halaman 1](assets/utp-resubmission/page-01-figure-02.png)

![Gambar 3 pada halaman 1](assets/utp-resubmission/page-01-figure-03.png)

## Halaman 02

```text
DAFTAR ISI

DAFTAR ISI............................................................................................................................. 2
BAB I PENDAHULUAN...........................................................................................................3
    1.1. Latar Belakang...........................................................................................................3
    1.2. Tujuan Proyek............................................................................................................3
    1.3. Deskripsi Aplikasi..................................................................................................... 3
BAB II PERANCANGAN PROJECT DAN PEMBAGIAN TUGAS..........................................4
    2.1. Use Case Diagram (Keseluruhan Aplikasi).............................................................4
    2.2. Flowchart Sistem (Per Fungsi Utama).....................................................................5
    2.3. Konsep dan Desain Antarmuka (UI/UX)................................................................ 15
    2.4. Definisi Fungsi Utama (Problem Solving).............................................................15
    2.5. Pembagian tugas Anggota.....................................................................................17
    2.6. Screen Flow Aplikasi.............................................................................................. 17
BAB III IMPLEMENTASI TEKNIS......................................................................................... 18
    3.1. Implementasi Fungsi 1 : [Sistem Prediksi Restock Otomatis]............................18
    3.2. Implementasi Fungsi 2 : [Sistem Prediksi Restock Otomatis]............................18
BAB IV HASIL DAN PENGUJIAN........................................................................................ 18
    4.1. Demonstrasi Hasil Project......................................................................................18
    4.2. Link Repository dan Video Demo..........................................................................18
    4.3. Kendala dan Solusi................................................................................................. 18
BAB V PENUTUP..................................................................................................................18
    5.1. Kesimpulan..............................................................................................................18
    5.2. Saran Pengembangan.............................................................................................18
```

## Halaman 03

```text
BAB I PENDAHULUAN

1.1.   Latar Belakang (Aditya Akbar)

       Pengelolaan sampah masih menjadi salah satu permasalahan lingkungan yang membutuhkan
perhatian dan partisipasi masyarakat. Salah satu kendalanya adalah keterbatasan pemahaman
mengenai cara mengidentifikasi, memilah, dan menangani sampah berdasarkan jenis materialnya.
Setiap jenis sampah, seperti plastik, kertas, kaca, logam, dan organik, memiliki karakteristik serta
metode penanganan yang berbeda. Kurangnya pemahaman tersebut dapat menyebabkan sampah
yang masih memiliki potensi untuk didaur ulang tercampur dengan sampah lainnya sehingga
menyulitkan proses pengolahan lebih lanjut.
        Selain itu, masyarakat sering kali mengalami kesulitan dalam menentukan tindakan yang
tepat  setelah memilah sampah, termasuk mengetahui cara membersihkan, menyimpan, atau
memanfaatkan kembali material tertentu. Keterbatasan informasi mengenai lokasi bank sampah,
jenis material yang diterima, serta prosedur penyetoran juga menjadi hambatan dalam menyalurkan
sampah yang masih memiliki nilai guna. Di sisi lain, diperlukan pendekatan yang dapat mendorong
masyarakat untuk berpartisipasi secara konsisten dalam kegiatan pengelolaan sampah.
      Perkembangan teknologi Artificial Intelligence (AI), khususnya Computer Vision, membuka
peluang untuk mempermudah identifikasi sampah melalui analisis gambar. Dengan mengintegrasikan
teknologi tersebut bersama layanan pemetaan digital, edukasi, dan sistem insentif, dikembangkan
EcoSnap, sebuah aplikasi mobile yang membantu pengguna mengenali jenis sampah, memperoleh
panduan penanganan, menemukan lokasi penyetoran yang sesuai, serta mendapatkan penghargaan
melalui aktivitas penyetoran sampah yang terverifikasi.

1.2.   Tujuan Proyek (Nafis Naufal Rahman)

       Tujuan utama pengembangan EcoSnap  adalah menyediakan  aplikasi mobile berbasis
Artificial Intelligence yang mempermudah masyarakat dalam mengenali, memilah, menangani, dan
menyetorkan sampah secara lebih tepat dan terarah.
Secara khusus, aplikasi EcoSnap bertujuan untuk:
     1. Memanfaatkan teknologi Computer Vision untuk mengidentifikasi jenis dan kategori material
      sampah berdasarkan gambar yang diambil melalui kamera smartphone.
     2. Memberikan  panduan  penanganan  sampah,   meliputi   pemilahan,  pembersihan,
       penyimpanan,  serta rekomendasi pemanfaatan kembali atau daur ulang berdasarkan
         karakteristik material.
     3. Membantu pengguna menemukan lokasi bank sampah atau mitra pengumpulan melalui
       layanan peta digital berdasarkan lokasi pengguna dan jenis material yang diterima.
     4. Memfasilitasi proses penyetoran sampah melalui sistem pencatatan dan verifikasi transaksi
      menggunakan kode QR, termasuk  informasi  jenis  material dan  berat sampah yang
        disetorkan.
     5. Mendorong partisipasi masyarakat melalui sistem poin dan penukaran reward dari aktivitas
       penyetoran sampah yang berhasil diverifikasi.
     6. Meningkatkan pengetahuan dan kepedulian masyarakat terhadap pengelolaan sampah
        melalui permainan edukatif dan fitur interaktif.
```

## Halaman 04

```text
1.3.   Deskripsi Aplikasi (Muhamad Fahry Pratama Putra)

      EcoSnap merupakan  aplikasi  Android  berbasis  Kotlin yang berfungsi sebagai  asisten
pengelolaan sampah  pribadi  sekaligus  platform  penghubung  antara  masyarakat dan  mitra
pengumpulan sampah. Aplikasi  ini mengintegrasikan teknologi  Artificial  Intelligence, Computer
Vision, layanan peta digital, serta sistem penyetoran dan penghargaan berbasis poin.
         Fitur utama EcoSnap adalah identifikasi sampah melalui foto. Pengguna dapat mengambil
gambar sampah menggunakan kamera smartphone, kemudian AI menganalisis gambar untuk
memprediksi jenis material dan kategori sampah. Hasil identifikasi disertai informasi karakteristik
material, panduan penanganan, serta rekomendasi pemanfaatan kembali atau ide daur ulang.
Pengguna dapat mengonfirmasi hasil identifikasi sebelum melanjutkan ke proses berikutnya.
        Setelah mengetahui  jenis material, pengguna dapat mencari lokasi mitra pengumpulan
melalui fitur peta digital. Aplikasi menampilkan informasi lokasi, jenis sampah yang diterima, jam
operasional, serta petunjuk navigasi untuk membantu pengguna menentukan tempat penyetoran
yang sesuai. EcoSnap juga menyediakan fitur penyimpanan informasi sampah yang ingin disetorkan
dan riwayat pemindaian.
       Proses penyetoran dilakukan secara langsung melalui mitra pengumpulan. Mitra bertugas
memeriksa sampah, mencatat jenis material dan berat setoran, serta melakukan verifikasi transaksi
menggunakan kode QR. Setelah transaksi berhasil diverifikasi, sistem mencatat riwayat penyetoran
dan memberikan poin kepada pengguna sesuai ketentuan. Poin yang terkumpul dapat ditukarkan
dengan berbagai reward yang tersedia di dalam aplikasi.
        Selain fitur tersebut, EcoSnap menyediakan permainan edukatif untuk membantu pengguna
memahami konsep pemilahan dan pengelolaan sampah secara interaktif. Aplikasi juga menyediakan
riwayat  aktivitas agar pengguna dapat memantau hasil pemindaian, transaksi penyetoran, dan
perolehan poin.
      Dengan menghubungkan proses  identifikasi, edukasi, pencarian lokasi, penyetoran, dan
pemberian insentif dalam satu aplikasi, EcoSnap diharapkan dapat mempermudah masyarakat dalam
menerapkan praktik pengelolaan sampah yang lebih bertanggung jawab serta mendorong partisipasi
dalam kegiatan penggunaan kembali dan daur ulang.
```

## Halaman 05

```text
BAB II PERANCANGAN PROJECT DAN PEMBAGIAN TUGAS

2.1.   Use Case Diagram (Keseluruhan Aplikasi) (Muhammad Ghazy Humaidi)

     Berikut  ini merupakan use case diagram yang digunakan dalam pembuatan aplikasi kami.
Detailnya dapat dilihat melalui gambar berikut ini:





    Diagram use case EcoSnap menggambarkan interaksi dua aktor utama, yaitu pengguna biasa
dan mitra atau bank sampah. Persona pengguna biasa adalah masyarakat yang membutuhkan
bantuan untuk mengenali jenis sampah, memahami cara penanganan dan pemanfaatannya, serta
menemukan lokasi penyetoran yang sesuai. Pengguna memperoleh EcoPoints dari setoran sampah
yang terverifikasi dan dapat mengikuti kegiatan edukasi serta gamifikasi dalam aplikasi. Sementara
itu, persona mitra adalah petugas bank sampah atau pengelola lokasi penerimaan sampah yang
bertanggung jawab memeriksa setoran, mencatat data aktual sampah, dan menyediakan QR
```

![Gambar 1 pada halaman 5](assets/utp-resubmission/page-05-figure-01.png)

## Halaman 06

```text
transaksi sebagai dasar klaim poin pengguna. Kedua aktor dapat melakukan registrasi, login, dan
logout sesuai peran masing-masing.
    Pengguna biasa dapat menggunakan fitur Scan Sampah untuk memperoleh identifikasi berbasis
AI, informasi penanganan, panduan pemanfaatan, dan rekomendasi lokasi mitra. Pengguna dapat
mencari mitra melalui  peta, melihat  detailnya, menyimpan  favorit, mengakses kontak, serta
mendapatkan petunjuk  arah.  Aplikasi menyediakan  kalkulator  estimasi poin sebagai simulasi
sebelum penyetoran. Setelah menyetorkan sampah, pengguna memindai QR dari mitra, memeriksa
detail  transaksi, dan mengonfirmasi klaim. Sistem memvalidasi klaim sebelum menambahkan
EcoPoints ke akun pengguna. Pengguna dapat melihat saldo, mengakses katalog dan koleksi reward,
serta menukarkan poin setelah memenuhi ketentuan penukaran. Fitur lainnya mencakup riwayat
aktivitas, laporan yang dapat diunduh, artikel edukasi, misi mingguan, mini games, serta leaderboard
setoran dan games. XP dari mini games dicatat terpisah dari EcoPoints yang digunakan untuk
penukaran reward. Pengguna juga dapat mengelola profil, melihat pencapaian, mengatur preferensi
akun dan notifikasi, membaca bantuan, serta mengirim laporan masalah atau masukan.
     Mitra dapat mengakses fitur pengelolaan setoran setelah melakukan login. Mitra memeriksa
kondisi  fisik sampah dan menyetujui atau menolak setoran berdasarkan ketentuan penerimaan.
Untuk sampah yang diterima, mitra mencatat jenis dan berat aktual, kemudian sistem menghitung
poin berdasarkan aturan yang berlaku. Hasil identifikasi AI dapat digunakan sebagai referensi awal
dan dikoreksi apabila berbeda dengan pemeriksaan fisik. Setelah persetujuan, pencatatan data, dan
perhitungan poin selesai, mitra dapat menghasilkan QR transaksi. Sistem memeriksa kelayakan
penerbitan QR agar transaksi memenuhi persyaratan klaim. Mitra dapat membuat ulang QR untuk
transaksi yang belum diklaim dengan menonaktifkan QR sebelumnya. Selain itu, mitra dapat melihat
riwayat penyetoran di lokasinya untuk memantau transaksi yang telah dilakukan.

2.2.   Flowchart Sistem (Per Fungsi Utama)

2.2.1 Fitur Scan QR (Muhammad Ghazy Humaidi)
Dalam penggunaan fitur Scan QR, penulis melakukan pengelompokkan terhadap 3 alur yaitu, alur
saat Akses Kamera, alur saat Validasi QR, dan alur saat konfirmasi & klaim hasil. Untuk detail dari
masing masing alur tersebut dapat dilihat melalui gambar berikut ini:


2.2.1.1 Akses dan Kamera
```

## Halaman 07

```text
Alur akses dan kamera bertujuan memastikan bahwa pengguna memiliki sesi login yang valid
serta  izin untuk menggunakan kamera sebelum melakukan pemindaian QR. Pemeriksaan  ini
diperlukan karena poin  hasil penyetoran harus masuk ke akun pengguna yang terautentikasi,
sedangkan proses pemindaian membutuhkan akses kamera perangkat. Alur ini juga menyediakan
penanganan ketika pengguna belum login, menolak izin kamera, atau belum berhasil memindai QR.
     Secara umum, proses dimulai dengan pemeriksaan sesi pengguna. Apabila sesi masih valid,
pengguna dapat membuka halaman poin dan memilih fitur Scan QR. Apabila sesi tidak valid,
pengguna harus melakukan login terlebih dahulu. Setelah itu, aplikasi memeriksa izin kamera dan
meminta izin apabila belum tersedia. Kamera kemudian diaktifkan agar pengguna dapat memindai
QR transaksi yang diberikan mitra. QR yang berhasil dibaca akan diteruskan ke tahap validasi
transaksi pada gambar kedua.
     Secara spesifik, hasil pemeriksaan sesi menentukan apakah pengguna langsung mengakses fitur
atau menjalankan subproses  login. Subproses  login mencakup percobaan  login, penanganan
kegagalan, serta pilihan mengulangi atau membatalkan. Jika login akhirnya berhasil, pengguna
melanjutkan ke halaman poin, jika pengguna membatalkan tanpa berhasil login, alur berakhir.
Penghubung S merupakan jalur masuk kembali dari pilihan Scan Ulang pada gambar ketiga menuju
halaman poin dan fitur Scan QR. Jika izin kamera sudah tersedia, aplikasi langsung mengaktifkan
kamera. Jika belum tersedia, aplikasi meminta izin: persetujuan mengaktifkan kamera, sedangkan
penolakan menampilkan pesan bahwa izin kamera diperlukan dan mengarahkan pengguna melalui
penghubung N menuju pilihan navigasi pada gambar ketiga. Selanjutnya, apabila QR belum terbaca,
aplikasi meminta pengguna memperbaiki posisinya, kemudian penghubung K mengembalikan proses
ke kamera untuk melanjutkan pemindaian. Apabila QR berhasil terbaca, penghubung B meneruskan
proses ke gambar kedua.


2.2.1.2  Validasi QR
```

![Gambar 1 pada halaman 7](assets/utp-resubmission/page-07-figure-01.png)

## Halaman 08

```text
Alur  validasi QR bertujuan memastikan bahwa QR yang dipindai merujuk pada transaksi
penyetoran yang sah dan memenuhi ketentuan pemberian poin. Pemeriksaan ini digunakan untuk
mencegah klaim melalui QR yang tidak sesuai, transaksi yang belum disetujui mitra, QR yang
kedaluwarsa, atau transaksi yang sudah pernah diklaim. QR berfungsi sebagai referensi untuk
mengambil data transaksi dari sistem, sehingga jumlah poin tidak langsung dipercaya dari isi QR
semata.
     Secara umum, aplikasi memeriksa format QR, mengecek koneksi, dan mengambil transaksi
berdasarkan referensi yang diperoleh. Sistem kemudian memeriksa keabsahan transaksi, persetujuan
setoran, masa berlaku QR, serta status klaimnya. Apabila seluruh ketentuan terpenuhi, aplikasi
menampilkan detail transaksi agar pengguna dapat memeriksanya sebelum melakukan konfirmasi.
Jika terjadi gangguan atau ketentuan tidak terpenuhi, aplikasi menampilkan informasi kegagalan dan
menyediakan jalur untuk mencoba kembali atau berpindah halaman.
     Secara spesifik, penghubung B menerima hasil pemindaian dari gambar pertama. Jika format
QR tidak sesuai, aplikasi menampilkan pesan QR tidak valid dan meneruskan pengguna melalui
penghubung N menuju pilihan navigasi pada gambar ketiga. Jika format sesuai, aplikasi memeriksa
koneksi dan mengambil transaksi. Kegagalan pengambilan data akibat gangguan koneksi atau
layanan menghasilkan pesan kesalahan serta pilihan mencoba kembali. Pilihan Ya mengarahkan
proses melalui penghubung R untuk mengulangi pemeriksaan koneksi dan pengambilan transaksi,
sedangkan pilihan Tidak menuju N. Jika data berhasil diambil, sistem memeriksa apakah transaksi
ditemukan dan sah, setoran telah disetujui mitra, usia QR masih kurang dari 10 menit sejak dibuat,
serta transaksi belum diklaim. QR yang mencapai usia tepat 10 menit sudah dianggap kedaluwarsa.
Kegagalan salah satu pemeriksaan menampilkan alasan yang sesuai dan mengarahkan pengguna ke
N. Apabila seluruh syarat terpenuhi, aplikasi menampilkan nama mitra, jenis sampah, berat, jumlah
poin, dan waktu transaksi, lalu penghubung C meneruskan proses ke gambar ketiga. Pada tahap ini,
QR belum digunakan dan poin belum ditambahkan.
```

![Gambar 1 pada halaman 8](assets/utp-resubmission/page-08-figure-01.png)

## Halaman 09

```text
2.2.1.1  Konfirmasi & Hasil Klaim





     Alur konfirmasi dan hasil klaim bertujuan memberikan kesempatan kepada pengguna untuk
menyetujui transaksi sebelum poin ditambahkan ke akunnya. Alur  ini juga memastikan bahwa
penerima klaim, penggunaan QR, penambahan poin, dan pencatatan riwayat diproses secara
konsisten. Pemeriksaan ulang diperlukan karena QR dapat kedaluwarsa atau diklaim pengguna lain
setelah detail transaksi ditampilkan. Penanganan hasil yang belum diketahui juga diperlukan agar
gangguan koneksi tidak menyebabkan pemberian poin berulang.
     Secara umum, pengguna menerima detail transaksi dari tahap sebelumnya, kemudian memilih
mengonfirmasi atau membatalkan klaim. Jika pengguna mengonfirmasi, aplikasi memeriksa koneksi
dan mengirim permintaan klaim menggunakan sesi pengguna yang sedang login. Sistem memvalidasi
kembali kelayakan transaksi,  lalu mengikat transaksi ke akun penerima, menandai QR sebagai
terpakai, menambahkan poin, dan menyimpan riwayat sebagai satu transaksi. Hasilnya dapat berupa
keberhasilan, penolakan atau kegagalan, maupun hasil yang belum diketahui. Setelah menerima
hasil atau memilih keluar, pengguna dapat melakukan scan ulang, membuka halaman poin, atau
membuka halaman utama.
     Secara spesifik, penghubung C menerima pengguna yang telah melihat detail transaksi pada
gambar kedua. Jika pengguna membatalkan, QR tidak digunakan dan saldo tidak berubah; proses
kemudian menuju penghubung N. QR tersebut tetap dapat diklaim selama belum kedaluwarsa dan
belum digunakan pengguna lain. Jika pengguna mengonfirmasi tetapi koneksi tidak tersedia, aplikasi
menampilkan gangguan koneksi. Pilihan mencoba kembali menuju penghubung T untuk mengulangi
pemeriksaan koneksi, sedangkan pilihan tidak mencoba kembali menuju N. Jika koneksi tersedia,
sistem memvalidasi ulang sesi, keabsahan dan persetujuan transaksi, umur QR, serta status klaim.
```

![Gambar 1 pada halaman 9](assets/utp-resubmission/page-09-figure-01.png)

## Halaman 10

```text
Pengguna  pertama  yang  berhasil menyimpan  klaim  menjadi  penerima  poin.  Keberhasilan
menampilkan bukti transaksi dan saldo terbaru, kemudian menuju N; penolakan atau kegagalan yang
sudah dipastikan menampilkan alasannya, kemudian juga menuju N. Jika hasil belum diketahui,
aplikasi menjalankan subproses pemeriksaan status, dengan pilihan memeriksa kembali atau keluar.
Status berhasil mengarahkan pengguna ke bukti transaksi; status pasti belum diklaim menuju T
untuk mencoba kembali melalui validasi; penolakan atau klaim oleh akun lain menuju penghubung D
untuk menampilkan alasan kegagalan; sedangkan pilihan keluar menuju N. Pada pilihan navigasi,
Scan Ulang menggunakan penghubung S untuk kembali ke gambar pertama, sementara pilihan
halaman poin atau halaman utama membuka halaman yang dipilih dan mengakhiri alur.


2.2.2 Peta Mitra (Muhamad Fahry Pratama Putra)
      Pada fitur Peta Mitra, aplikasi EcoSnap menyediakan layanan pencarian lokasi bank sampah
atau mitra pengumpulan sampah yang dapat membantu pengguna menemukan tempat penyetoran
sampah sesuai dengan kebutuhannya. Untuk menggambarkan proses kerja fitur tersebut, kami
membuat  flowchart yang mencakup  alur pemeriksaan  izin  lokasi, pengambilan data  mitra,
penyaringan berdasarkan jenis sampah, hingga penampilan informasi detail dan petunjuk navigasi
menuju lokasi mitra. Flowchart ini bertujuan untuk memberikan gambaran mengenai tahapan proses
serta pengambilan keputusan yang terjadi selama pengguna mengakses fitur Peta Mitra. Alur
lengkap dari proses tersebut dapat dilihat pada gambar berikut.
```

## Halaman 11

```text
Alur Peta Mitra bertujuan untuk membantu pengguna menemukan lokasi bank sampah atau
mitra pengumpulan sampah terdekat berdasarkan lokasi pengguna dan jenis sampah yang ingin
disetorkan. Fitur ini menyediakan informasi lokasi mitra, detail mitra, serta petunjuk navigasi menuju
lokasi yang dipilih.
       Secara umum, proses dimulai ketika pengguna membuka halaman Peta Mitra. Sistem akan
memeriksa apakah akses lokasi sudah tersedia. Jika belum tersedia, aplikasi akan meminta izin lokasi
kepada pengguna. Apabila  izin diberikan, sistem mengambil koordinat lokasi pengguna secara
otomatis. Namun, jika izin ditolak, pengguna tetap dapat menggunakan fitur dengan memasukkan
lokasi secara manual. Selanjutnya, sistem mengambil data mitra berdasarkan lokasi dan jangkauan
pencarian yang telah ditentukan.
        Setelah data mitra diperoleh, pengguna dapat memilih filter berdasarkan jenis sampah untuk
mempersempit  hasil pencarian. Apabila mitra ditemukan, aplikasi akan menampilkan lokasinya
dalam bentuk pin pada peta. Pengguna kemudian dapat memilih salah satu pin untuk melihat
informasi detail mitra. Sebaliknya, jika tidak ditemukan mitra yang sesuai, aplikasi akan menampilkan
```

![Gambar 1 pada halaman 11](assets/utp-resubmission/page-11-figure-01.png)

## Halaman 12

```text
pesan bahwa mitra tidak ditemukan dan memberikan pilihan untuk mengubah filter atau lokasi
pencarian.
       Secara khusus, setelah informasi detail mitra ditampilkan, pengguna dapat memilih untuk
membuka petunjuk navigasi menuju lokasi mitra. Jika pengguna memilih navigasi, aplikasi akan
membuka rute menuju lokasi tersebut. Namun, apabila pengguna tidak memilih navigasi, proses
dapat langsung diakhiri. Dengan demikian, fitur Peta Mitra mempermudah pengguna dalam mencari
dan menentukan lokasi penyetoran sampah yang sesuai dengan kebutuhannya.


2.2.3 Fitur Foto Sampah (Muhamad Fahry Pratama Putra)
      Pada fitur Foto Sampah, aplikasi EcoSnap menyediakan layanan identifikasi jenis sampah
menggunakan teknologi Artificial Intelligence (AI) untuk membantu pengguna mengenali sampah
dan mengetahui cara penanganannya. Untuk menggambarkan proses kerja fitur tersebut, dibuat
flowchart yang mencakup  alur pengambilan atau pemilihan foto sampah, proses  identifikasi
menggunakan  AI, pemeriksaan keberhasilan  identifikasi, hingga penampilan hasil berupa jenis
sampah, saran penanganan, dan rekomendasi mitra. Flowchart ini bertujuan untuk memberikan
gambaran mengenai tahapan proses serta pengambilan keputusan yang terjadi selama pengguna
menggunakan fitur Foto Sampah. Alur lengkap dari proses tersebut dapat dilihat pada gambar
berikut.





       Secara umum, proses dimulai ketika pengguna membuka fitur Foto Sampah pada aplikasi
EcoSnap. Selanjutnya, pengguna dapat mengambil gambar sampah secara langsung menggunakan
kamera perangkat atau memilih gambar yang sudah tersimpan di galeri. Foto yang telah dipilih
kemudian dikirimkan ke sistem untuk diproses menggunakan teknologi Artificial Intelligence (AI) yang
bertugas mengidentifikasi jenis sampah berdasarkan gambar tersebut.
```

![Gambar 1 pada halaman 12](assets/utp-resubmission/page-12-figure-01.png)

## Halaman 13

```text
Setelah proses  identifikasi dilakukan, sistem akan memeriksa apakah sampah berhasil
dikenali atau tidak. Apabila  identifikasi berhasil, aplikasi akan menampilkan hasil berupa jenis
sampah yang terdeteksi, saran penanganan yang sesuai, serta rekomendasi mitra atau bank sampah
yang dapat menerima sampah  tersebut. Informasi  ini bertujuan untuk membantu pengguna
memahami karakteristik sampah dan menentukan langkah pengelolaan yang tepat.
       Secara khusus, apabila sistem tidak berhasil mengenali sampah pada gambar, aplikasi akan
mengarahkan  pengguna  untuk  mengambil  foto  ulang.  Pengguna  kemudian  dapat  kembali
menggunakan kamera atau memilih gambar lain dari galeri agar proses identifikasi dapat dilakukan
kembali. Proses ini dapat diulangi hingga sistem berhasil mengenali sampah. Setelah hasil identifikasi
berhasil ditampilkan, alur penggunaan fitur Foto Sampah dinyatakan selesai.


2.2.4 Fitur Reward dan Tukar Poin (Nafis Naufal Rahman)
      Dalam fitur Reward dan Tukar Poin, pengguna EcoSnap dapat melihat saldo poin hasil klaim
transaksi penyetoran sampah, menelusuri katalog reward, serta mengajukan penukaran sesuai
jumlah poin yang dimiliki. Fitur ini menghubungkan aktivitas penyetoran sampah dengan insentif
bagi pengguna tanpa mengubah proses verifikasi setoran yang menjadi tanggung jawab mitra.
        Alur dimulai dengan pemeriksaan sesi login. Setelah pengguna membuka halaman Reward,
aplikasi mengambil saldo poin terbaru dan daftar reward beserta biaya poin, ketersediaan, dan
ketentuan penukarannya. Pengguna memilih reward dan sistem melakukan pemeriksaan awal
terhadap status reward serta kecukupan saldo. Reward yang tidak tersedia atau membutuhkan poin
lebih banyak tidak dapat dilanjutkan ke tahap penukaran.
        Apabila  persyaratan awal  terpenuhi,  aplikasi menampilkan  detail reward dan  dialog
konfirmasi. Pembatalan  tidak mengubah  saldo.  Jika pengguna menyetujui,  aplikasi mengirim
permintaan  penukaran  ke  server.  Server  memvalidasi  ulang  identitas  pengguna,  saldo,
status/ketersediaan reward, dan permintaan transaksi sebelum memproses pengurangan poin dan
pencatatan penukaran secara atomik. Dengan demikian, kegagalan proses tidak menyebabkan saldo
berkurang tanpa riwayat penukaran yang sah.
```

## Halaman 14

```text
Apabila penukaran  berhasil,  saldo  poin pada  aplikasi  diperbarui, riwayat penukaran
ditambahkan, dan pengguna memperoleh bukti atau status penukaran. Apabila validasi gagal,
misalnya poin sudah digunakan pada transaksi lain, reward tidak lagi tersedia, atau terjadi gangguan
jaringan, sistem menampilkan pesan kegagalan dan meminta pengguna memperbarui data sebelum
mencoba kembali. Permintaan berulang perlu dicegah menggunakan identitas transaksi unik agar
poin tidak terpotong dua kali.


2.2.5 Generate QR (Aditya Akbar)
```

![Gambar 1 pada halaman 14](assets/utp-resubmission/page-14-figure-01.png)

## Halaman 15

```text
Pada fitur Generate QR aplikasi EcoSnap, penulis membagi proses menjadi dua alur utama,
yaitu alur akses dan perhitungan poin serta alur verifikasi dan pembuatan QR Code. Pembagian ini
bertujuan untuk mempermudah pemahaman mengenai proses pembuatan QR Code transaksi yang
dilakukan oleh mitra berdasarkan jenis dan berat sampah yang disetorkan pengguna.





        Alur akses dan perhitungan poin bertujuan untuk memfasilitasi mitra dalam memasukkan
data sampah serta mengetahui jumlah poin yang akan diberikan kepada pengguna. Proses ini
diperlukan agar perhitungan poin dilakukan berdasarkan  jenis dan berat sampah yang telah
disetorkan.
       Secara umum, proses dimulai ketika mitra melakukan login ke aplikasi EcoSnap, kemudian
membuka halaman Kalkulator Poin. Selanjutnya, mitra memasukkan jenis dan berat sampah yang
telah diterima. Berdasarkan data tersebut, sistem secara otomatis menghitung jumlah poin sesuai
dengan ketentuan yang berlaku dan menampilkan hasil perhitungannya.
       Secara khusus,  hasil perhitungan poin yang telah ditampilkan akan diteruskan melalui
penghubung A menuju alur verifikasi dan pembuatan QR Code pada gambar kedua. Selain itu,
penghubung B digunakan sebagai jalur kembali ke proses input data sampah apabila pada tahap
verifikasi ditemukan ketidaksesuaian data atau jumlah poin.
```

![Gambar 1 pada halaman 15](assets/utp-resubmission/page-15-figure-01.png)

## Halaman 16

```text
Alur verifikasi dan Generate QR bertujuan untuk memastikan bahwa data setoran sampah
dan jumlah poin telah sesuai sebelum sistem menghasilkan QR Code transaksi. Proses verifikasi
dilakukan untuk mengurangi kemungkinan kesalahan dalam pemberian poin kepada pengguna.
       Secara umum, proses dimulai dari penghubung A yang menerima hasil perhitungan poin dari
alur sebelumnya. Mitra kemudian memeriksa kesesuaian data setoran dan jumlah poin yang
ditampilkan. Apabila data sudah sesuai, mitra dapat melanjutkan proses dengan menekan tombol
Generate QR. Sistem kemudian membuat QR Code transaksi dan menampilkannya pada aplikasi. QR
Code tersebut selanjutnya diberikan kepada pengguna untuk dipindai melalui fitur Scan QR.
       Secara khusus, pemeriksaan kesesuaian data menghasilkan dua kemungkinan, yaitu sesuai
atau  tidak  sesuai.  Jika data belum sesuai, mitra melakukan perbaikan data sampah melalui
penghubung B yang mengarahkan proses kembali ke tahap input data pada gambar pertama.
Sebaliknya, jika data sudah sesuai, proses dilanjutkan dengan pembuatan QR Code transaksi. Setelah
QR Code berhasil ditampilkan dan diberikan kepada pengguna, proses Generate QR dinyatakan
selesai.
```

![Gambar 1 pada halaman 16](assets/utp-resubmission/page-16-figure-01.png)

## Halaman 17

```text
2.3.   Konsep dan Desain Antarmuka (UI/UX) (Muhamad Fahry Pratama Putra)

       Desain antarmuka  aplikasi EcoSnap dirancang menggunakan pendekatan modern dan
minimalis dengan mengadaptasi prinsip Material Design 3. Aplikasi kami mengusung konsep ramah
lingkungan dengan dominasi warna hijau tua, hijau sage, dan krem sebagai identitas visual utama.
Warna hijau merepresentasikan kepedulian terhadap lingkungan dan keberlanjutan, sedangkan
warna krem digunakan sebagai latar belakang agar tampilan terlihat bersih dan nyaman. Desain juga
menerapkan komponen berbentuk rounded card, ikon sederhana, tipografi yang konsisten, serta
hierarki visual yang jelas untuk memudahkan pengguna memahami informasi dan mengakses fitur
aplikasi.
        Dari segi pengalaman pengguna, EcoSnap dirancang dengan alur navigasi yang sederhana
dan mudah dipahami. Aplikasi diawali dengan Splash Screen dan tiga halaman Onboarding yang
memperkenalkan fitur utama, kemudian dilanjutkan dengan halaman Login dan Registrasi untuk
pengguna biasa maupun mitra. Pada halaman utama, aplikasi menggunakan Bottom Navigation Bar
yang terdiri atas lima menu, yaitu Beranda, Peta, Scan, Poin, dan Games. Setiap halaman memiliki
fungsi berbeda, mulai dari menampilkan informasi poin dan misi mingguan, mencari lokasi bank
sampah melalui peta, mengidentifikasi jenis sampah menggunakan AI, memindai QR transaksi,
hingga menukarkan poin dengan berbagai pilihan hadiah.
        Selain fitur untuk pengguna biasa, EcoSnap juga menyediakan antarmuka khusus mitra
berupa  Kalkulator Sampah dan Generate QR. Mitra dapat memasukkan berat sampah yang
disetorkan, kemudian sistem secara otomatis menghitung jumlah poin berdasarkan nilai konversi
yang berlaku. Setelah perhitungan selesai, mitra dapat menghasilkan QR Code transaksi untuk
dipindai oleh pengguna sebagai proses klaim poin. Secara keseluruhan, rancangan UI/UX EcoSnap
menekankan  konsistensi  tampilan, kemudahan  interaksi, dan keterhubungan  antarfitur  agar
pengguna dapat melakukan pengelolaan sampah secara lebih praktis. Seluruh rancangan antarmuka
dibuat menggunakan Figma sebagai purwarupa sebelum diimplementasikan ke dalam  aplikasi
Android berbasis Kotlin.
Link Figma:
https://www.figma.com/design/GcZxuR5nuaOH8GB1Lo37lW/EcoSnap?node-id=0-1&t=8YeKIIoAKZ03
x507-1

2.4.   Definisi Fungsi Utama (Problem Solving)

        Aplikasi EcoSnap memiliki lima fungsi utama yang dirancang untuk membantu pengguna
dalam mengenali, mengelola, dan menyetorkan sampah melalui pemanfaatan teknologi Artificial
Intelligence (AI), layanan pemetaan digital, serta sistem transaksi berbasis QR Code. Setiap fungsi
tidak hanya melakukan penyimpanan atau pengambilan data, tetapi juga memiliki proses pengolahan
informasi untuk menghasilkan keluaran yang bermanfaat bagi pengguna maupun mitra bank
sampah.

Contoh:

 ●  Fungsi 1: Sistem Identifikasi Jenis Sampah Berbasis AI (Foto Sampah) (Muhamad Fahry
    Pratama Putra)
     ○ Sumber Data (CRUD asal): Data foto sampah yang diambil melalui kamera atau galeri, data
          kategori dan jenis sampah, informasi penanganan sampah, serta riwayat hasil identifikasi
        pengguna.
```

## Halaman 18

```text
○  Nilai Tambah/Problem Solving: Sistem tidak hanya menyimpan gambar sampah yang
       diunggah oleh pengguna, tetapi juga memproses gambar tersebut menggunakan teknologi
          Artificial  Intelligence   (AI)  untuk  mengidentifikasi  jenis  sampah  secara  otomatis.
        Berdasarkan hasil identifikasi, sistem memberikan informasi mengenai kategori sampah,
       rekomendasi penanganan, serta kemungkinan pemanfaatan atau daur ulang sampah
         tersebut. Selain itu, hasil identifikasi dapat digunakan untuk memberikan rekomendasi
        mitra yang sesuai dengan jenis sampah yang terdeteksi. Dengan demikian, pengguna tidak
        perlu menentukan jenis sampah secara manual dan dapat memperoleh informasi yang
      membantu proses pengelolaan sampah.
●  Fungsi 2: Sistem Pencarian dan Rekomendasi Lokasi Mitra (Peta Mitra) (Muhamad Fahry
   Pratama Putra)
    ○ Sumber Data (CRUD  asal): Data mitra bank sampah, koordinat lokasi mitra, kategori
       sampah yang diterima, informasi operasional mitra, serta data lokasi pengguna.
    ○  Nilai Tambah/Problem Solving: Sistem tidak hanya menampilkan daftar lokasi mitra, tetapi
        juga mengolah  informasi  geografis untuk membantu pengguna menemukan tempat
       penyetoran  sampah  berdasarkan  lokasi  dan  jangkauan  pencarian.  Aplikasi  dapat
       menggunakan koordinat pengguna dan koordinat mitra untuk menentukan jarak serta
       merekomendasikan lokasi yang relevan. Pengguna juga dapat melakukan penyaringan
        berdasarkan jenis sampah yang diterima mitra sehingga hasil pencarian lebih sesuai dengan
        kebutuhan. Setelah mitra dipilih, sistem menampilkan informasi detail dan menyediakan
        akses petunjuk  navigasi menuju  lokasi  tersebut. Dengan demikian, pengguna dapat
       menentukan tempat penyetoran sampah secara lebih mudah dan efisien.
●  Fungsi 3: Sistem Validasi QR dan Klaim Poin Transaksi (Scan QR) (Muhammad Ghazy Humaidi)
    ○ Sumber Data (CRUD asal): Data pengguna, data mitra, QR Code transaksi, data berat
       sampah, jumlah poin  transaksi, status QR Code, saldo poin pengguna, dan riwayat
         transaksi.
    ○  Nilai Tambah/Problem Solving: Sistem tidak hanya melakukan pemindaian QR Code melalui
       kamera, tetapi juga memvalidasi data transaksi penyetoran sampah yang sebelumnya
        dibuat oleh mitra. Setelah QR Code berhasil dipindai, sistem memeriksa keabsahan kode,
      masa berlaku, serta status transaksi untuk memastikan bahwa poin belum pernah diklaim.
        Apabila transaksi dinyatakan valid, pengguna dapat melihat informasi berat sampah dan
       jumlah poin yang akan diterima sebelum melakukan konfirmasi. Setelah dikonfirmasi,
        sistem menambahkan poin ke saldo pengguna dan memperbarui status transaksi agar QR
       Code tidak dapat digunakan kembali. Proses validasi ini bertujuan untuk mencegah klaim
        poin berulang dan memastikan setiap transaksi tercatat dengan benar.
●  Fungsi 4: Sistem Reward dan Tukar Poin (Nafis Naufal Rahman)
    ○ Sumber Data (CRUD asal): Data pengguna, saldo dan riwayat poin dari transaksi QR yang
        telah diklaim, katalog reward, biaya poin, ketersediaan reward, serta riwayat penukaran.
    ○  Nilai Tambah/Problem Solving: Sistem memeriksa kelayakan penukaran berdasarkan saldo
         aktual, harga poin, status, dan ketersediaan reward, meminta konfirmasi pengguna, lalu
        memvalidasi ulang transaksi pada server. Pengurangan saldo dan pencatatan penukaran
        harus dilakukan secara atomik agar tidak terjadi saldo negatif, penukaran ganda, atau
        kehilangan  poin  akibat  kegagalan  jaringan.  Hasil  penukaran  ditampilkan  bersama
        riwayatnya.
●  Fungsi 5: Sistem Kalkulasi Poin dan Generate QR Transaksi (Aditya Akbar)
```

## Halaman 19

```text
○ Sumber Data (CRUD asal): Data jenis sampah, berat sampah, nilai konversi poin, data mitra,
        dan data transaksi penyetoran sampah.
     ○  Nilai Tambah/Problem Solving: Sistem tidak hanya mencatat data sampah yang disetorkan,
          tetapi juga melakukan perhitungan poin secara otomatis berdasarkan jenis dan berat
        sampah. Setelah mitra memverifikasi kesesuaian data dan  hasil perhitungan, sistem
        menghasilkan QR Code unik yang merepresentasikan transaksi penyetoran sampah. QR
        Code tersebut diberikan kepada pengguna sebagai sarana untuk melakukan klaim poin
         melalui fitur Scan QR. Dengan mekanisme  ini, proses perhitungan poin menjadi lebih
          konsisten,  mengurangi  risiko  kesalahan  perhitungan  manual,  serta mempermudah
         pencatatan transaksi penyetoran sampah secara digital. Perhitungan poin dilakukan secara
         otomatis oleh sistem menggunakan rumus Total Poin = Berat Sampah (kg) × Poin per
         Kilogram.  Nilai konversi poin per kilogram ditentukan berdasarkan jenis sampah dan
         ketentuan yang  telah tersimpan dalam sistem. Dengan demikian, jumlah poin yang
         diperoleh pengguna disesuaikan dengan jenis dan berat sampah yang disetorkan, sehingga
         proses perhitungan menjadi lebih akurat dan konsisten.



2.5.   Pembagian tugas Anggota

 ● Muhamad Fahry Pratama Putra (245150207111046): Bertanggung jawab pada Logika CRUD
     Inventaris dan implementasi Fungsi 1 dan 2 (Foto Sampah dan Peta Mitra).
 ● Muhammad Ghazy Humaidi (245150200111071): Bertanggung jawab pada Logika CRUD
     Transaksi Kasir dan implementasi Fungsi 3 (Scan QR).
 ●  Aditya Akbar (245150200111028): Bertanggung jawab pada Logika CRUD Transaksi Kasir dan
     implementasi Fungsi 4 (Generate QR).
 ●  Nafis Naufal Rahman (245150200111008): Bertanggung jawab pada Logika CRUD Transaksi
     Kasir dan implementasi Fungsi 5 (Reward / Tukar Poin).

2.6.   Screen Flow Aplikasi (Berlima)

       User Flow aplikasi EcoSnap menggambarkan alur interaksi pengguna dengan sistem, mulai
dari membuka aplikasi, melakukan autentikasi, mengakses fitur utama, hingga menyelesaikan proses
penyetoran sampah dan penukaran poin. EcoSnap memiliki dua jenis pengguna, yaitu pengguna
umum yang melakukan identifikasi dan penyetoran sampah, serta mitra yang bertugas menerima,
memverifikasi, dan mencatat transaksi penyetoran sampah.

Splash Screen + On-Boarding
```

## Halaman 20

```text
Login / Register
```

![Gambar 1 pada halaman 20](assets/utp-resubmission/page-20-figure-01.png)

![Gambar 2 pada halaman 20](assets/utp-resubmission/page-20-figure-02.png)

![Gambar 3 pada halaman 20](assets/utp-resubmission/page-20-figure-03.png)

![Gambar 4 pada halaman 20](assets/utp-resubmission/page-20-figure-04.png)

## Halaman 21

```text
User Page
```

![Gambar 1 pada halaman 21](assets/utp-resubmission/page-21-figure-01.png)

![Gambar 2 pada halaman 21](assets/utp-resubmission/page-21-figure-02.png)

![Gambar 3 pada halaman 21](assets/utp-resubmission/page-21-figure-03.png)

![Gambar 4 pada halaman 21](assets/utp-resubmission/page-21-figure-04.png)

![Gambar 5 pada halaman 21](assets/utp-resubmission/page-21-figure-05.png)

![Gambar 6 pada halaman 21](assets/utp-resubmission/page-21-figure-06.png)

## Halaman 22

_Halaman ini tidak memiliki text layer yang terbaca. Lihat gambar di bawah._

![Gambar 1 pada halaman 22](assets/utp-resubmission/page-22-figure-01.png)

![Gambar 2 pada halaman 22](assets/utp-resubmission/page-22-figure-02.png)

![Gambar 3 pada halaman 22](assets/utp-resubmission/page-22-figure-03.png)

![Gambar 4 pada halaman 22](assets/utp-resubmission/page-22-figure-04.png)

![Gambar 5 pada halaman 22](assets/utp-resubmission/page-22-figure-05.png)

![Gambar 6 pada halaman 22](assets/utp-resubmission/page-22-figure-06.png)

## Halaman 23

_Halaman ini tidak memiliki text layer yang terbaca. Lihat gambar di bawah._

![Gambar 1 pada halaman 23](assets/utp-resubmission/page-23-figure-01.png)

![Gambar 2 pada halaman 23](assets/utp-resubmission/page-23-figure-02.png)

![Gambar 3 pada halaman 23](assets/utp-resubmission/page-23-figure-03.png)

![Gambar 4 pada halaman 23](assets/utp-resubmission/page-23-figure-04.png)

![Gambar 5 pada halaman 23](assets/utp-resubmission/page-23-figure-05.png)

![Gambar 6 pada halaman 23](assets/utp-resubmission/page-23-figure-06.png)

## Halaman 24

_Halaman ini tidak memiliki text layer yang terbaca. Lihat gambar di bawah._

![Gambar 1 pada halaman 24](assets/utp-resubmission/page-24-figure-01.png)

![Gambar 2 pada halaman 24](assets/utp-resubmission/page-24-figure-02.png)

![Gambar 3 pada halaman 24](assets/utp-resubmission/page-24-figure-03.png)

![Gambar 4 pada halaman 24](assets/utp-resubmission/page-24-figure-04.png)

![Gambar 5 pada halaman 24](assets/utp-resubmission/page-24-figure-05.png)

![Gambar 6 pada halaman 24](assets/utp-resubmission/page-24-figure-06.png)

## Halaman 25

_Halaman ini tidak memiliki text layer yang terbaca. Lihat gambar di bawah._

![Gambar 1 pada halaman 25](assets/utp-resubmission/page-25-figure-01.png)

![Gambar 2 pada halaman 25](assets/utp-resubmission/page-25-figure-02.png)

![Gambar 3 pada halaman 25](assets/utp-resubmission/page-25-figure-03.png)

![Gambar 4 pada halaman 25](assets/utp-resubmission/page-25-figure-04.png)

![Gambar 5 pada halaman 25](assets/utp-resubmission/page-25-figure-05.png)

![Gambar 6 pada halaman 25](assets/utp-resubmission/page-25-figure-06.png)

## Halaman 26

```text
Mitra Page
```

![Gambar 1 pada halaman 26](assets/utp-resubmission/page-26-figure-01.png)

![Gambar 2 pada halaman 26](assets/utp-resubmission/page-26-figure-02.png)

![Gambar 3 pada halaman 26](assets/utp-resubmission/page-26-figure-03.png)

![Gambar 4 pada halaman 26](assets/utp-resubmission/page-26-figure-04.png)

![Gambar 5 pada halaman 26](assets/utp-resubmission/page-26-figure-05.png)

## Halaman 27

```text
Alur  penggunaan  EcoSnap  dimulai  ketika  pengguna membuka  aplikasi  dan  sistem
menampilkan Splash Screen yang memuat identitas aplikasi. Selanjutnya, pengguna baru diarahkan
ke tiga halaman Onboarding yang memperkenalkan fungsi utama EcoSnap, yaitu mengenali jenis
sampah melalui foto, menemukan lokasi mitra pengumpulan, dan memperoleh poin dari aktivitas
penyetoran sampah.
        Setelah menyelesaikan Onboarding, pengguna diarahkan menuju halaman autentikasi.
Pengguna yang telah memiliki akun dapat melakukan Login menggunakan kredensial yang terdaftar,
sedangkan pengguna baru dapat membuat akun melalui halaman Register. Pada proses registrasi,
tersedia dua pilihan peran, yaitu pengguna umum dan mitra. Sistem melakukan validasi data
sebelum memberikan akses ke halaman utama sesuai peran masing-masing. Apabila autentikasi
gagal, sistem menampilkan pesan kesalahan agar pengguna dapat memperbaiki data dan mencoba
kembali.
        Setelah berhasil login sebagai pengguna umum, pengguna diarahkan menuju halaman
Beranda yang menampilkan ringkasan saldo poin, akses cepat menuju fitur utama, informasi misi
mingguan, serta rekomendasi mitra pengumpulan sampah. Navigasi utama menggunakan Bottom
Navigation Bar yang terdiri atas lima menu, yaitu Beranda, Peta, Scan, Poin, dan Games.
       Pengguna dapat memulai identifikasi sampah melalui menu Scan dengan mengambil foto
menggunakan kamera smartphone atau memilih gambar dari galeri. Setelah gambar dipilih, sistem
memprosesnya menggunakan teknologi Artificial Intelligence (AI) untuk mengenali jenis dan kategori
material sampah.
        Apabila identifikasi berhasil, aplikasi menampilkan foto sampah beserta informasi material,
kategori,  karakteristik,  dan  panduan penanganan yang  sesuai. Pengguna  juga memperoleh
rekomendasi pemanfaatan kembali atau ide daur ulang serta informasi mitra yang dapat menerima
material tersebut. Jika gambar tidak berhasil dikenali, pengguna dapat mengambil foto ulang atau
memilih gambar lainnya. Hasil identifikasi dapat digunakan sebagai referensi untuk menentukan
tindakan pengelolaan sampah berikutnya.
       Pengguna dapat mengakses menu Peta untuk menemukan lokasi bank sampah atau mitra
pengumpulan. Ketika halaman dibuka, aplikasi memeriksa izin akses lokasi. Jika izin diberikan, sistem
```

![Gambar 1 pada halaman 27](assets/utp-resubmission/page-27-figure-01.png)

![Gambar 2 pada halaman 27](assets/utp-resubmission/page-27-figure-02.png)

![Gambar 3 pada halaman 27](assets/utp-resubmission/page-27-figure-03.png)

## Halaman 28

```text
menggunakan lokasi perangkat untuk menampilkan mitra di sekitar pengguna. Apabila izin ditolak,
pengguna tetap dapat melakukan pencarian dengan menentukan lokasi secara manual.
        Lokasi mitra ditampilkan menggunakan penanda pada peta. Pengguna dapat menyaring hasil
pencarian berdasarkan jenis material yang diterima, kemudian memilih salah satu mitra untuk
melihat informasi detail berupa nama, alamat, jam operasional, dan kategori sampah yang diterima.
Pengguna juga dapat mengakses petunjuk navigasi menuju lokasi mitra. Jika tidak ditemukan mitra
yang sesuai, sistem menyediakan pilihan untuk mengubah filter atau area pencarian.
        Setelah menentukan mitra, pengguna membawa sampah ke lokasi pengumpulan untuk
diperiksa secara langsung. Mitra melakukan verifikasi kondisi sampah, mencatat jenis dan berat
material yang diterima, kemudian menghasilkan QR Code transaksi berdasarkan hasil perhitungan
poin.
       Pengguna membuka fitur Scan QR melalui aplikasi dan memindai kode yang diberikan mitra.
Sistem kemudian memeriksa keabsahan QR Code, status transaksi, masa berlaku, dan apakah
transaksi telah diklaim sebelumnya. Jika valid, aplikasi menampilkan detail transaksi yang mencakup
nama mitra, jenis sampah, berat setoran, serta jumlah poin yang akan diterima.
       Pengguna dapat memeriksa informasi tersebut sebelum melakukan konfirmasi klaim. Setelah
dikonfirmasi, sistem memvalidasi ulang transaksi, menambahkan poin ke saldo pengguna, dan
menyimpan riwayat penyetoran. Jika QR Code tidak valid, kedaluwarsa, atau sudah digunakan, sistem
menampilkan informasi kegagalan tanpa menambahkan poin.
        Melalui menu Poin, pengguna dapat melihat saldo poin yang terkumpul dari transaksi
penyetoran yang berhasil diklaim. Pengguna juga dapat mengakses riwayat perolehan poin dan
katalog reward yang tersedia.
        Ketika pengguna memilih salah satu reward, aplikasi menampilkan detail hadiah, jumlah poin
yang diperlukan, dan ketentuan penukaran. Sistem memeriksa ketersediaan reward serta kecukupan
saldo pengguna. Apabila persyaratan terpenuhi, pengguna dapat mengonfirmasi penukaran. Setelah
transaksi  berhasil  diproses, sistem mengurangi saldo poin, mencatat riwayat penukaran, dan
menampilkan bukti atau status penukaran. Jika persyaratan tidak terpenuhi, penukaran tidak dapat
dilanjutkan.
        Selain fitur pengelolaan sampah, EcoSnap menyediakan menu Games yang berisi permainan
dan aktivitas edukatif mengenai pemilahan serta pengelolaan sampah. Pengguna dapat memilih
permainan, mengikuti tantangan, dan melihat hasil aktivitasnya.
       EcoSnap  juga  menyediakan  halaman  riwayat  untuk  meninjau  aktivitas  pemindaian,
penyetoran, serta transaksi poin. Melalui halaman profil, pengguna dapat melihat dan mengelola
informasi akun serta mengakses pengaturan aplikasi.
       Pengguna yang login sebagai mitra diarahkan menuju antarmuka khusus mitra yang berfokus
pada pengelolaan transaksi penyetoran sampah. Halaman utama mitra menyediakan akses menuju
fitur pencatatan setoran, kalkulator poin, pembuatan QR Code, dan riwayat aktivitas.
        Ketika  pengguna umum  datang untuk menyetorkan sampah,  mitra  terlebih  dahulu
melakukan pemeriksaan fisik untuk memastikan jenis, kondisi, dan kelayakan material yang diterima.
Setelah sampah dinyatakan memenuhi ketentuan, mitra membuka halaman Kalkulator Sampah
untuk memasukkan jenis dan berat sampah hasil penimbangan.
       Sistem kemudian menghitung jumlah poin secara otomatis berdasarkan berat dan nilai
konversi poin dari masing-masing jenis material. Hasil perhitungan ditampilkan untuk diperiksa
kembali oleh mitra. Apabila terdapat ketidaksesuaian, mitra dapat memperbaiki data sebelum
melanjutkan transaksi.
```

## Halaman 29

```text
Setelah data dinyatakan sesuai, mitra menekan tombol Generate QR. Sistem menghasilkan
QR Code  unik yang  merepresentasikan  transaksi  penyetoran  tersebut. QR Code kemudian
ditampilkan kepada pengguna umum untuk dipindai melalui fitur Scan QR pada akunnya.
        Setelah klaim berhasil dilakukan, transaksi tersimpan dalam sistem dan dapat dilihat melalui
riwayat aktivitas. Mitra juga dapat mengakses informasi transaksi penyetoran sebagai bagian dari
pemantauan kegiatan pengumpulan sampah.
       Secara keseluruhan, User Flow EcoSnap dirancang untuk menghubungkan aktivitas pengguna
umum dan mitra dalam satu proses yang terintegrasi. Pengguna umum melakukan identifikasi
sampah menggunakan AI, memperoleh panduan penanganan, mencari lokasi mitra, dan melakukan
penyetoran secara langsung. Sementara itu, mitra bertanggung jawab memeriksa sampah, mencatat
hasil penimbangan, menghitung poin, dan menghasilkan QR Code transaksi.
         Interaksi kedua peran dihubungkan melalui proses pemindaian dan validasi QR Code untuk
memastikan bahwa poin diberikan berdasarkan transaksi penyetoran yang sah. Dengan alur tersebut,
EcoSnap  tidak  hanya  menyediakan  informasi  mengenai  pengelolaan  sampah,  tetapi  juga
memfasilitasi tindakan nyata melalui penyetoran yang tercatat dan sistem penghargaan yang
terintegrasi.
```

## Halaman 30

```text
BAB III IMPLEMENTASI TEKNIS

(Bab ini diisi oleh masing-masing individu sesuai dengan Fungsi Utama yang menjadi tanggung
jawabnya)

3.1.   Implementasi Fungsi 1 : [Sistem Prediksi Restock Otomatis]

Penanggung Jawab: [Andi - 245100...]

A. Logika Pemrosesan Data (Problem Solving)

Jelaskan bagaimana algoritma pemrosesan data diimplementasikan pada kode.

Contoh: Fungsi Reorder Point dihitung dengan rumus (Lead Time x Average Daily Sales). Berikut
adalah implementasi fungsinya pada ViewModel:


 Kode                                                      Fungsi

 fun calculateReorderPoint(avgSales: Int,        Fungsi Reorder Point dihitung dengan
 leadTimeDays: Int): Int {                      rumus (Lead Time x Average Daily Sales)
     val safetyStock = 5 // asumsi buffer
     return (avgSales * leadTimeDays) +
 safetyStock
 }



B. Layout dan UI

Jelaskan elemen layout yang digunakan untuk menampilkan output.

Contoh: Menampilkan alert card menggunakan Jetpack Compose apabila currentStock <=
reorderPoint.


 Kode                                                     Fungsi

 @Composable                                      Menampilkan alert card menggunakan
 fun StockAlertCard(productName: String,         Jetpack Compose apabila currentStock <=
 remaining: Int) {                                     reorderPoint
     Card(colors =
 CardDefaults.cardColors(containerColor =
 Color.Red)) {
         Text(text = "Peringatan! Stok
 $productName sisa $remaining")
     }
 }
```

## Halaman 31

```text
C. Event Handling & State

Uraikan bagaimana interaksi pengguna diproses dan state dipertahankan.

Contoh: State menggunakan MutableStateFlow di ViewModel agar saat layar di-rotate, peringatan
stok tidak memuat ulang perhitungan dari awal.


D. Komunikasi Client-Server / Database

Uraikan penggunaan Thread/Coroutine atau database lokal (Room/API).

Contoh: Pengambilan riwayat transaksi dari SQLite menggunakan Room Database dijalankan dalam
Coroutine viewModelScope.launch { Dispatchers.IO } agar tidak memblokir UI thread saat
menghitung rata-rata penjualan.

3.2.   Implementasi Fungsi 2 : [Sistem Prediksi Restock Otomatis]

Penanggung Jawab: [Budi - 245100...]

(Gunakan format A, B, C, D yang sama seperti sub-bab 3.1)
```

## Halaman 32

```text
BAB IV HASIL DAN PENGUJIAN

4.1.   Demonstrasi Hasil Project

Tampilkan screenshot hasil aplikasi saat dijalankan pada emulator/perangkat asli.

Contoh:

(Masukkan Gambar Screenshot 4.1: Dashboard dengan notifikasi merah yang menandakan stok
harus diisi ulang berdasarkan perhitungan algoritma)

4.2.   Link Repository dan Video Demo

 ●  Repository GitHub: https://github.com/Andi/SmartGudang (Pastikan commit history terlihat
     kolaboratif).
 ●  Video Demo Aplikasi (1080p): https://youtube.com/watch?v=... (Video wajib mendemokan
      nilai tambah dari fungsi, misal: memperlihatkan bagaimana angka batas restock berubah
     otomatis).

4.3.   Kendala dan Solusi

Jelaskan kendala teknis dan solusinya.


 Kendala                     Penyebab                         Solusi

 Saat menghitung histori         Logika menumpuk di main      Memindahkan logika
 transaksi ribuan baris, aplikasi   thread                        perhitungan (Problem Solving)
 mengalami freeze / lag sesaat.                                     dari Main Thread ke Worker
                                                          Thread menggunakan
                                                               Coroutine Dispatchers.Default.
```

## Halaman 33

```text
BAB V PENUTUP

5.1.   Kesimpulan

Tuliskan kesimpulan mengenai hasil project dan keberhasilan penerapan problem solving.

Contoh: Aplikasi SmartGudang telah berhasil dibangun. Fokus utama penambahan nilai berupa
kalkulasi otomatis Reorder Point berjalan dengan baik dan berhasil dipisahkan dari sekadar operasi
CRUD biasa, sehingga aplikasi dapat memberikan rekomendasi riil kepada pemilik toko.

5.2.   Saran Pengembangan

Tuliskan fitur yang dapat ditingkatkan di masa depan.

Contoh: Di masa depan, prediksi restock dapat ditingkatkan akurasinya dengan menyematkan model
Machine Learning ringan menggunakan TensorFlow Lite alih-alih perhitungan rata-rata statis.
```

## Link yang tertanam dalam PDF

- Halaman 17: <https://www.figma.com/design/GcZxuR5nuaOH8GB1Lo37lW/EcoSnap?node-id=0-1&t=8YeKIIoAKZ03x507-1>
