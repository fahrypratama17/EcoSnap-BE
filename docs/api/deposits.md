# Kontrak pembuatan dan detail setoran

Status: implementasi backend pada feat/deposit-creation (DEPOSIT-03, QR-01..03, klaim). Bukti: [setoran](../test-results/deposits-2026-11-20.md), [QR dan klaim](../test-results/deposit-qr-claims-2026-11-20.md). Aturan hitung sama dengan [estimasi poin](point-estimates.md) (D-17) dan memakai fungsi yang sama di server.

## Alur dan aturan

`created` → (mitra terbitkan QR) → `qr_issued` → (pengguna pertama klaim) → `claimed`.

- Setoran dibuat oleh akun **mitra active** yang memiliki lokasi **verified** (D-14, D-15). Lokasi diambil dari akun caller; client tidak mengirim partner_id.
- Setelah QR terbit, setoran **terkunci** (D-19): tidak ada edit/batal. Penolakan barang dilakukan sebelum setoran dibuat. Koreksi poin di masa depan lewat sistem banding (Q-17) sebagai entri ledger baru.
- QR **bearer** (D-18): siapa pun akun user active yang mengklaim pertama mendapatkan poin. Mitra tidak dapat mengklaim.
- Saldo hanya berubah pada klaim; klaim, entri `point_ledger`, update wallet, pemakaian token, dan status setoran satu transaksi.

## POST /api/v1/deposits

Header: `Authorization: Bearer <Firebase ID token>`, `Content-Type: application/json`, `Idempotency-Key` wajib (8-128 karakter `A-Z a-z 0-9 - _`). Android membuat satu key per percobaan setoran dan memakai ulang key yang sama ketika mengulang request akibat timeout/jaringan.

```json
{"items":[
  {"category_id":"10000000-0000-4000-8000-000000000001","weight_kg":"2.500"},
  {"category_id":"10000000-0000-4000-8000-000000000002","weight_kg":"1.000"}
]}
```

Validasi item identik dengan estimasi: berat string desimal bertitik maksimal 3 desimal, 1-50 kategori unik, total ≤ 1000 kg. Field selain `items` (misalnya total_points, partner_id, tarif) dan query parameter ditolak.

Server dalam satu transaksi: mengunci akun dan lokasi (agar suspensi tidak lolos di tengah), mengunci key, memilih tarif aktif lokasi atau fallback global, menghitung total, lalu menyimpan header dan snapshot item (nama kategori, rate_id, tarif, berat, hasil persis).

| Kondisi | Status | Kode |
|---|---|---|
| Baru dibuat | 201 | - |
| Key + isi sama diulang | 200, setoran yang sama | - |
| Key sama, isi berbeda | 409 | idempotency_conflict |
| Key hilang/tidak valid | 400 | invalid_idempotency_key |
| Item tidak valid | 400 | invalid_estimate |
| Material tidak diterima lokasi | 400 | waste_not_accepted |
| Kategori tidak ada | 404 | waste_category_not_found |
| Tarif aktif tidak tersedia | 409 | point_rate_unavailable |
| Bukan mitra / lokasi belum verified | 403 | deposit_forbidden |
| Akun suspended/deleted | 403 | account_forbidden |
| Profil belum dibuat | 404 | account_not_found |
| Timeout | 504 | request_timeout (ulangi dengan key yang sama) |

Replay mengembalikan snapshot tersimpan walaupun tarif sudah berubah sejak pembuatan.

## GET /api/v1/deposits/{depositID}

Hanya mitra pemilik lokasi setoran yang dapat membaca. Mitra lain, ID tidak dikenal, atau ID tidak valid menerima 404 `deposit_not_found` agar ID tidak membocorkan transaksi lokasi lain. Akses pengguna penerima ditambahkan setelah Q-16.

## Response

