# Kontrak akun EcoSnap

Pembaruan verifikasi: full suite PostgreSQL lokal portable kini PASS tanpa SKIP, termasuk akun, query mitra, favorit, dan CLI seed. Lihat [laporan PostgreSQL](../test-results/postgres-favorites.md). Catatan belum diuji/SKIP di bagian historis berikut menggambarkan pemeriksaan sebelumnya. Supabase, Firebase nyata, dan Android tetap belum diuji end-to-end.


Status: implementasi lokal tersedia; migration dan perilaku PostgreSQL nyata belum diuji. Frontend tidak diubah. Endpoint membutuhkan Firebase ID token melalui Authorization Bearer. Kontrak ini memenuhi FE-02/03/20 pada audit frontend.

## POST /api/v1/accounts

Content-Type application/json. Batas body 16 KiB; satu objek JSON; field yang tidak dikenal ditolak. UID, email, status, balance, dan password bukan field request. UID/email diambil dari token Firebase. Role wire adalah user atau mitra, bukan USER/PARTNER dari enum Kotlin.

Contoh user:

```json
{"role":"user","full_name":"Fahry","phone":"08123456789"}
```

Contoh mitra:

```json
{"role":"mitra","full_name":"Pemilik Tunas","phone":"08123456789","business_name":"Bank Sampah Tunas","business_category":"Plastik"}
```

Nama dan input teks dipangkas whitespace tepinya. Full name wajib 1-150 karakter; display_name otomatis memakai maksimal 80 rune pertama nama lengkap. Nomor berupa 9-15 digit dengan tanda + opsional di awal. Nama usaha 1-150 karakter dan kategori usaha 1-80 karakter wajib untuk mitra; user biasa tidak mengirim detail usaha. Nilai kategori usaha sementara berupa teks sesuai form, bukan kategori material yang otomatis disetujui.

Pembuatan pertama menghasilkan 201; request ulang identik setelah normalisasi menghasilkan 200 dan profil yang tersimpan. Perubahan role/nama/nomor/detail usaha melalui retry menghasilkan 409; endpoint ini bukan update profil. Email yang berubah pada token tidak menimpa email profil saat retry. Kegagalan atau timeout setelah commit dapat diselesaikan dengan GET profil dan retry identik.

Account, settings, wallet nol, dan detail usaha pending dibuat dalam satu transaksi database. INSERT akun memakai ON CONFLICT untuk UID yang sama; proses yang kalah membaca hasil transaksi pemenang. Perilaku ini masih perlu diverifikasi dengan request paralel pada PostgreSQL nyata.

Mitra pending disimpan pada partner_registrations, terpisah dari tabel lokasi partners. Tidak ada alamat palsu, lokasi terverifikasi, atau izin QR yang dibuat. Kolom status pendaftaran adalah pending/approved/rejected; endpoint persetujuan dan pelengkapan usaha belum diimplementasikan. Status tersebut tidak menggantikan status lokasi mitra di rancangan partners. Satu akun satu tempat tetap berlaku.

## GET /api/v1/me

Mengambil profil sendiri berdasarkan UID token. Query uid tidak memilih akun lain. Response 200 memakai bentuk yang sama dengan hasil pendaftaran:

```json
{
  "data": {
    "uid":"firebase-uid",
    "role":"user",
    "full_name":"Fahry",
    "display_name":"Fahry",
    "email":"user@example.com",
    "phone":"08123456789",
    "status":"active",
    "created_at":"2026-11-17T00:00:00Z",
    "wallet":{"balance":0,"total_earned":0,"total_spent":0},
    "settings":{"notification_enabled":true,"show_on_leaderboard":true,"onboarding_completed":false}
  }
}
```

Untuk mitra, response juga memuat business dengan name/category/status. User biasa tidak mempunyai field business. Saldo dan total memakai angka integer; format ribuan dibuat Android. Timestamp dikirim Go dengan timezone. Email dapat kosong jika token tidak memiliki claim email; kebijakan email verification wajib belum ditetapkan.

## Error

Envelope error mengikuti endpoint identitas: error.code dan error.message. Tidak mengirim SQL error/credential ke client.

| Status | Code | Kondisi |
|---|---|---|
| 400 | invalid_request | JSON salah/field asing/body tambahan |
| 400 | invalid_account | Data form tidak memenuhi validasi |
| 401 | unauthorized | Token hilang atau ditolak |
| 403 | account_forbidden | Akun suspended/deleted |
| 404 | account_not_found | Profil belum tersedia |
| 409 | account_conflict | Retry menggunakan detail berbeda |
| 413 | body_too_large | Body melebihi batas |
| 415 | unsupported_media_type | Content-Type bukan application/json |
| 504 | request_timeout | Deadline operasi akun 5 detik terlampaui |
| 500 | internal_error | Kegagalan internal/database lainnya |

Semua route akun dibungkus middleware auth dan memakai Cache-Control no-store. Middleware juga mengisi identitas dari token. GET /api/v1/auth/me tetap hanya identitas Firebase; GET /api/v1/me adalah profil PostgreSQL.

## File implementasi dan pengujian

