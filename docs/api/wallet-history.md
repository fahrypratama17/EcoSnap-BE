# Kontrak saldo dan riwayat poin

Status: implementasi backend pada feat/wallet-history (WALLET-01). Bukti: [laporan](../test-results/wallet-history-2026-11-21.md). Semua endpoint memakai `Authorization: Bearer <Firebase ID token>`; UID selalu dari token, bukan dari query.

## GET /api/v1/me/wallet

Akun active (user atau mitra). Query apa pun ditolak.

```json
{"data":{"balance":750,"total_earned":750,"total_spent":0,"updated_at":"2026-11-21T08:00:00Z"}}
```

`balance` selalu sama dengan jumlah `amount` pada riwayat poin.

## GET /api/v1/me/point-history

Riwayat ledger milik caller, terbaru di atas. Query: `limit` (1-100, default 20), `cursor`.

```json
{
  "data":[{
    "id":"…",
    "amount":150,
    "source":"deposit",
    "created_at":"2026-11-21T08:00:00Z",
    "deposit":{"id":"…","partner_id":"…","partner_name":"Tunas","total_weight_kg":"1.000"}
  }],
  "pagination":{"limit":20,"next_cursor":"…atau null"}
}
```

- `amount` bertanda: kredit positif. Debit reward atau koreksi banding (Q-17) akan muncul sebagai entri negatif dengan `source` baru; entri lama tidak diubah.
- `deposit` hanya ada untuk `source = deposit`.
- Satu klaim = satu entri (unique `deposit:<id>`).

## GET /api/v1/partner/deposits

Setoran milik lokasi mitra caller (mitra active, lokasi verified). Query: `limit`, `cursor`, `status` (`created` | `qr_issued` | `claimed`, opsional).

```json
{
  "data":[{"id":"…","status":"claimed","total_weight_kg":"1.000","total_points":150,"uses_demo_rates":true,"created_at":"…","claimed_at":"…"}],
  "pagination":{"limit":20,"next_cursor":null}
}
```

Identitas pengklaim tidak ditampilkan ke mitra. `claimed_at` null selama belum diklaim. Detail item per setoran tetap lewat `GET /api/v1/deposits/{id}`.

## Pagination cursor

- Urutan `created_at DESC, id DESC`. Android meneruskan `next_cursor` apa adanya ke request berikutnya; berhenti bila `null`.
- Cursor tidak perlu dipahami client dan bisa berubah format; jangan disimpan permanen.
- Entri baru yang masuk saat user scroll tidak menggeser halaman berikutnya (tidak ada duplikat/terlewat). Entri baru tersebut muncul saat refresh dari halaman pertama.

## Error

| Kondisi | Status | Kode |
|---|---|---|
| limit/cursor/status tidak valid, parameter asing atau ganda | 400 | invalid_query |
| Profil belum dibuat | 404 | account_not_found |
| Akun suspended/deleted | 403 | account_forbidden |
| Bukan mitra / lokasi belum verified (partner/deposits) | 403 | deposit_forbidden |
| Timeout | 504 | request_timeout |

Batas: mitra yang lokasinya disuspend tidak dapat melihat riwayat setorannya; perubahan aturan menunggu Q-04.
