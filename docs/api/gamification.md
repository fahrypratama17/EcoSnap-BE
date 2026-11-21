# Kontrak misi, badge, dan leaderboard

Status: **kode ada (ADA KODE)** pada branch lokal `dev` menurut D-24 dan D-25; tes PostgreSQL dan HTTP terarah lulus tanpa SKIP ([bukti](../test-results/gamification-2026-11-21.md)). Pemilik melaporkan migration `00001`–`00010` Applied di Supabase development pada 2026-11-21 pukul 17:37. Integrasi Android dan Firebase nyata untuk endpoint gamifikasi belum diuji.

## Aturan (D-24, D-25)

- **Periode mingguan:** Senin 00.00 sampai Senin berikutnya 00.00 zona `Asia/Jakarta` (WIB). Akhir periode **eksklusif**. Jam yang dipakai adalah jam database server; `server_time` dikirim di setiap respons.
- **Progres misi** hanya dari setoran yang **diklaim** user pada minggu berjalan. Tidak ada progres dari game (game ditunda).
- **XP terpisah dari EcoPoints.** XP tidak masuk saldo dan tidak bisa ditukar reward. Satu-satunya sumber XP saat ini adalah klaim hadiah misi.
- **Klaim manual, satu kali per misi per minggu.** Hadiah yang belum diklaim hangus saat periode berakhir. Tampilkan `period_end` sebagai tanggal berakhir.
- **Badge permanen**, diturunkan langsung dari klaim setoran all-time yang tidak dapat diubah, tanpa klaim dan tanpa bonus.
- **Leaderboard:** XP mingguan; hanya user aktif yang opt-in; hanya `display_name` dan `photo_path`; mitra tidak ikut; tanpa hadiah EcoPoints.
- Semua endpoint memerlukan Bearer token. Respons `{"data": …}`; error `{"error":{"code","message"}}`. Query/body asing → 400.

## GET /api/v1/me/missions

Tanpa query.

```json
{"data":{
  "period_start":"…","period_end":"…","server_time":"…","weekly_xp":0,"total_xp":100,
  "missions":[
    {"id":"first_deposit","title":"…","description":"…","metric":"…","unit":"deposit","target":1,"progress":1,"reward_xp":50,"status":"claimable","claimed_at":null},
    {"id":"three_kilograms","title":"…","description":"…","metric":"…","unit":"gram","target":3000,"progress":1200,"reward_xp":100,"status":"in_progress","claimed_at":null}
  ]}}
```

| Misi | Syarat minggu ini | XP |
|---|---|---|
| `first_deposit` | jumlah setoran terklaim ≥ 1 (unit `deposit`) | 50 |
| `three_kilograms` | total berat setoran terklaim ≥ 3000 (unit `gram`) | 100 |

`status`: `in_progress` → `claimable` → `claimed`. `claimed_at` berisi waktu klaim atau `null`. Hitung sisa waktu dari `period_end − server_time`, bukan jam HP.


## POST /api/v1/me/missions/{missionID}/claim

Request harus berupa JSON dengan `Content-Type: application/json`, body tepat `{"period_start":"…"}`. Nilainya wajib timestamp RFC 3339 (RFC 3339Nano diterima) yang persis sama dengan `period_start` dari `GET /api/v1/me/missions`. Kirim token periode yang ditampilkan; ini mencegah retry/permintaan lama mengklaim periode baru setelah pergantian minggu.

```json
{"data":{"mission":{…seperti objek di daftar, status "claimed"…},"xp_awarded":50,"weekly_xp":50,"total_xp":150,"period_start":"…","period_end":"…","server_time":"…"}}
```

| Kondisi | Status | Kode |
|---|---|---|
| Klaim pertama | 201 | - |
| Klaim ulang pada periode yang sama | 200; `xp_awarded` berisi nilai hadiah misi, tetapi tidak ada award ganda | - |
| Body hilang, JSON/timestamp salah, atau field asing | 400 | `invalid_request` |
| `period_start` dari periode lama atau masa depan | 410 | `mission_expired` |
| ID misi tidak dikenal | 404 | `mission_not_found` |
| Target belum tercapai | 409 | `mission_incomplete` |
| Caller mitra | 403 | `gamification_forbidden` |
| Akun tidak active | 403 | `account_forbidden` |
| Profil belum dibuat | 404 | `account_not_found` |
| Timeout | 504 | `request_timeout` (ulangi dengan `period_start` yang sama) |