- domain/entity/account_profile.go: data profil/settings/wallet/detail usaha untuk hasil operasi.
- internal/service/account_registration.go: validasi, input, interface penyimpanan dan error; account_profile.go memeriksa status hasil baca.
- internal/repository/postgres/account_registration.go: transaksi pendaftaran/idempotency; account_profile.go membaca relasi profil.
- internal/delivery/http/handler/account_registration.go: JSON/batas request dan pemetaan error; account_profile.go menangani GET.
- internal/bootstrap/app.go, delivery/http/router.go, cmd/app/main.go: penyambungan pool dan route.
- database/migrations/00002_create_partner_registrations.sql: tabel pendaftaran usaha dengan RLS dan grant dibatasi. DBML lama tetap arsip; migration merupakan perubahan implementasi.
- internal/delivery/http/account_endpoints_test.go: uji HTTP dengan verifier dan penyimpanan pengganti. Tidak membuktikan rollback/commit/constraint/race PostgreSQL.

Jalankan migration 00002 pada database development menggunakan Goose sebelum mencoba endpoint akun. CLI memakai environment GOOSE_DRIVER=postgres dan GOOSE_DBSTRING yang menunjuk database development; perintahnya `goose -dir database/migrations up`. Migration pertama tidak diubah, migration kedua belum diterapkan AI. Goose mencatat versi yang telah dijalankan; tidak perlu menjalankan SQL CREATE secara manual.

Pemeriksaan yang masih diperlukan: upgrade migration, rollback terkontrol pada database disposable, akun/settings/wallet atomik saat query gagal, request paralel UID sama, retry berbeda menghasilkan 409, serta integrasi token Firebase dan PostgreSQL nyata. Dilarang menyebut AUTH-03 selesai berdasarkan tes memory store saja.

## Suite PostgreSQL tambahan

Integration test melalui Register/Profile dan runner Docker opsional sudah ditambahkan. Sembilan skenario serta cara menjalankan dijelaskan pada [panduan pengujian PostgreSQL](../testing-accounts-postgres.md). Saat ini test database masih SKIP karena engine/environment belum tersedia. Ini tidak mengganti status verifikasi PostgreSQL yang belum selesai di atas.

## PATCH /api/v1/me (2026-11-20)

Endpoint update profil sendiri sudah diimplementasikan. Membutuhkan Authorization Bearer dan Content-Type application/json. Body maksimal 16 KiB, satu objek JSON; hanya full_name dan phone yang diterima, minimal salah satu. Field yang tidak dikirim tetap menggunakan nilai tersimpan. Null, string kosong setelah trim, tipe non-string, field asing, body tambahan, dan parameter query ditolak. Nomor memakai validasi registrasi: 9-15 digit, + opsional di awal. Full name maksimal 150 karakter Unicode.

```json
{"full_name":"Nama Baru"}
```

```json
{"phone":"+628123456789"}
```

Jika nama berubah, display_name dihitung ulang dari maksimal 80 rune pertama nama lengkap. Endpoint tidak menyediakan perubahan display_name terpisah. UID berasal dari token; uid/role/status/email/business/wallet bukan field update. Email tetap dikelola melalui alur Firebase yang belum diintegrasikan. Foto profil belum termasuk endpoint ini.

Response 200 berisi envelope data dengan AccountProfile seperti GET /api/v1/me. Request sama dapat diulang tanpa efek tambahan. Update memakai transaksi dan row lock akun, sehingga status aktif diperiksa sebelum penyimpanan dan partial update bersamaan tidak menimpa field yang tidak dikirim. Akun missing menghasilkan 404, suspended/deleted 403. Input profil invalid menghasilkan 400 invalid_profile; kesalahan JSON/field/null/query menghasilkan 400 invalid_request. Error 401/413/415/504/500 mengikuti kontrak akun sebelumnya.

Wallet, settings, role, email, dan detail usaha tidak di-update. Jika profil sudah diedit, retry POST pendaftaran dengan data lama dapat menghasilkan 409 karena data berbeda dari profil tersimpan. Gunakan GET untuk pemulihan sesi; POST pendaftaran ditujukan ketika profil memang belum tersedia, bukan untuk sinkronisasi setiap login.

Source tambahan: account_profile_update.go pada service, repository/postgres, dan delivery/http/handler. AccountStore diperluas dengan UpdateProfile; router memakai middleware auth yang sama. Tidak ada migration baru atau perubahan Android.

Verifikasi 2026-11-20: baseline seluruh suite PASS terlebih dahulu. Tes HTTP baru gagal karena PATCH belum terpasang, kemudian lolos setelah implementasi. Full suite sesudah perubahan juga PASS pada PostgreSQL portable nyata tanpa SKIP. TestAccountProfileUpdateIntegration menguji update sebagian, retry, Unicode, dua update paralel pada field berbeda, wallet tetap 250, profil user lain utuh, missing/suspended ditolak dan data tidak berubah setelah penolakan. HTTP menguji field terlarang/null/empty/tipe salah/limit body/auth/content type. Bukti: docs/test-results/profile-update-2026-11-20.md.
