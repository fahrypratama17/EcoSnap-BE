# Kontrak mitra dan kategori sampah

Pembaruan verifikasi: full suite PostgreSQL lokal portable kini PASS tanpa SKIP, termasuk akun, query mitra, favorit, dan CLI seed. Lihat [laporan PostgreSQL](../test-results/postgres-favorites.md). Catatan belum diuji/SKIP di bagian historis berikut menggambarkan pemeriksaan sebelumnya. Supabase, Firebase nyata, dan Android tetap belum diuji end-to-end.


Status: implementasi lokal tersedia; migration 00003, query PostgreSQL, dan seed belum diuji pada database nyata. Frontend tidak diubah. Data mengikuti WastePartner/PartnerLocation/WasteMaterial pada EcoSnap-FE dan requirement FE-06/07 pada audit. Satu akun mitra tetap satu tempat.

## Endpoint

Semua endpoint batch ini membutuhkan Authorization Bearer ID token Firebase. Daftar hanya berisi lokasi verified yang pemiliknya akun active dengan role mitra. Middleware memeriksa token caller; status PostgreSQL caller belum diperiksa pada endpoint baca informasi ini. Endpoint tidak memberi kewenangan membuat setoran atau QR.

| Method/path | Hasil |
|---|---|
| GET /api/v1/waste-categories | data berupa array kategori |
| GET /api/v1/partners | data berupa array lokasi, beserta pagination |
| GET /api/v1/partners/{partnerID} | data berupa satu lokasi visible |

Daftar mitra menerima q (substring nama atau area, case insensitive), material (slug kategori), limit (default 20, 1-100), offset (default 0, 0-10000). Pencarian memang nama/area sesuai UI; tidak menambahkan pencarian alamat atau radius tanpa kebutuhan. q maksimal 100 karakter; material maksimal 100 karakter berupa slug huruf/angka/hyphen, dinormalisasi lowercase. Field query asing, query berulang, dan angka invalid ditolak 400. Karakter wildcard SQL pada q tidak berperilaku sebagai wildcard karena query memakai pencarian substring literal. Kategori valid tetapi tidak dikenal menghasilkan daftar kosong.

Contoh:

```http
GET /api/v1/partners?q=Malang&material=plastic&limit=20&offset=0
```

Envelope daftar:

```json
{"data":[],"pagination":{"limit":20,"offset":0,"has_more":false}}
```

Urutan daftar adalah name kemudian UUID; has_more dihitung dengan mengambil satu hasil tambahan. Untuk halaman berikutnya tambahkan limit ke offset, selama tidak melebihi batas offset. Tidak ada jumlah total, sort jarak, atau cursor snapshot. Data kosong dikirim sebagai array kosong, bukan null.

## Data lokasi

```json
{
  "data": {
    "id":"20000000-0000-4000-8000-000000000001",
    "name":"Bank Sampah Tunas",
    "area":"Kawasan Malang",
    "address":"Alamat simulasi, Malang",
    "phone":"",
    "opening_hours":"Senin-Sabtu, 08.00-16.00",
    "description":"Lokasi simulasi development",
    "location":{"latitude":-7.9775,"longitude":112.6304},
    "accepted_materials":[{"id":"10000000-0000-4000-8000-000000000001","name":"Plastik","slug":"plastic","code":"PLASTIC","handling_guide":"","requires_special_handling":false}],
    "hours":[{"day_of_week":0,"opens_at":"","closes_at":"","is_closed":true}]
  }
}
```

Contoh hours di atas hanya menunjukkan satu hari; seed mengisi tujuh hari. Hari 0=Minggu, 1=Senin, sampai 6=Sabtu. Hari tutup memakai jam kosong pada JSON dan NULL pada database. Jam terstruktur memakai HH:MM. Migration saat ini mendukung satu interval per hari, tanpa interval lintas tengah malam. opening_hours adalah ringkasan teks tersimpan untuk kebutuhan layar; saat fitur pengelolaan jam dibuat, penulis harus menjaga ringkasan dan hours konsisten. Belum ada fitur open-now atau timezone lokasi.

partnerID menggunakan UUID canonical, bukan ID dummy tunas/lestari. UUID salah format menghasilkan 400; lokasi tidak ada/pending/rejected/suspended atau pemilik inactive menghasilkan 404 yang sama. Response tidak membocorkan owner UID atau status internal lokasi tersembunyi. Status aktif tidak dihitung dari jam buka: lokasi tetap dapat ditampilkan saat sedang tutup.

