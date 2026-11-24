# Kontrak API AI Scan (layanan FastAPI mandiri)

Dokumen serah-terima untuk tim backend/FE. Kontrak mesin-terbaca: [`ai-scan.openapi.json`](ai-scan.openapi.json) (OpenAPI 3.1).

> **Status runtime BE:** service tersedia di `ai-service/`; `POST /api/scan` kini **wajib** `Authorization: Bearer <Firebase ID token>` (lihat "Autentikasi"). Jumlah tes dan bukti runtime terkini untuk perubahan auth ini **menunggu laporan parent/QA** dan belum dinyatakan di sini. Angka historis sebelum auth (52 tes Python lulus dengan provider sintetis; upload Swagger terisolasi yang berakhir `503 missing_api_key`) hanya riwayat transfer, bukan bukti auth. Tidak ada bukti key/kuota/akurasi model, Firebase/Supabase nyata, atau perangkat Android nyata di dokumen ini.

## Sumber (asal historis dan referensi terbaru)

Implementasi sekarang berada di `ai-service/` repo EcoSnap-BE, dipindahkan dari source tim AI pada SHA upstream terbaru di bawah. Go dan Python tetap proses terpisah; source FE lokal tidak diubah karena sedang berisi pekerjaan integrasi lain dan tidak mempunyai direktori `backend/`.

