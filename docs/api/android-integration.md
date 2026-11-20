# Integrasi backend dan Android EcoSnap

Audit kode lokal: 17 November 2026. Backend: EcoSnap-BE. Android: direktori EcoSnap-FE di samping repository backend. Dokumen ini mencatat kontrak yang sudah tersedia, kondisi frontend, dan urutan pekerjaan berikutnya. Status implementasi keseluruhan tetap dicatat di [BUILD_PROCESS.md](../../BUILD_PROCESS.md).

Requirement dan keputusan diperbarui melalui [audit mendalam](../FRONTEND_BACKEND_AUDIT.md). Dokumen ini mempertahankan kontrak dua route yang sudah tersedia. Tahap audit telah dilanjutkan ke implementasi akun atas permintaan pemilik; lihat accounts.md dan pembaruan pada akhir dokumen. Mitra demo memakai seed, akun baru pending tanpa izin QR, dan keputusan terakhir satu akun mengelola satu tempat. Bagian rencana di bawah dibaca bersama keputusan terbaru tersebut.

## 1. Hasil pemeriksaan frontend

Pemeriksaan dilakukan pada source Kotlin dan konfigurasi Gradle. Build dan pengujian runtime Android belum dijalankan.

| Area | Bukti source Android | Kebutuhan integrasi |
|---|---|---|
| Login | `navigation/AuthNavGraph.kt`: callback `onLogin` mengabaikan email/password dan langsung membuka main graph. | Firebase SDK melakukan login. Android mengirim ID token ke backend. Navigasi mengikuti hasil autentikasi dan kelengkapan profil. |
| Registrasi | `features/auth/model/AuthRole.kt`: RegistrationData berisi role, fullName, email, phone, password, businessName, businessCategory. Callback di AuthNavGraph hanya kembali ke halaman sebelumnya. | Password dikirim ke Firebase SDK. Profil aplikasi disimpan melalui backend setelah token tersedia. Retry pembuatan profil harus aman. |
| Validasi registrasi | `RegisterScreen.kt`: commonValid memanggil `isValidPhone(email)`. Regex nomor di AuthRole.kt berakhir dengan karakter `}` tambahan. | Perbaiki validasi Android saat mengerjakan integrasi auth. Backend tetap memvalidasi input sendiri. |
| Reset password | AuthNavGraph menghubungkan halaman melalui callback navigasi. | Firebase mengirim reset link. Perubahan password memerlukan action code yang valid; berpindah halaman saja tidak mengubah password. |
| Beranda | `HomeUiState.demo()` dipakai di MainNavGraph. | Isi nama, saldo, jumlah setoran, misi, dan mitra dari backend. Data kosong harus dapat ditampilkan tanpa angka demo. |
| Peta mitra | WastePartner dan DummyPartners menyimpan data lokal. | Kontrak daftar/detail mitra perlu memuat ID, nama, alamat, koordinat, jam operasional, kontak, dan kategori diterima. |
| Klaim QR | LocalDeposits menganggap isi QR sebagai ID transaksi; ScanNavGraph langsung mencari hasil lokal. | Kirim token QR ke backend. Backend memvalidasi masa berlaku dan klaim atomik sebelum Android menampilkan keberhasilan. |
| Foto sampah | Model dan repository lokal sudah tersedia. | Provider AI belum ditentukan. Integrasi tidak boleh menyajikan hasil hardcoded sebagai prediksi nyata. |
| Games | GameQuestion membawa correctAnswer; GamePlayScreen menghitung jawaban benar di perangkat. | Server menyimpan kunci jawaban dan menghitung XP. Kontrak soal sebelum dijawab tidak mengirim correctAnswer. |
| Leaderboard dan artikel | LocalLeaderboard dan LocalArticles menyediakan konten lokal. | Ganti sumber data setelah kontrak daftar/detail/periode dibuat. |
| Profil | UserProfile berisi name/email; GreenStats memakai String untuk angka. Logout masih TODO. | Profil mengambil data akun aplikasi; angka diformat di UI. Logout mengakhiri sesi Firebase di Android. |
| Halaman lanjutan | Riwayat, voucher, pencapaian, laporan, dan privasi profil memakai ProfilePlaceholderScreen. Beberapa route utama juga placeholder. | Keberadaan route tidak membuktikan fitur tersebut selesai. Detail kebutuhan dikembangkan sesuai backlog. |