## Kategori dan mapping Android

Response kategori memakai id, name, slug, code, handling_guide, requires_special_handling. Empat seed awal: plastic/PLASTIC/Plastik, paper/PAPER/Kertas, metal/METAL/Logam, glass/GLASS/Kaca. Label umum Semua adalah pilihan UI tanpa material filter, bukan row kategori.

DTO Android perlu memetakan accepted_materials[].code ke WasteMaterial, nama JSON snake_case ke field Kotlin, dan UUID server ke navigasi detail. Jangan terus memakai hardcoded Routes.partnerDetail("tunas"). Label/slug baru harus ditangani sebelum dipetakan ke enum; tidak semua kategori masa depan pasti mempunyai enum di APK sekarang. JSON ini tidak mengirim Android resource ID.

## Migration dan seed development

00003_create_partners_and_categories.sql membuat partner_status, waste_categories, partners, partner_hours, dan partner_accepted_waste beserta constraint/index/RLS/revoke. partners mempunyai mitra_uid unique; area dan opening_hours ditambahkan untuk memenuhi model FE, melampaui DBML awal yang diarsipkan. grants ke anon/authenticated tidak dibuka; Go harus memakai koneksi database dengan izin yang sesuai.

Jalankan Goose sampai migration 00003 pada database development terlebih dahulu. Migration tidak otomatis memasukkan konten atau membuat akun Firebase.

```powershell
goose -dir database/migrations up
go run ./cmd/seed-partners --demo --mitra-uid <UID_MITRA_DEVELOPMENT>
```

DATABASE_URL pada proses harus menunjuk database development; command tidak otomatis memuat .env. Daftarkan akun mitra tersebut lewat Firebase dan POST /api/v1/accounts dahulu. Seed memeriksa role mitra, status active, serta partner_registrations pending/approved. Akun user/suspended/rejected atau akun yang sudah mempunyai lokasi lain ditolak. Tidak ada role akun yang dinaikkan atau password dibuat.

Seed memasukkan empat kategori dan satu lokasi Tunas dengan koordinat contoh frontend, alamat simulasi, telepon kosong, jam Senin-Sabtu 08-16, Minggu tutup, menerima plastik/kertas/logam. Lokasi dibuat verified dan pendaftaran usaha pending diubah approved oleh command development eksplisit ini. Ini pengecualian seed demo yang dikelola developer, bukan fitur persetujuan admin atau hak endpoint publik. Angka/label tidak menyatakan usaha nyata.

ID demo stabil, ulang command dengan akun sama tidak menambah kategori/lokasi/jam/material yang sama dan tidak menimpa field lokasi lama. Konflik ID demo dengan owner berbeda atau lokasi/status lama ditolak. Seluruh proses seed memakai satu transaksi; jangan memakai seed sebagai alat edit/persetujuan produksi.

## Error dan batas pengujian

Error mengikuti envelope error.code/error.message: 400 invalid_partner_query, 401 unauthorized, 404 partner_not_found, 504 request_timeout, dan 500 internal_error. Deadline query lima detik. SQL memakai parameter binding dan pembacaan rows ditutup. Daftar material/jam disusun terurut pada query.

Tes service dan HTTP disiapkan menggunakan fixture untuk validasi, pagination, bentuk respons, detail tidak ditemukan, kategori, dan auth. Fixture bukan bukti bahwa SQL filter/status/join bekerja. Migration 00003 juga ditambahkan ke daftar SQL Up/Down fixture integration akun; Docker/tes PostgreSQL sebelumnya masih tertunda. Seed, filtering SQL, constraint koordinat/jam, serta visibilitas owner suspended perlu dibuktikan pada PostgreSQL nyata.

Favorit, pengelolaan/verifikasi profil usaha lewat aplikasi, radius/GPS, kalkulator tarif, setoran, dan QR belum dibuat pada batch ini. Penelitian environment Docker aplikasi tetap DEV-01 terpisah.

Hasil verifikasi lokal: 2 tes service dan 19 tes HTTP seluruh fitur lolos; full suite 21 passed in 12 packages dengan binary tes stripped. Build seluruh package exit 0. Status ini tidak membuktikan perilaku PostgreSQL/seed; suite database tetap SKIP.
