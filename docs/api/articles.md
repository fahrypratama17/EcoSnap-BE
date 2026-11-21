# Kontrak artikel

Status: implementasi backend pada feat/articles (CONTENT-01). Bukti: [laporan](../test-results/articles-2026-11-21.md). Konten adalah data demo (D-23); selama admin ditunda, isi dikelola lewat file seed.

Struktur mengikuti FE-19 (`Article`, `ArticleSection`, `ArticleCategory`): ringkasan, kategori, durasi baca, pembuka, beberapa bagian, tip, dan penanda featured. Semua teks adalah **plain text**; Android menampilkannya sebagai teks biasa, bukan HTML/Markdown.

## GET /api/v1/articles

Artikel yang **terbit** (is_published dan `published_at` sudah lewat), terbaru di atas.

| Query | Keterangan |
|---|---|
| `limit` | 1-100, default 20 |
| `cursor` | dari `pagination.next_cursor` |
| `q` | cari di judul atau ringkasan, tidak peka huruf besar/kecil, maks. 100 karakter; `%` dan `_` dicari apa adanya |
| `category` | `GUIDE` \| `SORTING` \| `RECYCLING` \| `SPECIAL_WASTE` \| `NEWS` |
| `featured` | `true` hanya artikel unggulan |

```json
{"data":[{"id":"…","title":"Cara Memilah Sampah Rumah Tangga dengan Benar","summary":"…","category":"SORTING","reading_minutes":4,"is_featured":true,"is_demo":true,"published_at":"2026-11-01T01:00:00Z"}],
 "pagination":{"limit":20,"next_cursor":null}}
```

## GET /api/v1/articles/{articleID}

Field daftar ditambah:

```json
{"introduction":"…","sections":[{"title":"Siapkan tiga wadah","body":"…"}],"tip":"…"}
```

`sections` selalu array (bisa kosong). Draft, artikel terjadwal yang belum waktunya, atau ID tidak dikenal → 404 `article_not_found`. "Selesai membaca" tetap navigasi Android; tidak dicatat di server (misi menunggu Q-07).

## Error

| Kondisi | Status | Kode |
|---|---|---|
| Parameter tidak dikenal/ganda, kategori salah, `featured` bukan boolean, `limit`/`cursor` salah | 400 | invalid_query |
| Artikel tidak ada/belum terbit | 404 | article_not_found |

## Schema dan seed

Migration `00009_create_articles.sql`: tabel `articles` dengan enum kategori, `sections` JSONB (array), CHECK artikel terbit wajib punya `published_at`.

```powershell
go run ./cmd/seed-articles --demo   # membaca data/seeders/dev/articles.json
```

Berbeda dengan seed lain, seed artikel **memperbarui** artikel demo dengan ID yang sama (file JSON = alat edit selama admin ditunda). Set `"is_published": false` untuk menyembunyikan. Artikel yang bukan demo (`is_demo=false`) tidak pernah ditimpa, dan artikel yang tidak ada di file tidak dihapus.
