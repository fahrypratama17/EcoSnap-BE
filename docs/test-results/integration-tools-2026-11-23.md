# Paket alat integrasi frontend — 23 November 2026

## Cakupan

Tidak ada endpoint, migration, atau aturan bisnis baru. Paket ini menambahkan OpenAPI 3.0.3, koleksi/environment Postman, panduan eksekusi, dan smoke flow Home, laporan/PDF, inbox serta bantuan. Source Android, `.env.example`, Firebase nyata dan database remote tidak diubah.

## Validasi artefak

- `openapi-spec-validator` 0.9.0 menerima `docs/api/openapi.yaml`: 46 operasi. Struktur diperiksa ulang setelah memperbaiki empat properti `description` yang semula salah dikutip sebagai string YAML.
- Koleksi Postman diterima JSON Schema resmi Postman v2.1. Schema diperoleh dari `https://schema.postman.com/json/collection/v2.1.0/collection.json`; fetch Python ke URL lama yang redirect sempat mendapat 403, lalu reader mengambil schema resmi dan validator menggunakan salinan sementara.
- Environment yang dibagikan tidak berisi token. Token dua peran diisi masing-masing pengembang; tidak ada login dengan password dalam koleksi.

## Harness offline dan guard jaringan

PowerShell 5.1 parser menerima `smoke-test.ps1`, `smoke-flow-checks.ps1`, dan `test-smoke-flows.ps1`. Eksekusi `scripts/test-smoke-flows.ps1` terakhir: **84 PASS, 0 FAIL**. Pada run pertama satu assertion harness gagal karena indexing scalar string; assertion diperbaiki agar memeriksa daftar request PATCH secara eksplisit, tanpa mengurangi syarat tidak ada mutasi preflight.

Harness menjalankan validator/helper produksi dengan transport in-memory: membedakan counter bulanan/all-time, membuka ID kartu, menolak akses lintas akun, mendeteksi PDF rusak dan header salah, mencocokkan total laporan serta event inbox, menguji replay tiket, batas tiket dan pemulihan preferensi pada kegagalan.

Guard koleksi diuji melalui Newman terhadap server spy loopback untuk dua skenario: token wajib kosong dan write ke host di luar allowlist tanpa opt-in. Keduanya menghasilkan **0 request HTTP terkirim**, 0 percobaan request pada statistik Newman. Guard memakai `pm.execution.skipRequest()`, bukan hanya `throw`.

## Server HTTP dan PostgreSQL aktual

Harness sementara memasang handler `bootstrap.NewServer` asli pada listener loopback dan PostgreSQL portable disposable dengan migration 00001–00012. Hanya boundary Firebase yang diganti verifier/login sintetis (`plain-user`, `owner-a`); request smoke dan Newman benar-benar melewati HTTP, service, repository dan SQL. Tidak memakai Supabase atau credential Firebase.

| Eksekusi | Hasil |
|---|---|
| Smoke `-SettingsOnly` | 47 PASS, 0 FAIL; 52 HTTP; 0 POST finansial/tiket baru |
| Smoke `-FlowsOnly` | 86 PASS, 0 FAIL; 90 HTTP; 0 POST finansial/tiket baru |
| Full smoke dengan `-CreateSupportTicket` | 186 PASS, 0 FAIL; 185 HTTP; satu tiket baru |
| Newman | 104 request, 354 assertion; 0 gagal |
| Cakupan respons sukses | Semua 46 operasi |
| Validasi schema respons JSON | 435 respons cocok |
| Validasi schema request sukses | 58 body cocok |
| PDF dan respons kosong | 8 PDF dengan magic bytes benar; 4 respons 204 tanpa body |

Total **447 observasi HTTP** termasuk request langsung tambahan untuk registrasi user/mitra baru dan profil pending. Newman menjalankan koleksi tanpa dua request registrasi untuk akun yang sudah ada dan tanpa lima helper manual; pagination tetap dijalankan. Preferensi asli fixture dipastikan pulih setelah smoke; Postman tetap memiliki efek samping yang didokumentasikan.

Validasi respons awal menemukan `nullable: true` dengan `allOf` masih menolak `missions: null` dan `leaderboard.me: null`. Schema nullable object diperbaiki tanpa mengubah runtime. Validasi terakhir menerima semua request/response yang dicatat, tanpa mismatch. Pemeriksaan schema tambahan menerima Home dengan kartu/misi `null` dan menolak objek kosong sebagai pengganti tiga entitas tersebut. Ini uji kontrak, bukan pembuktian runtime katalog kosong.

Tes sementara `TestIntegrationToolsProof` PASS; marker `INTEGRATION_TOOLS_RUNTIME_PASS` dan `INTEGRATION_TOOLS_CLUSTER_REMOVED` tercetak. Cluster dihentikan dan dihapus. Harness dan capture sementara tidak masuk commit; tidak ada build/full suite Go ulang karena kode server tidak berubah.


## Cara menjalankan ulang

- Validasi OpenAPI dengan validator OpenAPI 3.0.3 pilihan tim.
- Ikuti [`postman.md`](../api/postman.md) untuk urutan request, prasyarat dan efek samping; jangan menjalankan semua folder pada database nyata.
- Jalankan `./scripts/test-smoke-flows.ps1` untuk harness tanpa jaringan.
- Smoke server aktif: `./scripts/smoke-test.ps1 -ApiKey <FirebaseWebApiKey>`. `-SettingsOnly` membatasi pemeriksaan settings; `-FlowsOnly` menambahkan pemeriksaan flow tanpa transaksi finansial; `-CreateSupportTicket` mengizinkan satu tiket baru. Settings yang diuji dipulihkan.

## Batas bukti

Validasi schema dan harness bukan bukti Android berhasil menjalankan flow. Runtime Firebase/Supabase nyata, migration remote 00011–00012, pembekuan kontrak bersama tim dan generated Kotlin client belum dibuktikan dalam paket ini. QR adalah bearer; collection export harus dibersihkan dari transient QR sebelum dibagikan. Koleksi Postman sengaja tidak memulihkan profil/preferensi/transaksi; efek samping dicatat pada panduannya.
