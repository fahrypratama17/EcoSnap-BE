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

- `amount` bertanda: kredit positif (`source = deposit`), debit negatif untuk penukaran reward (`source = redemption`, dengan objek `redemption`). Koreksi banding (Q-17) belum ada; bila dibuat akan memakai `source` baru, jadi tampilkan `source` tak dikenal secara generik. Entri lama tidak diubah.
- `deposit` hanya ada untuk `source = deposit`; `redemption` hanya untuk `source = redemption`.
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

## GET /api/v1/me/home

Beranda user dalam **satu** request. Tanpa query. Akun active; mitra aktif tetap boleh memanggil (tidak ada pengetatan otorisasi), tetapi `missions` bernilai `null` dan layar mitra memakai `GET /partner/home`. Profil belum dibuat → 404 `account_not_found`; suspended/deleted → 403 `account_forbidden`.

> Prasyarat: `unread_count` membutuhkan migration 00011; lihat Prasyarat database di [README](README.md). Tanpa tabel itu endpoint gagal, bukan mengembalikan `0`.

Seluruh kartu dibaca dalam satu snapshot database (satu transaksi), sehingga saldo, aktivitas, misi, dan `unread_count` konsisten satu sama lain. HTTP dan PostgreSQL lokal dengan pool satu koneksi sudah diverifikasi ([bukti](../test-results/home-package-2026-11-23.md)); Firebase/Supabase/Android nyata untuk paket ini belum diuji. Contoh (nilai ilustrasi; ID dipendekkan; `total_earned` 300 = dua klaim @150, `total_spent` 100 = satu penukaran):

```json
{"data":{
  "user_name":"Fahry",
  "server_time":"2026-11-23T03:00:00Z",
  "month":{"from":"2026-11-01T00:00:00+07:00","to":"2026-11-23T10:00:00+07:00","timezone":"Asia/Jakarta"},
  "wallet":{"balance":200,"total_earned":300,"total_spent":100,"updated_at":"2026-11-23T02:55:00Z"},
  "deposits_claimed":2,
  "total_weight_kg":"2.000",
  "monthly_deposits":1,
  "active_vouchers":1,
  "recent_activity":[
    {"id":"…","amount":150,"source":"deposit","created_at":"2026-11-23T02:55:00Z","deposit":{"id":"…","partner_id":"20000000-0000-4000-8000-000000000001","partner_name":"Bank Sampah Tunas","total_weight_kg":"1.200"}},
    {"id":"…","amount":-100,"source":"redemption","created_at":"2026-11-10T04:00:00Z","redemption":{"id":"…","reward_name":"Voucher Kopi"}},
    {"id":"…","amount":150,"source":"deposit","created_at":"2026-10-28T03:00:00Z","deposit":{"id":"…","partner_id":"20000000-0000-4000-8000-000000000001","partner_name":"Bank Sampah Tunas","total_weight_kg":"0.800"}}
  ],
  "missions":{
    "period_start":"2026-11-23T00:00:00+07:00","period_end":"2026-11-30T00:00:00+07:00","server_time":"2026-11-23T03:00:00Z",
    "weekly_xp":0,"total_xp":0,
    "missions":[
      {"id":"first_deposit","title":"…","description":"…","metric":"…","unit":"deposit","target":1,"progress":1,"reward_xp":50,"status":"claimable","claimed_at":null},
      {"id":"three_kilograms","title":"…","description":"…","metric":"…","unit":"gram","target":3000,"progress":1200,"reward_xp":100,"status":"in_progress","claimed_at":null}
    ]
  },
  "partner":{"id":"20000000-0000-4000-8000-000000000001","name":"Bank Sampah Tunas","area":"Kawasan Malang","address":"…","phone":"","opening_hours":"…","description":"…","location":{"latitude":-7.9775,"longitude":112.6304},"accepted_materials":[{"id":"…","name":"Plastik","slug":"plastic","code":"PLASTIC","handling_guide":"","requires_special_handling":false}],"hours":[{"day_of_week":0,"opens_at":"","closes_at":"","is_closed":true}]},
  "article":{"id":"…","title":"…","summary":"…","category":"NEWS","reading_minutes":3,"is_featured":false,"is_demo":true,"published_at":"2026-11-20T01:00:00Z"},
  "unread_count":2
}}
```

### Arti field