## GET /api/v1/me/badges

Tanpa query.

```json
{"data":{"server_time":"…","claimed_deposits":3,"claimed_weight_grams":4200,
  "badges":[{"id":"deposit_1","title":"…","description":"…","metric":"…","target":1,"progress":3,"earned":true,"unlocked_at":"…"}]}}
```
`unlocked_at` adalah waktu tercapai atau `null` bila badge belum earned.

| Badge | Target (all-time, terklaim) |
|---|---|
| `deposit_1` (Langkah Pertama) | 1 setoran |
| `weight_10kg` (Pejuang Daur Ulang) | 10000 gram |
| `deposit_10` (Konsisten) | 10 setoran |

Badge dihitung ulang dari data klaim setiap dibaca, sehingga tidak ada endpoint klaim badge dan tidak ada bonus.

## PATCH /api/v1/me/leaderboard-visibility

User aktif mengatur sendiri apakah ingin tampil di leaderboard. Hanya body `{"visible":true}` atau `{"visible":false}` (JSON ketat, `Content-Type: application/json`); `visible` wajib boolean. Nilai bawaan adalah `true`. Update bersifat idempotent dan mengubah `user_settings.show_on_leaderboard`.

```json
{"data":{"visible":false}}
```

| Kondisi | Status | Kode |
|---|---|---|
| Berhasil | 200 | - |
| Body hilang/tidak valid atau field asing | 400 | `invalid_request` |
| Caller mitra | 403 | `gamification_forbidden` |
| Akun tidak active | 403 | `account_forbidden` |


## GET /api/v1/leaderboard

Query `limit` (1–100, default 20); nilai lain → 400 `invalid_query`. Tidak ada cursor/pagination.

```json
{"data":{"period_start":"…","period_end":"…","server_time":"…","limit":20,
  "entries":[{"rank":1,"display_name":"…","photo_path":null,"xp":150}],
  "me":{"rank":1,"xp":150}}}
```

- XP = jumlah hadiah misi yang diklaim pada minggu berjalan; hanya XP > 0 yang tampil.
- Poin sama berperingkat sama dan peringkat berikutnya melompat (1, 1, 3).
- Syarat tampil: akun `user` active dengan `settings.show_on_leaderboard = true`.
- `me` bernilai `null` bila caller tidak memenuhi syarat atau XP 0. UID dan email tidak pernah dikirim.
- Dihitung langsung saat request (tanpa cache/snapshot; D-25). `photo_path` adalah path dari profil, bukan URL lengkap.
- Game belum ada, jadi tidak ada leaderboard game.

## Schema

Migration `database/migrations/00010_create_gamification.sql`: tabel `mission_claims` dan trigger yang menjadikan `mission_claims` serta `deposit_claims` tidak dapat diubah/dihapus, agar progres dan badge turunan tetap sahih. XP terpisah dari `point_ledger`.

## Catatan integrasi Android (temuan audit FE, belum diubah)

- `WeeklyChallengeCard.kt` memakai teks tetap "3 jenis sampah" dan "+50 EcoPoints". Itu **bertentangan** dengan D-24: misi adalah 1 setoran terklaim (50 XP) dan total 3 kg terklaim (100 XP); hadiahnya **XP**, bukan EcoPoints. Ganti dengan `missions[]` dari server (`target`, `progress`, `unit`, `reward_xp`).
- `GamesHomeScreen` memakai progres tetap (1/3) dan "5 hari". Ganti dengan `progress`/`target` dan sisa waktu dari `period_end − server_time`.
- Tautan misi pada `GamesNavGraph` belum punya tujuan; layar misi/badge perlu dibuat sebelum memakai endpoint ini.
- `LocalLeaderboard` memberi peringkat berurutan tanpa seri dan tidak punya foto. Pakai `rank` dari server (seri berperingkat sama) dan `photo_path` (boleh `null`).
- Game tetap ditunda (D-24), bukan dihapus: jangan menampilkan XP atau progres game palsu, dan jangan menambah XP dari sisi klien.
- Pengaturan visibilitas kini tersedia: `PATCH /api/v1/me/leaderboard-visibility` (body `{"visible":true|false}`); lihat kontrak di atas. Default `true`.
