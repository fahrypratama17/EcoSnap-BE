# Kontrak reward dan voucher

Status: implementasi backend pada feat/reward-redemption (REWARD-01..04). Bukti: [laporan](../test-results/rewards-2026-11-21.md). Voucher bersifat simulasi (D-08); aturan pemakaian, masa berlaku, dan refund mengikuti D-20.

## Aturan (D-20)

- Voucher berlaku **7 hari** sejak penukaran, dihitung dari jam server. Tepat pada `expires_at` sudah kedaluwarsa.
- Pemakaian sederhana: user menekan "tandai sudah dipakai". Tidak ada validasi kasir/mitra.
- **Tanpa refund.** Penukaran yang gagal tidak memotong poin sama sekali karena saldo, stok, riwayat poin, dan voucher disimpan dalam satu transaksi. Voucher kedaluwarsa atau sudah dipakai tidak mengembalikan poin.
- Hanya akun role `user` yang active yang dapat menukar.

## GET /api/v1/rewards

Reward aktif, urut biaya termurah (maks. 100). `available=false` bila stok habis.

```json
{"data":[{"id":"…","name":"Voucher Pulsa 5.000","description":"…","terms":"…","cost_points":500,"stock":50,"available":true,"is_demo":true}]}
```

`GET /api/v1/rewards/{rewardID}` mengembalikan satu objek yang sama; reward nonaktif → 404 `reward_not_found`.

## POST /api/v1/rewards/{rewardID}/redeem

Tanpa body. Header `Idempotency-Key` wajib (8-128 karakter `A-Z a-z 0-9 - _`); Android membuat satu key per tombol "Tukar" dan memakai ulang key saat retry.

```json
{"data":{
  "voucher":{"id":"…","reward_id":"…","reward_name":"Voucher Pulsa 5.000","cost_points":500,"code":"ECO-7KQ2-M9XA","status":"issued","created_at":"…","expires_at":"…+7 hari","used_at":null},
  "wallet_balance":250
}}
```

| Kondisi | Status | Kode |
|---|---|---|
| Berhasil | 201 | - |
| Key + reward sama diulang | 200, voucher sama, tidak dipotong lagi | - |
| Key sama untuk reward lain | 409 | idempotency_conflict |
| Saldo kurang | 409 | insufficient_points |
| Stok habis | 409 | out_of_stock |
| Reward tidak ada/nonaktif | 404 | reward_not_found |
| Caller mitra | 403 | redeem_forbidden |
| Key hilang/tidak valid | 400 | invalid_idempotency_key |
| Timeout | 504 | request_timeout (ulangi dengan key sama) |

Semua kegagalan di atas tidak memotong poin maupun stok. Riwayat poin menampilkan entri `source: "redemption"`, `amount` negatif, dengan `redemption.reward_name`.

## GET /api/v1/me/vouchers

Voucher milik caller, terbaru di atas. Query `limit`, `cursor` (sama dengan [riwayat poin](wallet-history.md)), `status` = `issued` | `used` | `expired`.

## GET /api/v1/me/vouchers/{voucherID}

Hanya pemilik. Voucher orang lain → 404 `voucher_not_found` (kode voucher tidak bocor).

## POST /api/v1/me/vouchers/{voucherID}/use

Tanpa body. 200 dengan voucher `status: "used"`. Mengulang pada voucher yang sudah dipakai mengembalikan hasil yang sama. Voucher kedaluwarsa → 410 `voucher_expired`.

## Schema dan seed

Migration `00008_create_rewards.sql`: `rewards` (stok ≥ 0), `redemptions` (snapshot nama/biaya, kode unik, unique `user_uid, idempotency_key`, status `issued|used`; `expired` diturunkan dari waktu), `point_ledger.redemption_id` dengan CHECK debit negatif. Enum `ledger_source` pada 00007 memuat `redemption` (00007 belum diterapkan ke Supabase).

Seed demo: `DATABASE_URL=… go run ./cmd/seed-rewards --demo` membaca `data/seeders/dev/rewards.json`. Re-run tidak menimpa reward atau mengubah stok yang sudah berkurang; perubahan nama/biaya untuk ID yang sama ditolak.
