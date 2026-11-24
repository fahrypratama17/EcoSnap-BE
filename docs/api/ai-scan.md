# Kontrak API AI Scan (layanan FastAPI mandiri)

Dokumen serah-terima untuk tim backend/FE. Kontrak mesin-terbaca: [`ai-scan.openapi.json`](ai-scan.openapi.json) (OpenAPI 3.1).

> **Status.** Kontrak diekspor dari kode sumber tim AI dan diverifikasi lokal terhadap snapshot commit di bawah: struktur OpenAPI 3.1 valid, 6 skema cocok dengan Pydantic sumber, 24 contoh respons valid, dan 18 skenario HTTP FastAPI lulus. Inferensi pada proof memakai fixture, bukan panggilan Groq; key, kuota, akurasi model, perangkat Android, Firebase dan Supabase tidak diuji. Bagian "Bukti runtime" upstream bukan verifikasi kami.

## Sumber (commit tetap)

- Repo/branch: <https://github.com/fahrypratama17/EcoSnap-FE/tree/13-feat/ai-waste-scan>
- Commit: `9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab`
- Berkas: [`backend/README.md`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/README.md), [`app/main.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/main.py), [`app/schema.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/schema.py), [`app/provider.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/provider.py), [`app/service.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/service.py), [`app/images.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/images.py), [`app/errors.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/errors.py), [`app/settings.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/settings.py), fixture Android [`waste_scan_success.json`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/app/src/test/resources/waste_scan_success.json).

Dokumentasi runtime/README upstream (setup, uji manual, daftar berkas) sudah ada di sumber di atas; dokumen ini hanya menambahkan kontrak HTTP yang diekspor (`docs/api/ai-scan.md` dan `ai-scan.openapi.json`).

## Layanan mandiri, bukan rute Go

| | Layanan AI | EcoSnap-BE (Go) |
|---|---|---|
| Port | 8000 | 8080 |
| Rute | `POST /api/scan`, `GET /health` (+ `/docs`, `/openapi.json` otomatis) | 46 rute (lihat [README.md](README.md)) |
| Auth | **Tidak ada** penegakan Firebase Bearer | Firebase Bearer |
| Respons | JSON mentah `ScanResponse` | pembungkus Go |

Tidak ada route/proxy/database/auth Go untuk scan. **Jangan** menyajikannya sebagai rute Go yang aman atau integrasi mitra nyata: peta mitra FE masih memakai `DummyPartners`. Respons tidak memuat `confidence` atau `scan_id`. Endpoint riwayat seperti `GET /api/v1/me/scans` belum tersedia; scan tidak mengubah wallet.

## Menjalankan lokal (repo FE, venv)

Perlu Python 3.11+. Dari root repo EcoSnap-FE (PowerShell). Jangan menimpa `.env` yang sudah ada; kunci cukup lewat variabel lingkungan sesi, di sisi server saja:

```powershell
python -m venv backend/.venv
backend/.venv/Scripts/python.exe -m pip install -r backend/requirements-dev.txt

$secure = Read-Host 'GROQ_API_KEY' -AsSecureString
$env:GROQ_API_KEY = [System.Net.NetworkCredential]::new('', $secure).Password
$env:GROQ_MODEL = 'qwen/qwen3.8-27b'   # default sebenarnya; opsional

backend/.venv/Scripts/python.exe -m uvicorn app.main:app --app-dir backend --host 127.0.0.1 --port 8000 --workers 1 --no-access-log --no-proxy-headers

# selesai:
Remove-Item Env:GROQ_API_KEY
```

- `GROQ_API_KEY` hanya server; jangan masuk FE, Gradle, BuildConfig, resource, repo, atau log. Jangan tempel kunci ke dokumen/chat.
- `GROQ_MODEL` default `qwen/qwen3.8-27b` (bukan `qwen3-32b`); `GROQ_BASE_URL` default `https://api.groq.com/openai/v1`.
- `load_dotenv` tidak menimpa variabel lingkungan yang sudah ada.
- Tanpa kunci: `503 missing_api_key`.
- Health: `http://127.0.0.1:8000/health` -> `{"status":"ok"}`.

Uji (memakai kuota/biaya Groq sungguhan; belum kami jalankan):

```powershell
curl.exe -X POST http://127.0.0.1:8000/api/scan -H 'X-Scan-Id: 6e759542-b5b7-4361-83c9-15e1bdb591fc' -F 'image=@C:/path/to/waste.jpg'
```

## Permintaan

`POST /api/scan`, `Content-Type: multipart/form-data`.

- Field `image`: **tepat satu** file, nama field persis `image`, tanpa field lain. JPEG/PNG/WebP statis; HEIC dan gambar beranimasi ditolak.
- Batas: 10 MiB file, 25 megapiksel. Server mengoreksi EXIF, memperkecil maks 1024 px, menaruh transparansi di latar putih, keluaran JPEG q83 tanpa metadata. Unggahan tidak disimpan.
- Header `X-Scan-Id` opsional (UUID). Jika tidak ada, server membuat UUID acak. Tidak valid -> `400 invalid_scan_id`.

## Respons sukses (HTTP 200)

Badan adalah `ScanResponse` **mentah** (bukan pembungkus Go), header `Cache-Control: no-store`. Semua field wajib; "kosong" berarti `null` atau `[]`. Semua objek `additionalProperties: false` dan mode strict. Skema lengkap: `components.schemas` di berkas OpenAPI (dibangkitkan dari `model_json_schema()` Pydantic v2 pada `schema.py`; referensi `$defs` dipindah ke `components/schemas`).

### Status domain vs error HTTP

`status` ∈ `success | invalid_image | multiple_objects`. Dua yang terakhir adalah **non-identifikasi domain** dan tetap **HTTP 200** (minta pengguna foto ulang). Kegagalan operasional memakai status HTTP error dan badan `{"error":{"code":"...","message":"..."}}` (di bawah).

### Field

`ScanResponse` (semua wajib):

| Field | Tipe | Null? |
|---|---|---|
| `status` | enum di atas | tidak |
| `message` | string 1-900 | ya (wajib terisi bila status bukan success) |
| `identification` | `Identification` | ya |
| `handling_steps` | string[] (maks 6, tiap 1-900) | tidak (boleh `[]`) |
| `upcycling` | `Upcycling` | tidak |
| `industrial_recycling` | `IndustrialRecycling` | ya |
| `deposit_recommendation` | `DepositRecommendation` | ya |

`Identification` (semua wajib): `object_name` string 1-160 atau null; `material` string 1-160 atau null; `category` enum `Organik|Anorganik|B3|Residu|Tidak Diketahui` atau null; `recyclable` bool atau null; `recycling_status` enum `recyclable|conditionally_recyclable|non_recyclable|unknown` (tidak null); `tags` string[] (maks 5, tiap 1-160); `description` string 1-900 atau null; `requires_verification` bool; `verification_note` string 1-900 atau null; `hazardous` bool.

`Upcycling`: `possible` bool; `reason` string 1-900 atau null; `ideas` `Idea[]` maks 3.

`Idea` (semua wajib, tidak null): `title` 1-160; `description` 1-900; `difficulty` enum `Mudah|Sedang|Sulit`; `tools_and_materials` string[] 1-12 (tiap 1-160); `steps` string[] 3-6 (tiap 1-900); `safety_warning` 1-900.

`IndustrialRecycling`: `possible` bool atau null; `description` string 1-900 atau null; `potential_products` string[] maks 6 (tiap 1-160); `limitations` string[] maks 6 (tiap 1-900).

`DepositRecommendation`: `recommended` bool; `partner_type` 1-160 atau null; `search_keyword` 1-160 atau null; `preparation_note` 1-900 atau null.

String di-strip sebelum batas dihitung.

### Invarian semantik (tidak terwakili JSON Schema)

- `recyclable`: true untuk `recyclable`/`conditionally_recyclable`, false untuk `non_recyclable`, null untuk `unknown`. `unknown` mensyaratkan `requires_verification=true`. `requires_verification=true` mensyaratkan `verification_note`. `B3` mensyaratkan `hazardous=true`.
- `upcycling.possible == (ideas tidak kosong)`; `reason` wajib bila `possible=false`; judul ide unik (tanpa beda huruf besar/kecil).
- `industrial_recycling.possible` bukan true -> `potential_products` kosong.
- `deposit_recommendation.recommended=true` -> `partner_type` dan `search_keyword` terisi.
- Status bukan `success`: `message` terisi, `identification`/`industrial_recycling`/`deposit_recommendation` null, `handling_steps` `[]`, `upcycling.possible=false`, `ideas` `[]`.
- `success`: identification dengan `object_name`, `category`, `description`; `handling_steps` tidak kosong; `industrial_recycling` (dengan `description`) dan `deposit_recommendation` tidak null.
- Limbah `hazardous` tidak boleh punya ide DIY. Kata kunci berbahaya (`baterai`, `battery`, `aki `, `jarum suntik`, `limbah medis`, `pestisida`, `limbah elektronik`, `komponen elektronik`) pada nama+material mewajibkan `hazardous=true` dan tanpa ide.
- Material belum terverifikasi (`requires_verification` atau `material=null`): ide tak boleh memuat `wadah makanan`, `air minum`, `gelas minum`, `botol minum`.
- `non_recyclable` melarang `industrial_recycling.possible=true`; `conditionally_recyclable` mensyaratkan `limitations` tidak kosong.

Validasi ini tidak membuktikan kebenaran ilmiah saran AI.

### Contoh sukses (fixture Android `waste_scan_success.json`)

```json
{
  "status": "success",
  "message": null,
  "identification": {
    "object_name": "Botol Plastik Minuman",
    "material": "Plastik",
    "category": "Anorganik",
    "recyclable": true,
    "recycling_status": "conditionally_recyclable",
    "tags": [
      "Plastik",
      "Anorganik",
      "Potensi Daur Ulang"
    ],
    "description": "Botol plastik bekas yang memerlukan pemeriksaan material.",
    "requires_verification": true,
    "verification_note": "Periksa kode material pada kemasan dan ketentuan fasilitas.",
    "hazardous": false
  },
  "handling_steps": [
    "Kosongkan sisa isi botol.",
    "Bilas bila diperlukan dan aman, lalu keringkan.",
    "Pisahkan sesuai ketentuan mitra."
  ],
  "upcycling": {
    "possible": true,
    "reason": null,
    "ideas": [
      {
        "title": "Pot Tanaman Mini",
        "description": "Ubah botol bekas menjadi wadah tanaman.",
        "difficulty": "Mudah",
        "tools_and_materials": [
          "Botol bekas",
          "Tanah",
          "Bibit",
          "Gunting"
        ],
        "steps": [
          "Bersihkan dan keringkan botol.",
          "Potong botol dan rapikan tepinya.",
          "Buat lubang drainase dan isi tanah.",
          "Tanam bibit."
        ],
        "safety_warning": "Gunakan alat tajam dengan hati-hati dan dengan pendampingan orang dewasa untuk anak-anak."
      }
    ]
  },
  "industrial_recycling": {
    "possible": true,
    "description": "Plastik tertentu dapat dipilah dan diolah menjadi bahan baku baru.",
    "potential_products": [
      "Pelet plastik"
    ],
    "limitations": [
      "Bergantung pada material dan fasilitas yang tersedia."
    ]
  },
  "deposit_recommendation": {
    "recommended": true,
    "partner_type": "Bank Sampah",
    "search_keyword": "Bank sampah botol plastik",
    "preparation_note": "Pastikan botol kosong dan bersih."
  }
}
```

### Contoh `invalid_image` (HTTP 200, valid secara semantik)

```json
{
  "status": "invalid_image",
  "message": "Foto tidak jelas. Ambil ulang foto satu objek sampah dengan pencahayaan cukup.",
  "identification": null,
  "handling_steps": [],
  "upcycling": {
    "possible": false,
    "reason": "Objek tidak dapat dikenali sehingga tidak ada panduan yang aman.",
    "ideas": []
  },
  "industrial_recycling": null,
  "deposit_recommendation": null
}
```

(`multiple_objects` identik dengan `status` dan `message` berbeda; contoh ada di OpenAPI.)

### Contoh error operasional (HTTP 429, `Retry-After: 60`, `Cache-Control: no-store`)

```json
{"error": {"code": "rate_limit", "message": "Terlalu banyak pemindaian. Tunggu satu menit lalu coba lagi."}}
```

## Peta error

Badan: `{"error":{"code":"...","message":"..."}}`; semua error membawa `Cache-Control: no-store`. `Retry-After` (detik) hanya bila disebut. Pesan provider mentah tidak pernah dikembalikan.

| HTTP | `code` | Penyebab | Retry-After |
|---|---|---|---|
| 400 | `invalid_upload` | Content-Length tidak valid; multipart rusak/melebihi 1 file atau ada field lain; rute/metode tak dikenal (mis. `GET /api/scan`) dan error validasi FastAPI juga dipetakan ke sini | - |
| 400 | `invalid_scan_id` | `X-Scan-Id` bukan UUID | - |
| 400 | `missing_image` | field `image` tidak ada/bukan file, atau jumlah field != 1 | - |
| 408 | `upload_timeout` | badan tidak selesai dalam 20 dtk | - |
| 409 | `scan_id_conflict` | ID sama (IP klien sama) dengan byte gambar berbeda | - |
| 413 | `image_too_large` | > 10 MiB (badan > 10 MiB + 64 KiB) atau > 25 MP / decompression bomb | - |
| 415 | `unsupported_upload` | bukan `multipart/form-data` | - |
| 415 | `unsupported_image` | kosong, rusak, bukan JPEG/PNG/WebP, atau beranimasi | - |
| 429 | `rate_limit` | 6/IP/menit atau 30 global/menit terlampaui (atau > 4096 IP terlacak) | 60 |
| 429 | `provider_rate_limit` | Groq 429 | 1-3600 (dari provider) |
| 500 | `internal_error` | kesalahan tak terduga | - |
| 502 | `provider_error` | Groq gagal (selain kasus 4xx/429 yang dipetakan lain) | - |
| 502 | `truncated_response` | `finish_reason=length` | - |
| 502 | `invalid_ai_response` | JSON/skema/semantik tidak valid, respons > 256000 byte, atau tidak selesai | - |
| 503 | `busy` | 4 unggahan sudah aktif, atau cache 64 entri penuh dengan tugas belum selesai | 5 |
| 503 | `missing_api_key` | `GROQ_API_KEY` kosong | - |
| 503 | `invalid_api_key` | Groq 401/403 | - |
| 503 | `model_unavailable` | Groq 400/404/422 | - |
| 503 | `provider_network` | gagal koneksi/jaringan ke Groq | - |
| 504 | `provider_timeout` | batas pemrosesan 85 dtk atau baca provider 65 dtk habis | - |

Retry otomatis server hanya satu kali: untuk connect error (jeda 1 dtk) dan provider 429/502/503/504 dengan Retry-After ≤ 5 dtk. Read timeout, JSON rusak, pelanggaran skema, dan output terpotong tidak di-retry.

## Idempotensi dan batas laju (hanya RAM, proses tunggal)

- Kunci: (IP klien, `X-Scan-Id`) + hash SHA-256 byte gambar. Permintaan bersamaan dengan kunci sama berbagi satu inferensi.
- Hasil yang selesai dengan sukses disimpan hingga 600 detik **sejak entri dibuat**; maksimal 64 entri. Saat penuh, entri selesai tertua dibuang lebih awal; jika semuanya masih berjalan -> `503 busy`.
- Tugas yang gagal dengan error layanan dihapus (tidak disimpan); hanya hasil sukses yang bertahan. Mencoba ulang ID yang sama setelah gagal memicu inferensi baru.
- **Tidak tahan restart dan tidak dibagi antar proses/replika.** IP klien dibaca dari koneksi (`--no-proxy-headers`); di balik reverse proxy semua klien terlihat sebagai IP proxy.
- Hitungan laju dihitung untuk setiap permintaan yang diterima, termasuk duplikat/retry dengan UUID sama dan yang berujung error; permintaan yang ditolak 429/503 oleh middleware tidak dihitung. Eviction cache tidak mengurangi hitungan laju.
- Batas: 6/IP/menit, 30 global/menit, 4 unggahan diterima serentak, 2 pemrosesan serentak (worker tunggal).

## Timeout

| Fase | Batas |
|---|---|
| Unggah badan | 20 dtk (`408`) |
| Pemrosesan (antrean slot + decode + provider + retry) | 85 dtk (`504`), dimulai saat tugas dibuat |
| Groq connect / read | 10 dtk / 65 dtk (fase terpisah) |
| Android call timeout | 100 dtk |

Batas-batas ini terpisah; **tidak ada jaminan total end-to-end 85 dtk** (waktu unggah ditambahkan di luar 85 dtk). Klien harus menangani timeout sendiri dan dapat mengulang dengan `X-Scan-Id` yang sama dan file yang sama.

## Persyaratan deployment (belum diimplementasikan)

- TLS/HTTPS dan reverse proxy dengan batas ukuran body & koneksi.
- Autentikasi (mis. verifikasi Firebase Bearer) dan kuota per pengguna; saat ini hanya batas IP/global. Login upstream hanya demo lokal.
- Rate limiter dan cache idempotensi terdistribusi/bersama **sebelum** memakai lebih dari satu worker atau replika.
- Panggilan provider dapat memakai kuota/biaya Groq; kami belum melakukan panggilan live.

## Bukti runtime upstream

README upstream menyatakan satu uji live pada 2026-11-24 dan tes Python/Android. Itu laporan tim AI dan **bukan verifikasi kami**.

## Mengekspor dan melihat dokumentasi

- Spesifikasi: [`docs/api/ai-scan.openapi.json`](ai-scan.openapi.json). Unduh dan impor ke Swagger Editor (File -> Import file) atau Redoc yang mendukung OpenAPI 3.1.
- Hanya `GET /health` dan `POST /api/scan` yang didokumentasikan; tidak ada skema keamanan karena layanan tidak menegakkan Bearer.
- Spesifikasi otomatis FastAPI (`/openapi.json`) menghilangkan requestBody, header `X-Scan-Id`, dan error karena runtime membaca `request.form` secara manual; berkas ekspor ini sengaja mendokumentasikannya dari implementasi.
- Jangan pernah mengekspos `GROQ_API_KEY` ke FE atau menaruhnya di spesifikasi/contoh.
