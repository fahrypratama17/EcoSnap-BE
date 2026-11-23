# Gate CI kontrak integrasi — 23 November 2026

## Perubahan

Workflow `.github/workflows/ci.yml` tetap berjalan untuk push dan pull request menuju `main`/`dev`. Job `test` (gofmt, vet dan full suite Go dengan PostgreSQL disposable serta penolakan SKIP) tidak diubah. Dua job ditambahkan:

- `api-contracts`: Ubuntu, Python 3.12 dan Node 22. Memasang dependency tooling, memvalidasi OpenAPI/Postman dan menjalankan uji negatif CLI.
- `smoke-offline`: Windows PowerShell 5.1. Mem-parse tiga script smoke dan menjalankan harness flow in-memory yang sudah tersedia.

Tidak membutuhkan Firebase token, password, secret CI, Supabase atau database remote. Source aplikasi Go, migration, kontrak API, koleksi/environment contoh serta `.env.example` tidak diubah. Branch protection GitHub tidak diubah.

## Kontrak gate

`scripts/check-api-contracts.py` menerima `--root <repo>` dan default-nya mengikuti lokasi script. Validasi mencakup OpenAPI dan referensi lokal, operation ID unik, duplicate key YAML, JSON Schema Postman v2.1, environment ekspor tanpa credential terisi, variable shadowing dan cakupan route. Schema resmi Postman di-vendor di `scripts/postman-collection-v2.1.schema.json`; tidak diunduh saat gate berjalan. External OpenAPI `$ref` ditolak sebelum validator dapat mengambilnya lewat jaringan.

Inventaris route berasal dari registrasi literal `Handle`/`HandleFunc` dalam `internal/delivery/http/router.go` dan `*_routes.go`. Go dan OpenAPI dibandingkan sebagai himpunan tepat setelah normalisasi nama parameter. URL contoh negatif Postman (UUID maupun slug misi) dapat dipetakan ke parameter OpenAPI; endpoint literal baru di bawah path berparameter tidak dianggap otomatis tercakup. Tidak ada hitungan route yang ditanam tetap.

`scripts/check-postman-scripts.cjs` memeriksa sintaks script koleksi, folder dan request sebagai function body tanpa mengeksekusinya. `return` di tingkat atas script diterima. Error menyebut konteks request/event tetapi tidak menyalin body script atau cuplikan JSON. Gate ini tidak mengeksekusi `pm` dan bukan pengganti Newman terhadap server.

## Verifikasi lokal yang dijalankan

Toolchain lokal: Python 3.12.3, Node 24.14.0, Windows PowerShell 5.1.26100.9818. CI mengatur Node 22; hasil lokal Node 24 tidak diklaim sebagai eksekusi Node 22.

| Pemeriksaan | Hasil terakhir |
|---|---|
| Instalasi virtualenv terpisah dari requirements tooling | PASS: PyYAML 6.0.4, jsonschema 4.26.0, openapi-spec-validator 0.9.0 |
| CLI kontrak pada artefak repository | PASS |
| Regresi CLI Python | 12 tes PASS |
| Sintaks script Postman aktual | 222 script PASS |
| Regresi CLI Node | 4 tes PASS |
| Parser tiga script PowerShell | PASS |
| Harness flow offline | 84 pemeriksaan PASS, 0 FAIL |
| Parse YAML workflow | PASS |

Regresi Python mencakup route baru yang belum didokumentasikan, route literal di bawah parameter, route OpenAPI tanpa request Postman, operation ID ganda, YAML key ganda, `$ref` rusak/eksternal, schema koleksi salah, URL tidak teresolusi, token terisi dan credential tersembunyi pada variable duplikat. Fixture berada di direktori sementara dan dibersihkan.

Regresi Node mencakup traversal koleksi/folder/request, `exec` string/array dan `return`, sintaks rusak dengan konteks error, JSON rusak serta file hilang; nilai sensitif pada fixture tidak muncul di error.

Pada pengembangan gate, assertion duplicate-operation-ID diperbaiki untuk memeriksa kategori error validator, bukan wording buatan. Pemetaan route juga diperbaiki setelah CLI positif menolak slug misi; verifikasi terakhir di atas dijalankan setelah perbaikan. Tidak ada input kontrak yang dilonggarkan untuk membuat gate lulus.

## Menjalankan ulang

Dari virtualenv Python 3.12 aktif, Node 22+ tersedia:

```text
python -m pip install -r scripts/requirements-api-contracts.txt
python scripts/check-api-contracts.py
node scripts/check-postman-scripts.cjs
python scripts/test-api-contracts.py
node --test scripts/test-postman-scripts.cjs
```

PowerShell: `./scripts/test-smoke-flows.ps1`. Perintah yang sama tercantum pada workflow dan [indeks API](../api/README.md#gate-ci-kontrak-dan-smoke-offline).

## Batas bukti

Ini gate schema/route/sintaks dan perilaku validator flow, bukan validasi semua field respons server secara otomatis. Route Go dinamis tidak diinventarisasi. Newman full flow, generated Kotlin client, Firebase/Supabase nyata, Android dan hosting tetap memerlukan pengujian tersendiri. Tidak menjalankan ulang full suite Go secara lokal karena kode Go dan job lamanya tidak berubah. Status run GitHub diperiksa setelah publikasi; hasil lokal tidak dipakai untuk mengklaim GitHub sukses.
