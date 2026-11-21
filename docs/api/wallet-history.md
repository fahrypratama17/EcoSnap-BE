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

Mitra dengan lokasi pending/suspended/rejected tetap dapat membaca riwayat dan ringkasan setorannya (read-only, D-21); membuat setoran/QR tetap ditolak.

## Filter riwayat (HISTORY-01)

| Endpoint | Parameter tambahan |
|---|---|
| `GET /api/v1/me/point-history` | `type` = `deposit` \| `redemption`, `from`, `to` |
| `GET /api/v1/partner/deposits` | `status`, `from`, `to` |

- `from` inklusif, `to` eksklusif, format RFC 3339 dengan zona waktu, mis. `2026-11-01T00:00:00Z`. Tanda `+` pada offset wajib di-encode (`%2B07:00`), atau gunakan `Z`.
- `from` harus sebelum `to`. Filter digabung dengan cursor; cursor dari query berbeda tidak boleh dipakai ulang.
- Android menentukan batas hari/bulan menurut zona waktu pengguna lalu mengirimnya sebagai timestamp.

## GET /api/v1/me/deposits/{depositID}

Detail setoran (item, berat, tarif snapshot) untuk **user yang mengklaimnya**, dari tombol detail di riwayat poin. Format sama dengan detail setoran mitra. User lain, mitra, atau setoran yang belum diklaim → 404 `deposit_not_found`.

## GET /api/v1/me/home (HOME-01)

```json
{"data":{
  "wallet":{"balance":200,"total_earned":300,"total_spent":100,"updated_at":"…"},
  "deposits_claimed":2,
  "total_weight_kg":"2.000",
  "active_vouchers":1,
  "recent_activity":[ …maks. 3 entri riwayat poin… ]
}}
```

Angka bersifat sepanjang waktu (all-time). `active_vouchers` = voucher issued yang belum kedaluwarsa. Misi/badge ditambahkan setelah Q-07.

## GET /api/v1/partner/home

```json
{"data":{
  "partner_id":"…","business_name":"Bank Sampah Tunas","partner_status":"verified",
  "deposits":{"created":1,"qr_issued":1,"claimed":2},
  "claimed_weight_kg":"2.000","claimed_points":300,
  "recent_deposits":[ …maks. 3 ringkasan setoran… ]
}}
```

`partner_status` dipakai Android untuk menampilkan banner "menunggu verifikasi"/"disuspend". Akun tanpa lokasi atau bukan mitra → 403 `deposit_forbidden`.
