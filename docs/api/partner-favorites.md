# Kontrak favorit mitra

Pembaruan verifikasi: full suite PostgreSQL lokal portable kini PASS tanpa SKIP, termasuk akun, query mitra, favorit, dan CLI seed. Lihat [laporan PostgreSQL](../test-results/postgres-favorites.md). Catatan belum diuji/SKIP di bagian historis berikut menggambarkan pemeriksaan sebelumnya. Supabase, Firebase nyata, dan Android tetap belum diuji end-to-end.


Fitur memenuhi tombol hati dan state favoriteIds pada PartnerMapScreen. Frontend belum diubah. Favorit bersifat per akun aplikasi aktif; UID diperoleh dari token Firebase. Tidak menerima UID user lain melalui request.

## Endpoint

| Method/path | Hasil |
|---|---|
| GET /api/v1/me/favorites | Array partner_id dan created_at, dengan pagination |
| PUT /api/v1/me/favorites/{partnerID} | Tambah favorit, 204 tanpa body |
| DELETE /api/v1/me/favorites/{partnerID} | Hapus favorit, 204 tanpa body |

Ketiganya membutuhkan Authorization Bearer. Caller harus memiliki akun PostgreSQL active. Akun belum terdaftar menghasilkan 404 account_not_found; suspended/deleted menghasilkan 403 account_forbidden. Semua akun aktif dapat memakai favorit; tidak ada kewenangan transaksi yang diperoleh dari menambahkan favorit.

PUT/DELETE tidak memerlukan request body dan tidak menerima parameter query. partnerID harus UUID canonical. PUT hanya menerima lokasi verified dengan owner akun mitra active; lokasi tersembunyi dan ID tidak ditemukan sama-sama 404 partner_not_found. PUT berulang tidak menambah row atau mengubah created_at. DELETE berulang pada UUID valid tetap 204, termasuk ketika row tidak ada atau lokasi sudah disembunyikan. DELETE selalu dibatasi user_uid caller.

GET menerima limit default 20 (1-100) dan offset default 0 (0-10000). Parameter asing, duplikat, atau angka tidak valid menghasilkan 400 invalid_partner_query. Hasil diurutkan created_at descending kemudian partner_id. Response kosong memakai array, bukan null:

```json
{"data":[],"pagination":{"limit":20,"offset":0,"has_more":false}}
```

Setiap item berisi partner_id dan created_at RFC3339. Android menggunakan partner_id untuk menyusun himpunan favorit; data lengkap lokasi tetap dari endpoint daftar/detail mitra. Untuk daftar lebih dari satu halaman, ambil halaman berikutnya memakai offset+limit selama dalam batas offset.

Lokasi yang tidak visible disaring dari GET favorit tanpa menghapus row. Jika menjadi visible kembali, favorit muncul lagi kecuali user sudah menghapusnya. Tidak mengirim owner UID, status lokasi tersembunyi, atau favorit user lain. Body/query tidak dapat mengganti identitas caller.

## Implementasi

- domain/entity/partner_favorites.go: item favorit.
- internal/service/partner_favorites.go: validasi ID dan pagination, bentuk hasil kosong/has_more.
- internal/repository/postgres/partner_favorites.go: transaksi penambahan/penghapusan/pembacaan, pemeriksaan akun aktif, dan visibility lokasi.
- internal/delivery/http/handler/partner_favorites.go: pembacaan UID context, query, deadline lima detik, dan response.
- router.go dan bootstrap/app.go: wiring dependency dari pool yang sudah ada.
- database/migrations/00004_create_partner_favorites.sql: UUID row, foreign key, unique(user_uid,partner_id), index listing, RLS, dan pencabutan grant client Supabase.

Pemeriksaan akun memakai shared row lock selama transaksi. Saat menambah, lokasi dan owner juga dikunci shared agar perubahan status tidak meloloskan target yang sudah tidak visible pada operasi tersebut. Pengulangan insert memakai ON CONFLICT DO NOTHING. SQL di-parameterize; tidak menggabungkan UID/input ke teks query.

## Pengujian

Tes HTTP partner_favorites_test.go menguji auth, retry, identitas caller, invalid ID, target tidak ditemukan, dan query UID asing. Store/verifier pengganti membuktikan alur HTTP saja.

Suite PostgreSQL TestPartnerFavoritesIntegration memeriksa 12 insert paralel untuk favorit yang sama, isolasi alice/bob, penghapusan target tersembunyi, retry delete, akun tidak ada/suspended, dan query mitra dari batch terdahulu. Fixture memakai database acak per run pada cluster lokal disposable. Migration fixture sekarang mencakup 00001 sampai 00004 beserta Down terbalik. Hasil aktual PostgreSQL dicatat pada BUILD_PROCESS.md setelah suite dijalankan; keberadaan file tes bukan bukti PASS.

Runner Docker diperbarui agar menjalankan seluruh package, bukan hanya suite akun. Alternatif portable Windows tersedia:

```powershell
./scripts/test-postgres-portable.ps1 -PostgresBin 'PATH/TO/pgsql/bin'
```

Runner portable membuat cluster sementara hanya di loopback dengan data sintetis, memasang environment tes, menjalankan full suite, menghentikan server, lalu membersihkan direktori cluster miliknya. Binary tidak dipasang sebagai service Windows. Ia tidak membaca DATABASE_URL aplikasi. Source binary berasal dari arsip EDB yang dirujuk [halaman Windows PostgreSQL](https://www.postgresql.org/download/windows/) dan [halaman binary EDB](https://www.enterprisedb.com/download-postgresql-binaries?lang=en). Trust auth hanya untuk cluster sintetis yang berumur singkat di loopback.

Migration 00004 tetap perlu diterapkan melalui Goose pada database development aplikasi sebelum endpoint favorit dicoba di server aplikasi. Menjalankan integration test lokal tidak menerapkan migration atau seed ke Supabase, tidak membuktikan credential Firebase, dan tidak mengintegrasikan Android.