Dependensi OkHttp sudah ada dan digunakan untuk peta pada MainActivity. Pemeriksaan source belum menemukan pemanggilan backend aplikasi atau Firebase Auth Android. Daftar dependency app juga belum memasukkan Firebase Auth.

## 2. Batch pertama yang sudah ditulis

Tujuan batch ini adalah menyediakan pemeriksaan identitas Firebase melalui HTTP. Android belum disambungkan otomatis oleh perubahan backend ini.

| Bagian | Tanggung jawab |
|---|---|
| `cmd/app/main.go` | Membuat satu Firebase Auth client saat startup dan meneruskannya ke server. |
| `internal/bootstrap/app.go` | Menyusun server menggunakan dependency verifier dari startup. |
| `internal/delivery/http/router.go` | Memasang route publik health dan route identitas yang dilindungi middleware. |
| `internal/delivery/http/middleware/auth.go` | Membaca Bearer token, memanggil verifier, dan menyimpan identitas terverifikasi pada request context. |
| `internal/delivery/http/handler/auth.go` | Mengembalikan identitas dari context; tidak membuat akun PostgreSQL. |
| `internal/delivery/http/response/json.go` | Menulis response JSON dan bentuk error yang konsisten untuk endpoint baru. |
| `internal/delivery/http/router_test.go` | Memeriksa perilaku HTTP dengan verifier pengganti tanpa credential. |

`TokenVerifier` merupakan interface kecil yang dipenuhi Firebase Auth client. Pengujian menggantinya dengan fungsi lokal agar dapat menguji penerimaan/penolakan tanpa mengirim token ke jaringan. Interface ini tidak mengganti keputusan penggunaan Firebase.

Handler-Service-Repository akan digunakan ketika endpoint memerlukan aturan bisnis dan akses database. Endpoint pemeriksaan identitas tidak memerlukan query atau service perantara.

## 3. Kontrak yang tersedia

### GET /health

Publik, response 200:

```json
{"status":"ok"}
```

Memeriksa proses HTTP saja. Proses startup kini tetap membutuhkan konfigurasi database dan Firebase sebelum server mulai melayani request.

### GET /api/v1/auth/me

Header:

```http
Authorization: Bearer <FIREBASE_ID_TOKEN>
```

Gunakan ID token dari pengguna yang login melalui Firebase SDK. API key project, UID, refresh token, service account JSON, dan kode transaksi bukan pengganti ID token.

Response 200:

```json
{
  "data": {
    "uid": "firebase-user-uid",
    "email": "user@example.com",
    "email_verified": false
  }
}
```

`email_verified` diteruskan dari claim token; false tidak dinaikkan menjadi true oleh backend. Email dapat berupa string kosong apabila claim tidak tersedia. Semua field identitas berasal dari token terverifikasi. Query/body berisi UID tidak mengubah identitas request.

Response 401 untuk header hilang, format salah, atau token ditolak:

```json
{
  "error": {
    "code": "unauthorized",
    "message": "ID token Firebase tidak valid atau sudah kedaluwarsa."
  }
}
```

Pesan untuk header hilang/format salah berbeda, tetapi code tetap `unauthorized`. Android menggunakan status dan code untuk percabangan; message dapat ditampilkan kepada pengguna.

Response endpoint ini memakai Content-Type application/json, Cache-Control no-store, dan header WWW-Authenticate Bearer pada penolakan. GET juga mengikuti dukungan HEAD bawaan ServeMux. Route dan kesalahan method yang ditangani ServeMux di luar endpoint ini masih dapat menghasilkan response standar net/http, bukan envelope JSON.

