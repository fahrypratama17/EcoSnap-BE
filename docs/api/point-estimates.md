# Kontrak estimasi poin multi-material

Status: implementasi backend pada feat/deposit-estimation. Pengujian dan batas verifikasi dicatat pada [laporan batch](../test-results/point-estimates-2026-11-20.md). Tarif awal adalah data development editable. Pemilik telah memilih pembulatan ke bawah setelah seluruh hasil item dijumlahkan (D-17).

## Tujuan dan batas

Estimasi menerima beberapa kategori sampah beserta beratnya, memilih tarif aktif dari server, dan menghasilkan perkiraan poin. Operasi tidak membuat setoran, QR, ledger, atau mutasi wallet. Saat transaksi nyata dibuat, server tetap menghitung ulang tarif; hasil estimasi bukan kewenangan untuk mengklaim saldo.

Endpoint: POST /api/v1/point-estimates, Authorization Bearer Firebase dan Content-Type application/json. Caller tidak mengirim UID, tarif, total poin, atau status transaksi. Seluruh query parameter, field asing, dan JSON tambahan ditolak; batas body 16 KiB dan timeout query 5 detik. Token valid cukup untuk membaca estimasi, tanpa syarat role mitra atau izin membuat QR.

```json
{
  "partner_id":"20000000-0000-4000-8000-000000000001",
  "items":[
    {"category_id":"10000000-0000-4000-8000-000000000001","weight_kg":"2.500"},
    {"category_id":"10000000-0000-4000-8000-000000000002","weight_kg":"1.000"}
  ]
}
```

partner_id opsional: tanpa lokasi, gunakan tarif global kategori. Jika lokasi dipilih, server memastikan lokasi visible/verified dengan owner mitra active dan menerima seluruh kategori; tarif khusus lokasi didahulukan, kemudian fallback global.

## Aturan hitung sementara

- Berat dikirim sebagai string desimal bertitik, maksimal tiga angka desimal; minimum 0.001 kg.
- Maksimal 50 kategori unik, total berat maksimal 1000.000 kg. Ini batas teknis demo, bukan hasil pengukuran bisnis yang sudah final.
- Kategori ganda ditolak agar client menggabungkan berat kategori yang sama secara eksplisit.
- Tarif memakai dua angka desimal, lebih dari nol hingga 1000000.00 poin/kg.
- Server mengubah berat ke gram dan tarif ke perseratus poin/kg. Hasil kali dijumlahkan sebagai integer sebelum pembulatan total ke bawah sekali.
- Contoh 0.006 kg pada 100.00 poin/kg = 0.60000 poin. Dua kategori dengan hasil itu menghasilkan total 1 poin, bukan 0.
- UI boleh menampilkan koma; DTO Android harus mengubahnya menjadi format bertitik. Bentuk JSON number, exponent, NaN/Infinity, negatif, atau presisi lebih tinggi ditolak.

Item response membawa category_id, category_name, weight_kg canonical, rate_id, points_per_kg canonical, exact_points (lima desimal, belum dibulatkan), dan is_demo. Total membawa total_weight_kg, estimated_points integer, uses_demo_rates, serta as_of dari waktu database. Tidak ada poin item bulat yang harus dijumlahkan agar cocok dengan total. Semua nilai desimal pada wire berupa string; estimated_points berupa JSON integer.

Contoh response dengan seed global (tanpa partner_id):

```json
{
  "data": {
    "as_of": "2026-11-20T14:00:00Z",
    "total_weight_kg": "3.500",
    "estimated_points": 330,
    "uses_demo_rates": true,
    "items": [
      {"category_id":"10000000-0000-4000-8000-000000000001","category_name":"Plastik","weight_kg":"2.500","rate_id":"50000000-0000-4000-8000-000000000001","points_per_kg":"100.00","exact_points":"250.00000","is_demo":true},
      {"category_id":"10000000-0000-4000-8000-000000000002","category_name":"Kertas","weight_kg":"1.000","rate_id":"50000000-0000-4000-8000-000000000002","points_per_kg":"80.00","exact_points":"80.00000","is_demo":true}
    ]
  }
}
```

## Tarif aktif dan data demo

