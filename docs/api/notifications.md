# Notifikasi dalam aplikasi

Kotak masuk notifikasi dalam aplikasi untuk pengguna dan mitra aktif. **Tidak ada push, email, atau SMS**: notifikasi hanya terlihat saat aplikasi memuat endpoint ini.

Semua endpoint memerlukan `Authorization: Bearer <Firebase ID token>`; pemilik ditentukan dari token, bukan dari parameter klien. Akun yang tidak aktif ditolak (`403`), profil belum ada `404`.

Verifikasi lokal pada worktree integrasi: tes PostgreSQL/HTTP lulus untuk atomicity, preferensi, retry, ownership, inbox/read dan hadiah XP ([bukti integrasi dev](https://github.com/fahrypratama17/EcoSnap-BE/blob/dev/docs/test-results/backend-support-reports-notifications-2026-11-21.md)). Migration 00011 belum diterapkan ke Supabase; Firebase nyata dan Android untuk endpoint ini belum diuji.

## Peristiwa

Notifikasi dibuat oleh trigger database (`AFTER INSERT`) di transaksi yang sama dengan peristiwanya; bila transaksi dibatalkan, notifikasi ikut hilang.

| `type` | Penerima | Sumber | `target.type` / `target.id` |
|---|---|---|---|
| `deposit_credited` | pengguna yang mengklaim | `deposit_claims` | `deposit` / ID setoran (`GET /api/v1/me/deposits/{id}`) |
| `deposit_claimed` | mitra pemilik setoran | `deposit_claims` | `deposit` / ID setoran (`GET /api/v1/deposits/{id}`) |
| `voucher_issued` | pengguna | `redemptions` | `voucher` / ID voucher (`GET /api/v1/me/vouchers/{id}`) |
| `mission_xp_claimed` | pengguna | `mission_claims` | `mission` / ID misi (`GET /api/v1/me/missions`) |

Isi notifikasi tidak memuat token QR, kode voucher, UID, atau identitas pengklaim.

## `GET /api/v1/me/notifications`

Query (semua opsional; parameter lain atau berulang -> `400 invalid_query`):

- `limit`: 1-100, default 20.
- `cursor`: dari `pagination.next_cursor` (urutan `created_at`, `id` menurun; stabil saat ada notifikasi baru).
- `unread=true`: hanya yang belum dibaca. Nilai lain ditolak.

```json
{
  "data": [{
    "id": "uuid", "type": "voucher_issued",
    "title": "Voucher berhasil ditukar",
    "body": "Voucher Kopi sebesar 100 poin telah terbit.",
    "target": {"type": "voucher", "id": "uuid"},
    "created_at": "2026-11-21T03:00:00Z",
    "seen_at": null
  }],
  "unread_count": 3,
  "pagination": {"limit": 20, "next_cursor": null}
}
```

`unread_count` adalah jumlah seluruh notifikasi belum dibaca milik pemanggil (tidak terpengaruh `unread`/`cursor`), dibaca dari snapshot yang sama dengan halaman.

## `PATCH /api/v1/me/notifications/{notificationID}/read`

Tanpa body. Menandai dibaca; idempoten: `seen_at` hanya diisi pertama kali dan tidak berubah pada pemanggilan berikutnya. Mengembalikan `{"data": <notifikasi>}`. ID tidak valid, tidak ada, atau milik orang lain -> `404 notification_not_found`.

## Batasan

- Hanya peristiwa setelah migrasi diterapkan yang dibuat; tidak ada isi ulang riwayat lama.
- Tidak ada kedaluwarsa/penghapusan otomatis dan tidak ada endpoint "tandai semua dibaca".
- Akun tanpa baris pengaturan tidak menerima notifikasi (gagal tertutup). Pengaturan `notification_enabled=false` menahan notifikasi **baru**; yang sudah ada tetap terbaca. Pengaturan dibaca tanpa kunci saat peristiwa terjadi: perubahan yang di-commit bersamaan dengan peristiwa berlaku hanya bila sudah terlihat oleh database saat itu.
- Bukan pemberitahuan keuangan atau banding.