Endpoint ini belum membaca role/status akun PostgreSQL, belum membuat profil/settings/wallet, dan belum menentukan kelayakan mitra. Firebase VerifyIDToken digunakan sebagaimana implementasi awal; pemeriksaan revocation dan akun Firebase disabled belum diaktifkan. Karena itu, endpoint ini tidak menjadi dasar tunggal otorisasi transaksi. Referensi: [Firebase ID token verification](https://firebase.google.com/docs/auth/admin/verify-id-tokens).

## 4. Urutan integrasi selanjutnya

Path pada bagian ini adalah usulan, belum tersedia. Prefix untuk endpoint baru mengikuti `/api/v1`.

| Urutan | Hasil | Alasan dan syarat |
|---|---|---|
| 1 | Registrasi profil: POST /api/v1/accounts; GET /api/v1/me | Pembuatan akun, settings, wallet dilakukan dalam transaksi dan idempotent berdasarkan UID. Profil milik caller diambil dari token. Role/status tidak dapat dinaikkan melalui update profil. |
| 2 | Integrasi Firebase Android untuk login/register/reset dan token HTTP | Login hanya membuka halaman utama setelah hasil Firebase dan state profil diketahui. Jika akun Firebase berhasil dibuat tetapi penyimpanan profil gagal, Android dapat mengulang penyimpanan profil. |
| 3 | Profil/settings/wallet dan ringkasan beranda | Menggantikan nama/saldo demo. Metrik yang tabelnya belum tersedia tidak diklaim sebagai data nyata. |
| 4 | Kategori dan daftar/detail mitra | Menyediakan sumber data peta dan kategori transaksi. Seed development memakai ID stabil. |
| 5 | Setoran multi-item, estimasi, penerbitan/reissue QR, klaim, ledger | Satu transaksi memuat beberapa material. QR berlaku 10 menit dari server; reissue membatalkan token lama. Klaim tunggal dan saldo harus aman terhadap request paralel/retry. |
| 6 | Riwayat dan reward simulasi | Riwayat berasal dari transaksi dan ledger. Penukaran mengurangi saldo serta stok secara atomik; tidak menghubungkan pencairan DANA nyata. |
| 7 | Artikel dan integrasi foto sampah | Artikel memakai konten terbit. Scan menunggu provider/model AI dan kebijakan media. |
| 8 | Games, XP, misi, leaderboard | Aturan pengulangan, periode, dan hadiah difinalisasi. Server memeriksa jawaban dan memisahkan XP dari saldo EcoPoints. |
| 9 | Notifikasi, support, laporan PDF, admin, Docker | Mengikuti scope rilis dan keputusan yang masih terbuka pada BUILD_PROCESS.md. |

Registrasi mitra memerlukan keputusan tambahan. Form Android belum meminta alamat, sedangkan rancangan partners mewajibkannya. Pilih alur pelengkapan profil usaha atau perluas form sebelum mengaktifkan mitra. Jangan menyimpan alamat kosong atau otomatis memberi status verified untuk menghindari keputusan ini.

## 5. Aturan penyesuaian data

- Bentuk field wire JSON memakai snake_case dan envelope `data`/`error`. Model Kotlin dapat tetap memakai camelCase melalui DTO mapping.
- AuthRole.USER dipetakan ke `user`; AuthRole.PARTNER dipetakan ke `mitra` setelah aturan pendaftaran role ditetapkan. Pilihan di form bukan bukti izin bertransaksi.
- Password hanya digunakan dalam proses Firebase Auth Android; backend profil tidak menerima atau menyimpan password.
- Saldo, harga poin, dan XP dikirim sebagai angka. String tampilan seperti `1.250 poin` dibuat oleh UI.
- Timestamp fitur berikutnya direncanakan memakai RFC3339; label seperti `21 Okt 2026` dibuat oleh Android.
- Kontrak estimasi memakai string desimal berat (maksimal 3 desimal) dan tarif (2 desimal); poin total integer dibulatkan ke bawah sesudah seluruh item dijumlahkan. Detail pada [point-estimates.md](point-estimates.md). Model demo Double tidak menentukan cara perhitungan saldo server.
- Kategori material sebaiknya memiliki ID/slug stabil dan label tampilan. Resource drawable Android tidak dikirim sebagai ID gambar lintas sistem.
- Loading, empty state, validation error, unauthenticated, forbidden, dan retry harus ditangani saat menghubungkan tiap layar.

## 6. Menjalankan dan memeriksa

Program membaca environment proses, tanpa memuat .env otomatis. Isi DATABASE_URL dan GOOGLE_APPLICATION_CREDENTIALS sesuai konfigurasi development lokal; .env.example hanya contoh placeholder. Jangan masukkan credential atau token ke source.

```powershell
go test -mod=readonly ./...
go vet -mod=readonly ./...
go build -mod=readonly ./...
go run ./cmd/app
```

Pada terminal lain, periksa penolakan tanpa token:

```powershell
curl.exe -i http://127.0.0.1:8080/api/v1/auth/me
```

Setelah Android mendapatkan ID token valid dari project Firebase yang sama, kirim token tersebut sebagai Authorization Bearer ke endpoint identitas. Jangan menempelkan token pada dokumentasi, commit, atau screenshot bersama.

Tes otomatis menggunakan verifier pengganti. Tes ini membuktikan wiring HTTP, context, error JSON, dan pemisahan route publik; bukan keberhasilan credential, signature token, expiry/project validation Firebase, atau koneksi Supabase nyata. Pengujian dengan layanan nyata perlu dicatat terpisah.

Untuk Android Emulator, alamat loopback backend host umumnya diakses melalui 10.0.2.2. Perangkat fisik memerlukan alamat host yang dapat dijangkau, HTTP_ADDR yang sesuai, dan pengaturan jaringan development. Kebijakan HTTP cleartext/HTTPS Android ditentukan saat integrasi; backend saat ini tetap default ke 127.0.0.1 agar tidak membuka listener jaringan tanpa kebutuhan.

## Pembaruan batch akun

POST /api/v1/accounts dan GET /api/v1/me kini memiliki implementasi lokal. Kontrak dan batas pengujian ada di [accounts.md](accounts.md). Daftar audit dan status dua route di atas adalah riwayat sebelum batch akun; migration 00002 belum diterapkan pada database remote.

## Pembaruan kategori dan mitra

GET /api/v1/partners, GET /api/v1/partners/{partnerID}, dan GET /api/v1/waste-categories memiliki implementasi lokal. Kontrak: [partners.md](partners.md). Tidak ada perubahan Android, penerapan migration remote, atau eksekusi seed pada batch ini.

## Pembaruan favorit

GET /api/v1/me/favorites serta PUT/DELETE /api/v1/me/favorites/{partnerID} tersedia; kontrak [partner-favorites.md](partner-favorites.md). Tes PostgreSQL lokal juga sudah PASS untuk akun, mitra, favorit, dan seed CLI. Supabase/Android belum diintegrasikan.

## Pembaruan edit profil 2026-11-20

PATCH /api/v1/me tersedia untuk full_name dan phone; field lain ditolak. Backend dan PostgreSQL lokal lolos tes. Kontrak lengkap [accounts.md](accounts.md). Form Android belum diubah atau diintegrasikan.

## Pembaruan estimasi multi-material 2026-11-20

POST /api/v1/point-estimates tersedia dengan Firebase bearer. Kategori/berat dikirim per item; tarif dihitung server, wallet tetap. Kontrak request/response/error, seed demo, serta kebutuhan DTO tercatat pada [point-estimates.md](point-estimates.md). features/deposit pada worktree FE masih placeholder .gitkeep; batch ini tidak mengubah Android. Setoran/QR/claim/ledger masih backlog.