Migration 00005 menghubungkan point_rates ke kategori dan partner opsional, dengan active_from/active_until dan penanda is_demo. Pemilihan memakai waktu database dalam transaksi read-only repeatable-read, sehingga visibility lokasi, penerimaan kategori, dan tarif dibaca pada snapshot yang sama. Batas awal inklusif dan akhir eksklusif. Tarif lokasi aktif didahulukan atas global; di dalam scope yang sama pilih active_from terbaru, lalu ID untuk urutan deterministik. Overlap periode diperbolehkan mengikuti aturan prioritas tersebut; constraint menolak dua versi kategori/scope dengan active_from yang sama. Tabel memakai RLS dan akses langsung role anon/authenticated dicabut.

Data contoh awal: plastic 100.00, paper 80.00, metal 150.00, glass 60.00 poin/kg. Seed diberi flag demo dan tidak dijalankan ke Supabase otomatis. File sumber: data/seeders/dev/point_rates.json. Command hanya memasukkan tarif global, dengan kategori yang sudah tersedia; seed mitra sebelumnya menyediakan empat kategori tersebut.

Setelah migration 00001-00005 diterapkan pada database development yang dituju dan DATABASE_URL diisi pada environment proses:

```powershell
go run ./cmd/seed-point-rates --demo
# Atau gunakan file versi tarif buatan tim:
go run ./cmd/seed-point-rates --demo --file data/seeders/dev/point_rates.json
```

File JSON berisi 1-50 versi, maksimal 64 KiB. Edit nilai sebelum seed pertama jika diperlukan. Jika ID sudah tersimpan, command memverifikasi isi sama dan tidak menimpa data; perubahan ditolak dan seluruh batch di-rollback. Untuk tarif baru, tambahkan record dengan UUID baru dan active_from baru. Masa aktif belum dimulai tidak dipakai. Seed ini tidak membuat akun Firebase, lokasi, transaksi, atau saldo. Nilai contoh tidak menjanjikan nilai tukar uang nyata.

## Error

Format mengikuti envelope bersama: `{"error":{"code":"point_rate_unavailable","message":"tarif aktif belum tersedia"}}`.

| HTTP | code | Kondisi |
|---|---|---|
| 400 | invalid_request | Struktur/tipe JSON salah atau field asing. |
| 400 | invalid_estimate | Berat, jumlah item, ID, duplikasi, atau query parameter invalid. |
| 400 | waste_not_accepted | Mitra tidak menerima kategori yang diminta. |
| 401 | unauthorized | Firebase bearer tidak valid atau tidak tersedia. |
| 404 | partner_not_found | Lokasi tidak ditemukan atau tidak visible. |
| 404 | waste_category_not_found | Ada kategori tidak ditemukan. |
| 409 | point_rate_unavailable | Ada kategori tanpa tarif aktif yang dapat dipakai. Tidak ada total parsial. |
| 413 | body_too_large | Pembacaan JSON melewati batas body. |
| 415 | unsupported_media_type | Content-Type selain application/json. |
| 504 | request_timeout | Deadline query terlampaui. |
| 500 | internal_error | Error internal; detail tidak dikirim ke client. |

## Integrasi Android

Folder features/deposit pada worktree EcoSnap-FE yang diperiksa masih berisi .gitkeep. Layar kalkulator pada arsip desain tetap menjadi acuan; source layar tersebut belum tersedia untuk disambungkan. Endpoint menyediakan total berat dan total poin untuk kartu ringkasan yang sama. Karena keputusan produk mengizinkan banyak material, integrasi perlu mengirim pasangan category_id/weight_kg per material, bukan menebak jenis sampah dari angka total.

Gunakan GET /api/v1/waste-categories untuk ID kategori dan detail mitra untuk daftar material diterima. Ubah koma tampilan berat menjadi titik pada DTO; hindari pembulatan melalui Double. Labeli uses_demo_rates pada tampilan simulasi dan ambil hasil terbaru saat item berubah. Desain/frontend belum diubah pada batch ini. Pembuatan setoran/QR kelak harus memakai ulang aturan hitung server dan menyimpan snapshot tarif; endpoint estimasi tidak mengunci harga untuk transaksi berikutnya.

## Cakupan verifikasi

Tes mencakup floor total, pecahan kecil, presisi/batas, input invalid/duplicate, rate unavailable, kategori/lokasi tidak ditemukan, dan auth. PostgreSQL nyata memeriksa tarif global/lokasi, prioritas versi terbaru, window expired/future, penolakan material, serta wallet tetap. Seed CLI diperiksa lewat eksekusi dua kali dan penolakan perubahan ID yang sama. Hasil aktual tersedia pada laporan batch; keberadaan tes saja tidak berarti PASS. Integrasi Supabase, token Firebase nyata, dan runtime Android tetap pemeriksaan terpisah.