```json
{"data":{
  "id":"…uuid…",
  "partner_id":"20000000-0000-4000-8000-000000000001",
  "status":"created",
  "total_weight_kg":"3.500",
  "total_points":455,
  "uses_demo_rates":true,
  "rates_as_of":"2026-11-20T14:00:00Z",
  "created_at":"2026-11-20T14:00:00Z",
  "items":[
    {"category_id":"…0001","category_name":"Plastik","weight_kg":"2.500","rate_id":"…0003","points_per_kg":"150.00","exact_points":"375.00000","is_demo":true},
    {"category_id":"…0002","category_name":"Kertas","weight_kg":"1.000","rate_id":"…0002","points_per_kg":"80.00","exact_points":"80.00000","is_demo":true}
  ]
}}
```

Format item sama dengan estimasi. `total_points` adalah floor dari jumlah `exact_points`.

## Schema

Migration `00006_create_deposits.sql`: `deposits` (status `created|qr_issued|claimed`, unique `partner_id, idempotency_key`, `request_hash` SHA-256 isi ter-normalisasi) dan `deposit_items` (snapshot; CHECK `exact_points_e5 = weight_grams * rate_hundredths`). Migration `00007_create_deposit_qr_and_ledger.sql`: `deposit_qr_tokens` (hanya hash SHA-256, histori semua penerbitan, maksimal satu token hidup per setoran), `deposit_claims` (PK deposit_id = satu klaim), `point_ledger` (append-only, `idempotency_key = deposit:<id>`). RLS aktif dan akses anon/authenticated dicabut seperti tabel lain.

## POST /api/v1/deposits/{depositID}/qr

Mitra pemilik menerbitkan QR. Panggilan berikutnya = **reissue**: token lama langsung tidak berlaku, token baru berlaku 10 menit sejak reissue (D-05, D-06). Tanpa body.

```json
{"data":{
  "deposit_id":"…",
  "token":"<43 karakter base64url>",
  "issued_at":"2026-11-20T14:00:00Z",
  "expires_at":"2026-11-20T14:10:00Z",
  "warning":"QR ini dapat diklaim oleh siapa pun yang memindainya pertama kali. Tunjukkan hanya kepada penyetor dan jangan bagikan foto atau tangkapan layarnya."
}}
```

- Android meng-encode `token` apa adanya ke QR dan **wajib menampilkan `warning`** di layar QR.
- Token hanya dikembalikan sekali; server menyimpan hash. Jika layar hilang, terbitkan ulang.
- Countdown memakai `expires_at` dari server, bukan jam perangkat.
- 201 sukses; 404 `deposit_not_found` (bukan milik caller); 403 `deposit_forbidden`/`account_forbidden`; 409 `deposit_already_claimed`.

## POST /api/v1/deposit-claims/preview dan POST /api/v1/deposit-claims

Body keduanya: `{"token":"<hasil scan>"}`. Token tidak pernah dikirim lewat URL/query. Hanya akun role `user` yang active.

Preview tidak mengubah apa pun dan mengembalikan `deposit_id, partner_id, partner_name, total_weight_kg, total_points, uses_demo_rates, expires_at, items`. Menutup preview tidak membatalkan setoran.

Klaim mengembalikan `deposit_id, partner_id, points, claimed_at, wallet_balance`. 201 klaim baru; 200 bila pengklaim yang sama mengulang (respons sebelumnya hilang) — tidak dikreditkan dua kali.

| Kondisi | Status | Kode |
|---|---|---|
| Token tidak dikenal/format salah | 404 | qr_invalid |
| Token sudah diganti reissue | 410 | qr_revoked |
| Lewat atau tepat pada `expires_at` | 410 | qr_expired |
| Sudah diklaim orang lain | 409 | deposit_already_claimed |
| Caller mitra | 403 | claim_forbidden |
| Akun suspended/deleted | 403 | account_forbidden |

Batas yang diketahui: wallet.balance masih INTEGER (maks ~2,1 miliar); kredit yang melampaui batas ditolak seluruhnya (rollback, 500).