- Repo/branch historis: <https://github.com/fahrypratama17/EcoSnap-FE/tree/13-feat/ai-waste-scan>
- Commit historis: `9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab`
- Source transfer: [`4310cd7dac442b71130f375ab61a4025d298686c/backend`](https://github.com/fahrypratama17/EcoSnap-FE/tree/4310cd7dac442b71130f375ab61a4025d298686c/backend). HTTP/schema Python tidak berubah dari snapshot historis; penyesuaian BE mencakup `.env` root, lokasi fixture tes dan publikasi kontrak Swagger lengkap.
- Berkas historis (pada commit lama): [`backend/README.md`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/README.md), [`app/main.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/main.py), [`app/schema.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/schema.py), [`app/provider.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/provider.py), [`app/service.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/service.py), [`app/images.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/images.py), [`app/errors.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/errors.py), [`app/settings.py`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/backend/app/settings.py), fixture Android [`waste_scan_success.json`](https://github.com/fahrypratama17/EcoSnap-FE/blob/9dad3cb5ff06f4c009c2a48af25b70c3f4a1f8ab/app/src/test/resources/waste_scan_success.json).

Setup/runtime saat ini ada di bagian "Menjalankan lokal" di bawah; dokumen ini menambahkan kontrak HTTP yang diekspor (`docs/api/ai-scan.md` dan `ai-scan.openapi.json`).

## Layanan mandiri, bukan rute Go

| | Layanan AI | EcoSnap-BE (Go) |
|---|---|---|
| Port | 8000 | 8080 |
| Rute | `POST /api/scan`, `GET /health` (+ `/docs`, `/openapi.json` otomatis) | 46 rute (lihat [README.md](README.md)) |
| Auth | `POST /api/scan` **wajib** Firebase ID token (Bearer); `GET /health`, `/docs`, `/openapi.json` publik | Firebase Bearer |
| Respons | JSON mentah `ScanResponse` | pembungkus Go |

Tidak ada route/proxy/database Go untuk scan: Go tidak mem-proxy scan dan runtime Python tetap terpisah. **Jangan** menyajikannya sebagai rute Go atau integrasi mitra nyata: peta mitra FE masih memakai `DummyPartners`. Respons tidak memuat `confidence` atau `scan_id`. Endpoint riwayat seperti `GET /api/v1/me/scans` belum tersedia; scan tidak mengubah wallet.

## Menjalankan lokal (EcoSnap-BE, `ai-service/`)

Perlu Python 3.11+. Dari **root EcoSnap-BE** (PowerShell). Layanan berdiri sendiri di `ai-service/` (`app/`, `tests/`, `requirements.txt` ber-pin) dan tidak bergantung pada kode Go. Ia memverifikasi Firebase ID token dengan service account yang sama dengan Go, tetapi tidak punya riwayat atau wallet.

```powershell
python -m venv ai-service/.venv
ai-service/.venv/Scripts/python.exe -m pip install -r ai-service/requirements.txt

ai-service/.venv/Scripts/python.exe -m uvicorn app.main:app --app-dir ai-service --host 127.0.0.1 --port 8000 --workers 1 --no-access-log --no-proxy-headers
```

Untuk menjalankan tes offline: pasang `ai-service/requirements-dev.txt` ke venv yang sama, lalu dari folder `ai-service` jalankan `.venv/Scripts/python.exe -m pytest -q`. Job CI `ai-service` menjalankan suite ini tanpa key/credential asli. Hasil terkini setelah auth: menunggu laporan parent (historis pra-auth: 52 PASS). Peringatan dependency Starlette tentang migrasi TestClient ke `httpx2` pernah muncul; dependency upstream tidak diubah untuk menghilangkan peringatan.

### Konfigurasi `.env` (root BE, tanpa env sesi PowerShell)

Pengaturan AI dimuat dari `.env` di **root EcoSnap-BE** (UTF-8, BOM `utf-8-sig` diterima, `override=False`); mekanisme ini tidak berubah. Tidak perlu `$env:GROQ_API_KEY` di sesi PowerShell. **Tambahkan** (append) baris berikut ke `.env` yang sudah ada; jangan menimpa file atau menghapus konfigurasi Go/Goose (`HTTP_ADDR`, `DATABASE_URL`, `GOOGLE_APPLICATION_CREDENTIALS`, `GOOSE_*`). Auth AI **membutuhkan** `GOOGLE_APPLICATION_CREDENTIALS` berisi path JSON service account Firebase **proyek yang sama** dengan Go (di luar repo); nilai ini terpisah dari kunci Groq dan tidak ditaruh di dokumen/log:

```dotenv
GROQ_API_KEY=ISI_KUNCI_BARU_ANDA_DI_SINI
GROQ_MODEL=qwen/qwen3.8-27b
GROQ_BASE_URL=https://api.groq.com/openai/v1
```

`GROQ_MODEL` dan `GROQ_BASE_URL` opsional (nilai di atas adalah default). Ganti placeholder dengan kunci baru Anda sendiri; jangan commit `.env` dan jangan tempel kunci ke dokumen, chat, atau log.

- `override=False`: variabel lingkungan OS/proses yang **sudah ada menang** atas `.env` (termasuk nilai kosong). Jika Anda pernah mengatur `GROQ_API_KEY` di OS atau terminal, buka terminal baru atau restart proses/IDE agar `.env` dipakai. Restart uvicorn setiap `.env` berubah; tidak ada hot reload.
- `GROQ_API_KEY` hanya server; jangan masuk FE, Gradle, BuildConfig, resource, repo, atau log. Go (`cmd/app`) tidak memerlukannya.
- Kunci Groq saja **tidak cukup** untuk scan nyata: `POST /api/scan` juga butuh Firebase ID token pengguna valid dan `GOOGLE_APPLICATION_CREDENTIALS` yang terbaca layanan AI. Tanpa kunci Groq (token valid): `503 missing_api_key`. Tanpa konfigurasi/sertifikat Firebase yang bisa dipakai: `503 auth_unavailable`.

### Autentikasi

- `POST /api/scan` wajib `Authorization: Bearer <Firebase ID token>`. `GET /health`, `/docs`, dan `/openapi.json` tetap publik. Tidak ada flag untuk menonaktifkan atau membuat auth opsional.
- Token diverifikasi **sebelum** multipart dibaca dan sebelum inferensi/Groq. Token hilang/tidak valid/kedaluwarsa -> `401 unauthorized` dengan header `WWW-Authenticate: Bearer`.
- Konfigurasi/sertifikat/jaringan Firebase tidak tersedia atau verifikasi melewati timeout 10 dtk -> `503 auth_unavailable` dengan `Retry-After: 5`.
- Tidak ada cek pencabutan token per scan; validitas akun Firebase yang menentukan, bukan peran/status di database Go (layanan AI tidak membaca database Go).
- UID dari token adalah identitas untuk batas laju dan cache idempotensi (bukan IP).
- Android: ambil token SDK sebelum setiap request (`getIdToken(false)`); pada 401, satu kali `forceRefresh` lalu ulangi dengan file dan `X-Scan-Id` yang sama, dengan penjaga sesi. UI tidak meminta API key apa pun.

### Arti API key dan perilaku inferensi

`GROQ_API_KEY` adalah **kredensial penyedia** (akses akun/kuota Groq), **bukan** model terlatih atau artefak model milik proyek. Tidak ada bobot model di repo. Implementasi yang ada mengirim **gambar sebenarnya** (telah diperkecil, JPEG tanpa metadata) ke model vision `qwen/qwen3.8-27b` lewat Groq dan memvalidasi JSON hasilnya; kualitas dan akurasi jawaban tidak dibuktikan dokumen ini.

### Health dan uji manual

- `http://127.0.0.1:8000/health` -> `{"status":"ok"}` hanya membuktikan **proses hidup**. Itu tidak membuktikan kunci ada, valid, berkuota, atau model tersedia; jangan menyimpulkannya dari `/health`.
- Uji `POST /api/scan` memakai kuota/biaya Groq sungguhan **dan** token Firebase pengguna. **Jalankan hanya bila Anda sendiri memutuskan** (belum kami jalankan di dokumen ini). Ganti placeholder dengan ID token Firebase pengguna sendiri; jangan tempel token asli ke dokumen/chat/log. Di Swagger `/docs`, tombol **Authorize** diisi token pengguna Firebase tersebut, **bukan** kunci Groq:

```powershell
curl.exe -X POST http://127.0.0.1:8000/api/scan -H 'Authorization: Bearer ISI_FIREBASE_ID_TOKEN_ANDA' -H 'X-Scan-Id: 6e759542-b5b7-4361-83c9-15e1bdb591fc' -F 'image=@C:/path/to/waste.jpg'
```

- Kunci yang pernah terekspos (di chat, repo, log, atau tangkapan layar) harus dianggap bocor: **cabut/revoke di konsol Groq dan buat kunci baru**; jangan memakainya untuk permintaan apa pun, termasuk uji ini.

### Alamat per klien dan batas layanan

- Go tetap di `:8080` (Emulator Android: `http://10.0.2.2:8080`); layanan AI di `:8000` (Emulator: `http://10.0.2.2:8000`). Keduanya tetap hanya backend localhost; tidak ada proxy Go untuk scan.
- Demo lab: Nginx di port 80 menyajikan **satu origin** `http://10.34.211.188/` untuk `API_BASE_URL` dan `AI_SCAN_BASE_URL` (override debug saja); Nginx meneruskan ke Go `:8080` dan AI `:8000` yang tetap lokal, sehingga klaim "tidak ada port tunggal bersama" tidak lagi berlaku untuk demo lab. Detail: [lab-vps-deployment.md](../lab-vps-deployment.md). Runtime Go dan AI tetap terpisah (bukan route Go).
- HTTP sementara ini **risiko yang diterima pengguna**: token dan foto tidak terenkripsi. Hanya untuk lab; jangan dipakai publik/data sensitif; HTTPS wajib kemudian.
- Jangan membuka layanan AI langsung ke publik (jangan bind `0.0.0.0` atau tunnel); akses lab lewat Nginx. Lihat "Persyaratan deployment".

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

Badan: `{"error":{"code":"...","message":"..."}}`; semua error membawa `Cache-Control: no-store`. `Retry-After` (detik) hanya bila disebut. Pesan provider mentah tidak pernah dikembalikan. Auth dicek lebih dulu: 401/503 `auth_unavailable` muncul sebelum validasi multipart.

| HTTP | `code` | Penyebab | Retry-After |
|---|---|---|---|
| 400 | `invalid_upload` | Content-Length tidak valid; multipart rusak/melebihi 1 file atau ada field lain; rute/metode tak dikenal (mis. `GET /api/scan`) dan error validasi FastAPI juga dipetakan ke sini | - |
| 400 | `invalid_scan_id` | `X-Scan-Id` bukan UUID | - |
| 400 | `missing_image` | field `image` tidak ada/bukan file, atau jumlah field != 1 | - |
| 408 | `upload_timeout` | badan tidak selesai dalam 20 dtk | - |
| 409 | `scan_id_conflict` | UID sama dengan ID sama tetapi byte gambar berbeda | - |
| 413 | `image_too_large` | > 10 MiB (badan > 10 MiB + 64 KiB) atau > 25 MP / decompression bomb | - |
| 415 | `unsupported_upload` | bukan `multipart/form-data` | - |
| 415 | `unsupported_image` | kosong, rusak, bukan JPEG/PNG/WebP, atau beranimasi | - |
| 401 | `unauthorized` | Bearer hilang/tidak valid/kedaluwarsa (header `WWW-Authenticate: Bearer`) | - |
| 429 | `rate_limit` | 6/UID/menit atau 30 global/menit terlampaui (atau terlalu banyak UID terlacak) | 60 |
| 429 | `provider_rate_limit` | Groq 429 | 1-3600 (dari provider) |
| 500 | `internal_error` | kesalahan tak terduga | - |
| 502 | `provider_error` | Groq gagal (selain kasus 4xx/429 yang dipetakan lain) | - |
| 502 | `truncated_response` | `finish_reason=length` | - |
| 502 | `invalid_ai_response` | JSON/skema/semantik tidak valid, respons > 256000 byte, atau tidak selesai | - |
| 503 | `busy` | 4 unggahan sudah aktif, atau cache 64 entri penuh dengan tugas belum selesai | 5 |
| 503 | `auth_unavailable` | konfigurasi/sertifikat/jaringan Firebase tidak tersedia, atau verifikasi melewati 10 dtk | 5 |
| 503 | `missing_api_key` | `GROQ_API_KEY` kosong (setelah auth lolos) | - |
| 503 | `invalid_api_key` | Groq 401/403 | - |
| 503 | `model_unavailable` | Groq 400/404/422 | - |
| 503 | `provider_network` | gagal koneksi/jaringan ke Groq | - |
| 504 | `provider_timeout` | batas pemrosesan 85 dtk atau baca provider 65 dtk habis | - |

Retry otomatis server hanya satu kali: untuk connect error (jeda 1 dtk) dan provider 429/502/503/504 dengan Retry-After ≤ 5 dtk. Read timeout, JSON rusak, pelanggaran skema, dan output terpotong tidak di-retry.

## Idempotensi dan batas laju (hanya RAM, proses tunggal)

- Kunci: (UID Firebase, `X-Scan-Id`) + hash SHA-256 byte gambar, bukan IP. UID sama + UUID sama + byte sama berbagi cache/inferensi lintas IP; UID berbeda terisolasi walau UUID dan byte sama. Permintaan bersamaan dengan kunci sama berbagi satu inferensi.
- Hasil yang selesai dengan sukses disimpan hingga 600 detik **sejak entri dibuat**; maksimal 64 entri. Saat penuh, entri selesai tertua dibuang lebih awal; jika semuanya masih berjalan -> `503 busy`.
- Tugas yang gagal dengan error layanan dihapus (tidak disimpan); hanya hasil sukses yang bertahan. Mencoba ulang ID yang sama setelah gagal memicu inferensi baru.
- **Tidak tahan restart dan tidak dibagi antar proses/replika.** Peringatan lama "di balik reverse proxy semua klien terlihat sebagai IP proxy" sudah tidak relevan karena kunci laju/cache memakai UID. Yang tetap: worker tunggal, tanpa replika (`--workers 1`).
- Hitungan laju dihitung per UID untuk setiap permintaan terautentikasi yang diterima, termasuk duplikat/retry dengan UUID sama dan yang berujung error; permintaan ditolak 401/503 auth tidak sampai ke pembatas, dan yang ditolak 429/503 oleh middleware tidak dihitung. Eviction cache tidak mengurangi hitungan laju.
- Batas: 6/UID/menit, 30 global/menit, 4 unggahan diterima serentak, 2 pemrosesan serentak (worker tunggal).

## Timeout

| Fase | Batas |
|---|---|
| Verifikasi auth Firebase | 10 dtk (`503 auth_unavailable`), di luar 85 dtk |
| Unggah badan | 20 dtk (`408`) |
| Pemrosesan (antrean slot + decode + provider + retry) | 85 dtk (`504`), dimulai saat tugas dibuat |
| Groq connect / read | 10 dtk / 65 dtk (fase terpisah) |
| Android call timeout | 100 dtk |

Batas-batas ini terpisah; **tidak ada jaminan total end-to-end 85 dtk**: auth (maks 10 dtk) dan waktu unggah ditambahkan di luar 85 dtk, jadi kasus terburuk (10 + 20 + 85) melewati call timeout Android 100 dtk; margin Android **ketat** dan timeout tidak diubah dalam perubahan auth ini. Klien harus menangani timeout sendiri dan dapat mengulang dengan `X-Scan-Id` yang sama dan file yang sama.

## Persyaratan deployment

- **Selesai:** autentikasi Firebase Bearer wajib dan batas laju/cache per UID pada `POST /api/scan`.
- **Masih tertunda:** TLS/HTTPS (demo lab saat ini HTTP sementara, risiko diterima pengguna) dan reverse proxy dengan batas ukuran body & koneksi yang matang.
- **Masih tertunda:** rate limiter dan cache idempotensi terdistribusi/bersama **sebelum** memakai lebih dari satu worker atau replika; deployment tetap satu worker.
- Panggilan provider dapat memakai kuota/biaya Groq; kami belum melakukan panggilan live.

## Bukti runtime upstream (historis)

README upstream menyatakan satu uji live pada 2026-11-24 dan tes Python/Android. Itu laporan tim AI (sebelum auth) dan **bukan verifikasi kami**. Verifikasi BE saat transfer (historis, pra-auth): 52 tes offline PASS dan upload Swagger terisolasi; tidak menggunakan key yang terekspos atau melakukan inferensi Groq nyata. Bukti terkini untuk auth: menunggu parent.

## Mengekspor dan melihat dokumentasi

- Spesifikasi: [`docs/api/ai-scan.openapi.json`](ai-scan.openapi.json). Unduh dan impor ke Swagger Editor (File -> Import file) atau Redoc yang mendukung OpenAPI 3.1.
- Hanya `GET /health` dan `POST /api/scan` yang didokumentasikan; `POST /api/scan` memakai skema keamanan Bearer Firebase (di Swagger: **Authorize** dengan ID token pengguna, bukan kunci Groq) dan dokumentasi 401/503 `auth_unavailable` mengikuti berkas OpenAPI yang dikelola parent.
- Spesifikasi otomatis FastAPI bawaan (`/openapi.json`) pada implementasi upstream menghilangkan requestBody, header `X-Scan-Id`, dan error karena runtime membaca `request.form` secara manual; berkas ekspor ini sengaja mendokumentasikannya dari implementasi.
- Runtime BE menyajikan berkas kontrak lengkap melalui `/openapi.json`; buka `http://127.0.0.1:8000/docs`, klik **Authorize** dan isi ID token Firebase pengguna, pilih **POST /api/scan → Try it out**, pilih file `image`, lalu **Execute**. Lewat Nginx lab: `http://10.34.211.188/` (HTTP sementara, lihat peringatan di atas). Browser memanggil origin yang sedang dibuka. Dengan token dan kunci Groq valid, Execute memakai kuota Groq nyata.
- Jangan pernah mengekspos `GROQ_API_KEY` ke FE atau menaruhnya di spesifikasi/contoh.
