# Kontrak akun EcoSnap

**Status saat ini:** akun, profil, dan settings lolos tes PostgreSQL lokal portable tanpa SKIP ([laporan](../test-results/postgres-favorites.md)). Pemilik melaporkan migration 00001–00010 Applied di Supabase development dan smoke 57 lulus dengan Firebase nyata ([bukti](../test-results/gamification-2026-11-21.md)); smoke itu mendahului settings, sehingga endpoint settings belum diuji dengan Firebase/Supabase nyata. Migration 00011–00012 belum diterapkan ke Supabase. Android belum terhubung. Catatan "belum diuji", "SKIP", dan "belum diterapkan" di bagian historis di bawah menggambarkan keadaan saat bagian itu ditulis. Urutan pemakaian di Android: [android-integration.md](android-integration.md#21-sesi-pemulihan-profil-registrasi).


Status awal (historis): implementasi lokal; saat itu migration dan perilaku PostgreSQL nyata belum diuji. Endpoint membutuhkan Firebase ID token melalui Authorization Bearer. Kontrak ini memenuhi FE-02/03/20 pada audit frontend.

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

Jalankan migration pada database development menggunakan Goose sebelum mencoba endpoint akun. CLI memakai environment GOOSE_DRIVER=postgres dan GOOSE_DBSTRING yang menunjuk database development; perintahnya `goose -dir database/migrations up`. Goose mencatat versi yang telah dijalankan; tidak perlu menjalankan SQL CREATE secara manual. Migration 00001–00010 dilaporkan Applied oleh pemilik; 00011–00012 belum.

Pemeriksaan yang diperlukan saat bagian ini ditulis (historis; sebagian sudah dibuktikan oleh suite PostgreSQL lokal, sedangkan token Firebase dan Supabase nyata untuk settings masih belum): upgrade migration, rollback terkontrol pada database disposable, akun/settings/wallet atomik saat query gagal, request paralel UID sama, retry berbeda menghasilkan 409, serta integrasi token Firebase dan PostgreSQL nyata. Dilarang menyebut AUTH-03 selesai berdasarkan tes memory store saja.

## Suite PostgreSQL tambahan

Integration test melalui Register/Profile dan runner Docker opsional sudah ditambahkan. Sembilan skenario serta cara menjalankan dijelaskan pada [panduan pengujian PostgreSQL](../testing-accounts-postgres.md). Saat bagian ini ditulis test database masih SKIP; suite portable kemudian lulus tanpa SKIP (lihat status di atas).

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

## GET/PATCH /api/v1/me/settings (2026-11-21)

Status: ADA KODE pada branch lokal; tes PostgreSQL nyata dan HTTP terarah lulus tanpa SKIP. Supabase/Firebase nyata untuk endpoint settings dan Android belum diuji. Pengaturan akun milik caller, diambil dari UID token. User dan mitra berstatus active dapat memakainya. Memakai tabel `user_settings` yang sudah ada; tidak ada migration baru.

Response 200 (GET dan PATCH) hanya memuat tiga boolean:

```json
{"data":{"notification_enabled":true,"show_on_leaderboard":true,"onboarding_completed":false}}
```

PATCH membutuhkan `Content-Type: application/json`, body maksimal 16 KiB, satu objek JSON. Hanya `notification_enabled` dan `onboarding_completed` yang dapat dikirim; keduanya opsional boolean, tetapi minimal salah satu wajib ada. Field yang dihilangkan tidak berubah; `false` adalah nilai sah. Ditolak dengan 400 `invalid_request`: parameter query apa pun (termasuk query malformed), body kosong, `{}`, `null`, nilai bukan boolean, field tidak dikenal (termasuk `show_on_leaderboard`), dan teks tambahan setelah objek JSON.

```json
{"notification_enabled":false}
```

```json
{"onboarding_completed":true}
```

- `show_on_leaderboard` hanya dapat dibaca di sini. Satu-satunya penulis tetap `PATCH /api/v1/me/leaderboard-visibility` (user active saja); lihat [gamification.md](gamification.md).
- `notification_enabled` mengendalikan pembuatan notifikasi inbox **baru** (migration 00011): bernilai `false` menahan notifikasi baru, sedangkan notifikasi yang sudah ada tetap terbaca. Tidak ada push (Q-10 tetap terbuka); lihat [notifications.md](notifications.md).
- `onboarding_completed` boleh dikirim `false` maupun `true` dan idempotent. Backend tidak memakainya untuk menggerbang akses; keputusan Q-12 lain tidak dibuat.
- Tidak ada perubahan Android.

| Status | Code | Kondisi |
|---|---|---|
| 400 | invalid_request | Query, body kosong/`{}`/`null`, bukan boolean, field asing, JSON tambahan |
| 401 | unauthorized | Token hilang atau ditolak |
| 403 | account_forbidden | Akun bukan active |
| 404 | account_not_found | Akun atau baris settings tidak ada (tidak ada nilai bawaan yang dibuat) |
| 413 | body_too_large | Body melebihi 16 KiB |
| 415 | unsupported_media_type | Content-Type bukan application/json |
| 504 | request_timeout | Batas operasi terlampaui |
| 500 | internal_error | Kegagalan internal |

Konstruksi produksi: `service.NewSettingsService(store service.SettingsStore)` dengan `SettingsStore{Settings(ctx, uid) (SettingsSnapshot, error); UpdateSettings(ctx, uid, AccountSettingsUpdate) (entity.AccountSettings, error)}`; `postgres.NewSettingsRepository(pool)`; `handler.NewSettingsHandler(svc)` (Get/Update); route didaftarkan oleh `httpdelivery.RegisterSettingsRoutes`. Error service `ErrInvalidSettingsUpdate` dipetakan ke 400 `invalid_request`.

Verifikasi: `go test -p 1 -mod=readonly -count=1 -v ./internal/repository/postgres ./internal/delivery/http ./internal/service -run 'Test.*Settings'` pada PostgreSQL lokal disposable menghasilkan exit 0, tanpa SKIP. TestSettingsRepositoryIntegration (8 skenario) membuktikan kedua role, nilai false, persistensi yang terlihat lewat profil, isolasi akun, privasi leaderboard tetap utuh, update paralel field berbeda, penolakan suspended/deleted, dan missing settings tanpa membuat default. Enam tes HTTP membuktikan auth, batas body/content type, query/field/null/tipe salah, PATCH parsial dan UID dari token. Paket service dikompilasi tetapi tidak memiliki tes yang cocok dengan filter ini. Tidak menjalankan ulang full suite; smoke 57 PASS sebelumnya belum mencakup settings.

