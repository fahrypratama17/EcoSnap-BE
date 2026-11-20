# Kontrak pembuatan dan detail setoran

Status: implementasi backend pada feat/deposit-creation (DEPOSIT-03). Bukti: [laporan batch](../test-results/deposits-2026-11-20.md). Aturan hitung sama dengan [estimasi poin](point-estimates.md) (D-17) dan memakai fungsi yang sama di server.

## Batas batch

- Setoran dibuat oleh akun **mitra active** yang memiliki lokasi **verified** (D-14, D-15). Lokasi diambil dari akun caller; client tidak mengirim partner_id.
- Status satu-satunya: `created`. QR, reissue, klaim, pembatalan, penolakan fisik, dan penerima setoran belum ada; transisinya menunggu Q-08/Q-16.
- Tidak ada perubahan wallet atau ledger. Saldo hanya boleh berubah melalui klaim atomik pada batch QR.
- Tidak ada endpoint edit. Kebijakan perubahan setoran masih Q-08.

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

Migration `00006_create_deposits.sql`: `deposits` (unique `partner_id, idempotency_key`, `request_hash` SHA-256 isi ter-normalisasi) dan `deposit_items` (snapshot; CHECK `exact_points_e5 = weight_grams * rate_hundredths`). RLS aktif dan akses anon/authenticated dicabut seperti tabel lain.