| Field | Arti | Catatan |
|---|---|---|
| `user_name` | `full_name` caller | |
| `server_time` | Instan dari database, dibaca sekali untuk semua periode di respons | Dasar hitung sisa waktu misi; bukan jam HP |
| `month` | Bulan kalender WIB berjalan: `from` = tanggal 1 pukul 00.00 WIB (inklusif), `to` = `server_time` (eksklusif), `timezone` = `Asia/Jakarta` | Parse sebagai instan; jangan bergantung pada offset teks |
| `wallet` | Saldo, total didapat, total terpakai | Sama dengan `GET /me/wallet` |
| `deposits_claimed` | Jumlah klaim setoran **all-time** | Bukan bulanan |
| `total_weight_kg` | Berat terklaim **all-time**, string desimal | |
| `monthly_deposits` | Jumlah klaim setoran dengan `claimed_at` dalam `month` | Dihitung per klaim (bukan per item, bukan `created_at`); setoran belum diklaim tidak masuk. Konvensi bulan sama dengan laporan aktivitas |
| `active_vouchers` | Voucher `issued` yang belum kedaluwarsa | |
| `recent_activity` | Maks. 3 entri riwayat poin, terbaru di atas; `[]` bila kosong | Format sama dengan `/me/point-history` |
| `missions` | Objek yang sama dengan `GET /me/missions` (katalog, `status`, `reward_xp`, `period_start` sebagai token klaim, `server_time`); `null` untuk mitra | `missions.missions` selalu array |
| `partner` | **Satu** mitra: lokasi verified pertama menurut urutan daftar mitra (nama, lalu ID); `null` jika tidak ada | Saran global, **bukan** mitra terdekat (tidak ada GPS/radius). Memakai bentuk detail mitra |
| `article` | Artikel terbit terbaru yang `published_at`-nya belum di masa depan; `null` jika tidak ada | Urutan sama dengan `/articles` |
| `unread_count` | Total notifikasi belum dibaca | Definisi sama dengan `/me/notifications`; preferensi `notification_enabled=false` tidak menghapus notifikasi lama |

Aturan: `partner` dan `article` bernilai `null` bila kosong (tampilkan state kosong, bukan data demo). `recent_activity` dan `missions.missions` tidak pernah `null` bila objeknya ada. Home tidak memuat token QR, kode voucher, atau data pribadi user lain. Tidak ada query parameter baru.

### Pemetaan `HomeUiState` Android

| `HomeUiState` / layar | Sumber wire | Catatan |
|---|---|---|
| `userName` | `user_name` | Default "Pengguna" hanya saat loading/gagal |
| `points` | `wallet.balance` | EcoPoints. Bukan XP |
| `monthlyDeposits` | `monthly_deposits` | Bukan `deposits_claimed` |
| `missionTitle`, `missionDescription` | `title`, `description` dari satu elemen `missions.missions[]` | Server tidak memilih prioritas; FE memilih satu (mis. `claimable`, lalu `in_progress`) |
| `missionProgress`, `missionTarget` | `progress`, `target` | Per `unit`: `first_deposit` = `deposit`, `three_kilograms` = **`gram`** (3000 = 3 kg). Jangan membandingkan dengan kg |
| `missionReward` | `reward_xp` | Label **XP**, bukan EcoPoints, dan tidak menambah saldo |
| aksi klaim misi | `missions.period_start` + `id` | Lihat [gamification.md](gamification.md); layar penuh tetap `GET /me/missions` |
| `nearbyPartner` (`NearbyPartnerUi`) | `partner.name`, `partner.area`, `partner.accepted_materials[].name` | Label "Mitra di sekitarmu" tidak akurat: ini saran global, gunakan label netral. Simpan `partner.id` untuk `Routes.partnerDetail(id)` → `GET /partners/{id}` |
| berita (`newsTitle/newsSummary/newsReadingMinutes`) | `article.title`, `article.summary`, `article.reading_minutes` | Simpan `article.id` untuk `Routes.articleDetail(id)` → `GET /articles/{id}`; hapus teks demo tetap |
| badge lonceng | `unread_count` | Membuka `Routes.NOTIFICATION` |
| aktivitas terbaru | `recent_activity` | Tidak ada field di `HomeUiState` sekarang; tambahkan jika ingin ditampilkan |

`NearbyPartnerUi` saat ini belum membawa ID; FE perlu menambahkannya agar klik kartu membuka detail yang benar. `HomeUiState.demo()` tidak boleh menjadi fallback jaringan: gunakan state loading/error.

Refresh: panggil ulang `/me/home` saat layar dibuka kembali, setelah klaim QR, penukaran reward, klaim misi, dan menandai notifikasi dibaca. Respons tidak bisa di-cache sebagai kebenaran saldo.

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
