# Kontrak bantuan (FAQ) dan tiket dukungan

Status: implementasi backend. Semua endpoint memakai `Authorization: Bearer <Firebase ID token>`; identitas hanya dari token (tanpa `uid` dari klien). Akun harus **active**; role `user` maupun `mitra` boleh, mitra tidak perlu lokasi terverifikasi. Akun tidak dikenal → 404 `account_not_found`, akun tidak aktif → 403 `account_forbidden`.

Verifikasi lokal pada worktree integrasi: tes PostgreSQL/HTTP/service/CLI lulus, termasuk retry paralel, batas tiket, isolasi dan balasan CLI terlihat oleh pengguna ([bukti integrasi dev](https://github.com/fahrypratama17/EcoSnap-BE/blob/dev/docs/test-results/backend-support-reports-notifications-2026-11-21.md)). Firebase/Supabase nyata dan Android untuk endpoint ini belum diuji.

## Batasan yang harus disampaikan ke pengguna

- **Tidak ada kontak publik** (email, telepon, chat). Satu-satunya jalur adalah tiket di aplikasi.
- **Tidak ada email, push, atau notifikasi real-time.** Pengelola membalas secara manual, kapan pun sempat. Pengguna melihat balasan **hanya dengan membuka ulang** daftar atau detail tiketnya (`GET`). Tampilkan "Belum ada balasan" saat `staff_response` null; jangan menjanjikan waktu balasan.
- Tidak ada lampiran. Alamat email tidak dikirim di tiket; tidak ada field `contact_email`. Balasan tidak pernah dikirim ke email profil.
- Tiket tidak mengubah poin, XP, voucher, atau saldo apa pun, dan tidak ada banding finansial. Tiket tidak dapat dihapus atau diedit pengguna.
- Konten FAQ adalah **teks demo** (`is_demo: true`), bukan kebijakan final.

## GET /api/v1/help/faqs

Tanpa query. Mengembalikan daftar tetap yang sudah ditinjau (Bahasa Indonesia):

```json
{"data":[{"id":"qr-setoran","category":"setoran","question":"…","answer":"…","is_demo":true}]}
```

Isi: QR setoran adalah token pembawa dan berlaku 10 menit; voucher berlaku 7 hari dan poin tidak dikembalikan; EcoPoints berbeda dari XP; arti label demo; cara melapor tanpa kontak publik. Semua teks plain text.

## POST /api/v1/me/support-tickets

Header wajib `Idempotency-Key` (8-128 karakter `A-Za-z0-9_-`, sama seperti setoran/penukaran) dan `Content-Type: application/json`. Body tepat tiga field, tidak boleh ada field lain (termasuk `status`, `contact_email`):

```json
{"kind":"problem","subject":"QR tidak terbaca","message":"Pemindai mitra gagal membaca QR saya."}
```

| Field | Aturan |
|---|---|
| `kind` | `problem` \| `feedback` |
| `subject` | 1-150 karakter setelah di-trim, satu baris |
| `message` | 1-4000 karakter setelah di-trim (dihitung per karakter Unicode, bukan byte); baris baru dibolehkan |

Karakter kontrol ditolak. Spasi di tepi dibuang dan `\r\n` dijadikan `\n` sebelum disimpan dan di-hash. Server menghitung hash kanonik `kind+subject+message`; klien tidak mengirim hash.

Respons `data`:

```json
{"data":{"id":"…","kind":"problem","subject":"…","message":"…","status":"open","staff_response":null,"responded_at":null,"created_at":"…","updated_at":"…"}}
```

| Kondisi | Status | Kode |
|---|---|---|
| Tiket baru | 201 | - |
| Retry key yang sama dengan isi yang sama | 200, tiket yang sama, tidak ada duplikat | - |
| Key yang sama, isi berbeda | 409 | `idempotency_conflict` |
| Key hilang/salah format | 400 | `invalid_idempotency_key` |
| Kind/subject/message tidak valid | 400 | `invalid_ticket` |
| JSON tidak valid, field tak dikenal, JSON tambahan, ada query string | 400 | `invalid_request` / `invalid_query` |
| Bukan `application/json` | 415 | `unsupported_media_type` |
| Body > 64 KiB | 413 | `body_too_large` |
| Sudah ada 20 tiket belum selesai (`open` + `in_progress`) | 429 | `ticket_limit_reached` |

Retry atas tiket yang sudah dibuat selalu dicek lebih dulu daripada batas 20, jadi retry setelah timeout tetap aman. Idempotency-Key berlaku per akun. Jika timeout (504), ulangi dengan key yang sama.

## GET /api/v1/me/support-tickets

Tiket milik pemanggil, terbaru di atas. Query: `limit` (1-100, default 20), `cursor` (dari `pagination.next_cursor`), `status` (`open` \| `in_progress` \| `resolved`). Parameter lain atau ganda → 400 `invalid_query`.

```json
{"data":[{…tiket…}],"pagination":{"limit":20,"next_cursor":null}}
```

## GET /api/v1/me/support-tickets/{ticketID}

Satu tiket milik pemanggil. ID tidak valid, tidak ada, atau milik akun lain → 404 `ticket_not_found` (identik, agar keberadaan tiket orang lain tidak bocor).

`status`: `open` (baru) → `in_progress` → `resolved`. Nilainya hanya berubah oleh pengelola; tidak ada endpoint publik yang mengubah status atau balasan, dan tidak ada PATCH/PUT/DELETE. `staff_response` dan `responded_at` null sampai pengelola membalas; `updated_at` berubah tiap pengelola mengubah tiket.

## Alur pengelola (CLI pemilik repo)

Belum ada role admin, jadi pengelola memakai `cmd/support-tickets` dengan `DATABASE_URL` pemilik (tanpa Firebase). Jalankan hanya ke database yang dituju. Perubahan wajib `--confirm`. Keluaran JSON; pesan galat tidak pernah memuat connection string atau password.

```powershell
$env:DATABASE_URL = "postgresql://…"
go run ./cmd/support-tickets list --status open --limit 20          # semua akun, terlama dulu; memuat user_uid & isi pesan
go run ./cmd/support-tickets list --id <ticketID>
go run ./cmd/support-tickets reply --id <ticketID> --message "Terima kasih, sudah kami cek." --status in_progress --confirm
go run ./cmd/support-tickets reply --id <ticketID> --message-file jawaban.txt --confirm
go run ./cmd/support-tickets status --id <ticketID> --status resolved --confirm
```

- `reply` menyimpan teks (1-4000 karakter, tidak boleh kosong) dan waktu server. Status **tidak** berubah otomatis dan tidak ada pengakuan otomatis; ubah hanya dengan `--status` eksplisit.
- Hanya balasan **terakhir** yang disimpan. Membalas tiket yang sudah punya balasan ditolak kecuali `--replace` (balasan lama hilang, tanpa riwayat).
- Perpindahan status yang diizinkan: `open → in_progress | resolved`, `in_progress → resolved`, `resolved → in_progress` (buka kembali). Lainnya, termasuk ke status yang sama, ditolak. Status dan balasan pada satu perintah diterapkan bersamaan atau tidak sama sekali, di bawah kunci baris.
- Pesan dan subjek asli tidak pernah berubah (trigger database), dan tiket tidak dapat dihapus. Tidak ada efek finansial: poin, XP, dan voucher tidak disentuh.

## Schema

Migration `00012_create_support_tickets.sql`: tabel `support_tickets` (`UNIQUE(user_uid, idempotency_key)`), enum `support_ticket_kind` dan `support_ticket_status`, trigger penjaga data asli, RLS aktif dan `REVOKE ALL` dari `anon`/`authenticated` (backend memakai koneksi pemilik). Batas 20 tiket belum selesai ditegakkan di transaksi pembuatan dengan advisory lock per akun, jadi permintaan paralel tidak melewati batas.
