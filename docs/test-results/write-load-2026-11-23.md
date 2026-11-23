# Bukti beban tulis redeem HTTP (QA-04) — 23 November 2026

Branch dev 64f095d. Tidak ada perubahan kode produksi, SQL, RLS, endpoint, atau kebijakan. Bukan bukti SLA, Supabase, Firebase nyata, maupun Android.

## Ruang lingkup
Harness sementara `.proof-write-load/main.go` (sudah dihapus setelah bukti ini disimpan) merakit **`bootstrap.NewServer` asli** (handler, service, repository PostgreSQL asli) di `127.0.0.1` port ephemeral lewat `bootstrap.Serve`. Satu-satunya stand-in: verifier **sintetis** (token `synthetic:<uid>` → uid); ini **bukan Firebase**. Pool dibuat dengan `postgres.NewPool` tanpa opsi `pool_*`, jadi default **MaxConns = 4**. Database: cluster PostgreSQL 17.12 portable milik harness sendiri (loopback, trust, dibuat lalu dihentikan dan dihapus), database disposable, migration 00001–00012 (bagian Up) diterapkan berurutan. Klien HTTP nyata (`net/http`) dengan 10 worker.

Perintah satu-satunya: `GOMAXPROCS=2 go run ./.proof-write-load` → exit 0, `done; records: 248`, `pg stop err=<nil>`; direktori cluster tidak tersisa. Seluruh hitungan di bawah dihitung via Python dari `result.json` (dihapus) dan dari dump DB mentah.

## Fixture (sintetis, terisolasi)
- 1 mitra `wl-mitra` (partner verified), 8 user `wl-user-01..08`, kategori `Plastik WL`, tarif 150 poin/kg.
- Wallet diisi **hanya lewat jalur sah via HTTP**: tiap user 2× (POST /deposits 2 kg → POST /deposits/{id}/qr → POST /deposit-claims) = 300 poin ×2 = 600 poin; 16 deposit, 16 QR, 16 claim, semuanya 201. Ledger = wallet sebelum beban (S0: seluruh saldo 600, wallet = SUM ledger).
- Reward: A (50 poin, stok 200), B (25 poin, stok 200), C (10 poin, stok 12), Pricey (400 poin, stok 50), Empty (10 poin, stok 0).

## Skenario (192 request HTTP redeem write + 48 setup write)
1. **Wave 1** (10 worker, urutan diacak seed 20261123): 80 kunci unik (8 user × 10; kunci genap → A, ganjil → B) + 32 duplikat simultan (16 kunci × 2 salinan, dikirim bersamaan) = 112 request.
2. **Wave 2** (10 worker): retry sesudah wave 1 untuk 40 kunci ganjil = 40 request.
3. **Penolakan sesudah beban** (24 request, 3 per user): `insufficient_points` (reward Pricey, saldo 225 < 400), `out_of_stock` (reward Empty), `idempotency_conflict` (kunci k00 reward A dipakai ulang ke reward B).
4. **Balapan stok** (16 request, 2 per user, kunci unik) pada reward C stok 12.
5. 8× GET /me/wallet (baca) untuk silang-cek saldo.

## Distribusi status (seluruh 248 request, per fase/jenis)
| fase | jenis | status | kode error | jumlah |
|---|---|---|---|---|
| exhaust | stock_race | 201 | - | 12 |
| exhaust | stock_race | 409 | out_of_stock | 4 |
| final_wallet_read | read | 200 | - | 8 |
| load1 | primary | 200 | - | 1 |
| load1 | primary | 201 | - | 79 |
| load1 | race-dup | 200 | - | 31 |
| load1 | race-dup | 201 | - | 1 |
| load2 | retry | 200 | - | 40 |
| reject | insufficient | 409 | insufficient_points | 8 |
| reject | key_conflict | 409 | idempotency_conflict | 8 |
| reject | out_of_stock | 409 | out_of_stock | 8 |
| setup | claim | 201 | - | 16 |
| setup | deposit | 201 | - | 16 |
| setup | qr | 201 | - | 16 |

- Wave 1: 80 kunci unik, **tepat satu 201 per kunci** (80 total 201 di wave 1+2: 79 primary + 1 race-dup yang menang atas primary), sisanya 200 replay dengan `voucher.id` dan `voucher.code` identik per kunci. 16 kunci dibalapkan 3 salinan: tiap kunci tepat 1×201 + 2×200. Tidak ada kunci yang melanggar (daftar pelanggar kosong).
- Wave 2: 40/40 replay 200. Kunci reuse: 8/8 `idempotency_conflict` 409. Penolakan saldo 8/8 dan stok 8/8 409.
- Balapan stok C: 12× 201, 4× 409 `out_of_stock` (stok 12 → 0 tepat).
- Request gagal tak terduga (status di luar tabel di atas): **0**. 5xx/0/network error: **0**. Saldo negatif: 0.

## Latensi (ms, sisi klien; wall-clock per request loopback)
Server tidak memancarkan header timing (hanya `X-Request-ID`), sehingga latensi diukur klien dari kirim hingga body terbaca. Percentile p95 = nearest-rank.

| kelompok | n | mean | median | p95 | max | min |
|---|---|---|---|---|---|---|
| Setup deposit+QR+claim (HTTP write) | 48 | 8.29 | 2.62 | 27.15 | 123.79 | 1.03 |
| Wave 1 (primary + race-dup) | 112 | 29.39 | 18.51 | 115.41 | 251.06 | 4.22 |
| Wave 2 (retry) | 40 | 3.82 | 3.5 | 5.9 | 6.76 | 1.42 |
| Load 1+2 total | 152 | 22.66 | 12.45 | 114.56 | 251.06 | 1.42 |
|   - status 201 | 80 | 32.17 | 19.1 | 115.84 | 251.06 | 4.22 |
|   - status 200 (replay) | 72 | 12.1 | 5.78 | 47.0 | 114.56 | 1.42 |
| Rejections | 24 | 3.25 | 3.26 | 4.45 | 5.5 | 1.58 |
| Stock race (C) | 16 | 10.59 | 11.77 | 15.9 | 15.9 | 2.17 |
| Semua redeem (192) | 192 | 19.23 | 10.23 | 111.16 | 251.06 | 1.42 |

Durasi dinding fase (ms): wave 1 334, wave 2 17, penolakan 9, balapan stok 20; throughput wave 1 ≈ 335 req/s (hanya deskriptif untuk mesin dev ini, bukan SLA). Request klien in-flight maksimum pada wave 1: 10.

## Pool dan koneksi
- MaxConns terkonfigurasi: 4. Acquired maksimum teramati (sampling ±200 µs, 1263 sampel): **4**; total koneksi maksimum 4.
- Backend klien PostgreSQL maksimum pada database (sampling 2 ms, 320 sampel): **4**.
- Acquired setelah beban: 0; di akhir: 0; setelah shutdown server (grace): 0; shutdown error: `<nil>`.
- Request-ID: 248 unik dari 248 (32 heksadesimal), 248 baris log `http request` server cocok 1:1 dengan ID tersebut.

## Invarian database (dump SQL mentah, sesudah wave 2 / S1, sesudah penolakan / S2, akhir / S3)
| titik | redemption | baris debit ledger | voucher code unik | (user,key) unik | saldo user | wallet = SUM ledger |
|---|---|---|---|---|---|---|
| S0 setelah setup | 0 | 0 | 0 | 0 | 8×600 | ya |
| S1 setelah beban | 80 | 80 | 80 | 80 | 8×225 | ya (balance, total_earned=kredit, total_spent=−debit) |
| S2 setelah penolakan | 80 | 80 | 80 | 80 | 8×225 | ya; wallet, ledger, stok, redemption **identik dengan S1** |
| S3 akhir | 92 | 92 | 92 | 92 | 4×205, 4×215 | ya |

- Logical redemption = voucher = debit ledger: 80 setelah beban (80 kunci unik dari 152 request), 92 akhir (80 + 12 balapan stok). Tiap debit ledger menautkan redemption unik dengan `amount = −cost` dan `idempotency_key = redemption:<id>`.
- Stok berkurang tepat sekali per kunci unik: A 200→160 (−40; 40 kunci), B 200→160 (−40), C 12→0 (−12); Pricey dan Empty tetap 50 dan 0. Jumlah 201 per reward (40/40/12) = penurunan stok.
- Total debit ledger −3120 = −(40×50 + 40×25 + 12×10). Saldo akhir user = 225 − 10 × (201 balapan C milik user); 8/8 cocok. GET /me/wallet 8/8 = saldo DB.
- `voucher.id` dan `voucher.code` pada response = baris `redemptions` DB untuk seluruh response sukses.

## Temuan
Tidak ada bug nyata. Tidak ada kode produksi diubah. (Satu hal operasional harness: `exec.Cmd.CombinedOutput` pada `pg_ctl start` menggantung di Windows karena postmaster mewarisi pipe; harness diperbaiki dengan mengalihkan keluaran ke file. Percobaan pertama itu dimatikan timeout dan cluster-nya dihapus; PG milik pihak lain tidak disentuh.)

## Batasan
- Verifier sintetis, bukan Firebase (tanpa cek revoke/token nyata); PostgreSQL 17.12 loopback portable, bukan Supabase/Supavisor (mode pooler transaksi port 6543 tidak diuji) dan tanpa latensi jaringan; bukan Android; bukan SLA atau uji kapasitas.
- Satu proses, satu mesin dev, GOMAXPROCS=2, satu kali jalan; latensi hanya deskriptif. Latensi wave 1 mencakup antrean pool 4 koneksi vs 10 worker dan kontensi kunci baris reward.
- Batas 5 s per request handler tidak tercapai (latensi maksimum 251 ms); perilaku timeout 504 tidak diuji.
- Hanya jalur redeem (plus setup claim); jalur lain tidak diulang. Tidak ada tes ownership HTTP tambahan (sudah diuji repository).

## Lampiran A — request setup (HTTP write, 48)
| seq | jenis | status | X-Request-ID | ms |
|---|---|---|---|---|
| 1 | deposit | 201 | 16fce9100c13583b31bbcd2e0ff50890 | 19.33 |
| 2 | qr | 201 | 517a0b406cadc12e6c0717b4e5889819 | 17.38 |
| 3 | claim | 201 | 65133b28a74d2c324eea3d36cfafcc52 | 27.15 |
| 4 | deposit | 201 | 5fac6152f747026282af6541409ce830 | 3.29 |
| 5 | qr | 201 | 25a121ae172648eb9be7edf1fc50b6b7 | 123.79 |
| 6 | claim | 201 | 388977ebfabd1e454a2a21fea7a7ac4b | 4.72 |
| 7 | deposit | 201 | 32749ef4ce51101fb37ab304b77d10c2 | 4.33 |
| 8 | qr | 201 | 30d465e470b270277c3b5111a6ebb51e | 102.16 |
| 9 | claim | 201 | 729c9618b3b5fff637df0dcbb8d113e8 | 3.66 |
| 10 | deposit | 201 | a08c3ad852f5b7cfe5d44d68c2f7baea | 1.03 |
| 11 | qr | 201 | c38e5a6cfaf06d88d74b7496d7d140f7 | 2.51 |
| 12 | claim | 201 | cdd7cf88f7065d01ed9c3c1a95cee23d | 2.67 |
| 13 | deposit | 201 | 7471dc295d72711d6b1b22f478c73d0c | 2.62 |
| 14 | qr | 201 | c8e2f92118b6b299259708d0d1fffa6e | 1.73 |
| 15 | claim | 201 | 9aa24205fb4ef6a57decada433d73c70 | 2.65 |
| 16 | deposit | 201 | 3dd2be31eed6cf727af7d67df78720c2 | 2.58 |
| 17 | qr | 201 | 528b95b5f59dc70f216ee614c5cbce39 | 1.05 |
| 18 | claim | 201 | a9c40760bc815087ab43ac84ec22a6b1 | 2.06 |
| 19 | deposit | 201 | 9e518faf392442b0c3e9771a02b56d58 | 2.61 |
| 20 | qr | 201 | 7f64b103dc5a9edd5216eb411a7cac5a | 3.25 |
| 21 | claim | 201 | ba197254f6556db3446031a2a4608095 | 2.68 |
| 22 | deposit | 201 | d5b16941a17aef68fd275ff554589cb3 | 2.06 |
| 23 | qr | 201 | 3db861d74d957ccbf53a3cbeda58ba5d | 1.55 |
| 24 | claim | 201 | 3f2860b680c46d1513a14f6ab0f6482f | 2.57 |
| 25 | deposit | 201 | aa7b2de7973b5bdfea4097505b2a879a | 2.10 |
| 26 | qr | 201 | 83e87c15805ad370e680449627f4d944 | 1.57 |
| 27 | claim | 201 | 0b8346e0f64fde1aa896c10edfaf8454 | 3.72 |
| 28 | deposit | 201 | 0525567e9e748159433fed29292fbeef | 3.32 |
| 29 | qr | 201 | 61b70dbe794fe4337aefa5ba424002a5 | 1.54 |
| 30 | claim | 201 | 091d4329de3456b41a0502ed128110da | 1.58 |
| 31 | deposit | 201 | 5b9845afd38a2df6c7f1aacf7e88b3bb | 2.49 |
| 32 | qr | 201 | 2445836716202fd81ec405cbf4b61131 | 2.48 |
| 33 | claim | 201 | b2d3cab9a71692cc69285edaba254793 | 3.21 |
| 34 | deposit | 201 | e62eae49c922e609f96ff20191eaab7d | 3.25 |
| 35 | qr | 201 | eacaf37b0f1d95096045a49ec2581cd3 | 2.11 |
| 36 | claim | 201 | 75b79381d7561b86d26a2b9178a78e3e | 2.68 |
| 37 | deposit | 201 | 6fb06a65c437f20437cc10a0a06f55eb | 2.92 |
| 38 | qr | 201 | 15c9903dfa5c40e89916aded72b1a6a4 | 1.62 |
| 39 | claim | 201 | b92d75361b75a5e47e4ecb4164016a3e | 2.54 |
| 40 | deposit | 201 | 6abe1483417acc1b6b6c39f481bf2dae | 2.65 |
| 41 | qr | 201 | 1ae44312dbf30aa8087e4c6e7f9c2266 | 1.54 |
| 42 | claim | 201 | 17c0d7f22ca445276f11a6b927a35532 | 3.65 |
| 43 | deposit | 201 | c21bd1c35a4c9aaed4c9fe5fa8e0b74c | 2.12 |
| 44 | qr | 201 | 0f567b14528f47b3011c7faca1f067cf | 1.53 |
| 45 | claim | 201 | 09db2ea22235ed71aa5a32897f7d5221 | 3.11 |
| 46 | deposit | 201 | 14fefca2f3e2bbdcc56f4920def1f497 | 3.12 |
| 47 | qr | 201 | 51603ac43071ac9471afb6bf19c84078 | 2.08 |
| 48 | claim | 201 | 686852728802718cb02eaeaae8fcb0ec | 1.64 |

## Lampiran B — seluruh request redeem (192)
| seq | fase | jenis | uid | Idempotency-Key | reward | status | X-Request-ID | ms |
|---|---|---|---|---|---|---|---|---|
| 49 | load1 | race-dup | wl-user-01 | wl-redeem-u01-k05 | B | 200 | f5cdd0a69350c90517fc184aea725086 | 28.92 |
| 50 | load1 | primary | wl-user-07 | wl-redeem-u07-k00 | A | 201 | 62b1625432372d1000d154468606abfd | 15.04 |
| 51 | load1 | race-dup | wl-user-07 | wl-redeem-u07-k00 | A | 200 | 00fa90482e026dd828b3c6d2dd3a7235 | 73.11 |
| 52 | load1 | primary | wl-user-04 | wl-redeem-u04-k08 | A | 201 | a3ddee27185d4dbaaa765f358841b748 | 251.06 |
| 53 | load1 | race-dup | wl-user-07 | wl-redeem-u07-k00 | A | 200 | 491c923e75fcf6f51e09042e21f7e25b | 15.57 |
| 54 | load1 | primary | wl-user-08 | wl-redeem-u08-k01 | B | 201 | 863412e8b0f92acf11435d93db0bd2ee | 35.09 |
| 55 | load1 | primary | wl-user-01 | wl-redeem-u01-k04 | A | 201 | d8395522078e833023babd776056dab8 | 115.41 |
| 56 | load1 | primary | wl-user-06 | wl-redeem-u06-k07 | B | 201 | 0af5b775e1399138a90b88b8229dff2b | 24.15 |
| 57 | load1 | primary | wl-user-01 | wl-redeem-u01-k05 | B | 201 | d104829675d4d82499f77bd06f0bd355 | 18.91 |
| 58 | load1 | race-dup | wl-user-01 | wl-redeem-u01-k05 | B | 200 | 9d109d12b198698851ad74f17687dc7f | 29.07 |
| 59 | load1 | primary | wl-user-07 | wl-redeem-u07-k02 | A | 201 | 4f58f4d01cfa07b50b924747335ee0bf | 27.26 |
| 60 | load1 | primary | wl-user-04 | wl-redeem-u04-k04 | A | 201 | e7fc87e65f619bd77870ceb0446091fd | 30.43 |
| 61 | load1 | primary | wl-user-02 | wl-redeem-u02-k04 | A | 201 | ae9a20763295cb05e391d35703086221 | 28.11 |
| 62 | load1 | primary | wl-user-03 | wl-redeem-u03-k09 | B | 201 | e66c5355640244a975f351332bebd75c | 26.58 |
| 63 | load1 | primary | wl-user-07 | wl-redeem-u07-k05 | B | 201 | 0ed16ce1b1ce0336c01fa8141f3808d2 | 27.65 |
| 64 | load1 | race-dup | wl-user-07 | wl-redeem-u07-k05 | B | 200 | 091072a6043f9ecb3ec59d72405076b8 | 27.50 |
| 65 | load1 | race-dup | wl-user-07 | wl-redeem-u07-k05 | B | 200 | affa9578c7c210e6e69bdbb25ee14684 | 25.46 |
| 66 | load1 | primary | wl-user-07 | wl-redeem-u07-k09 | B | 201 | 945ae0fc1bca2831e4c884af5f33974f | 21.01 |
| 67 | load1 | primary | wl-user-07 | wl-redeem-u07-k08 | A | 201 | dec7cca518ae14db6a41e2c99ee7b4b7 | 19.34 |
| 68 | load1 | primary | wl-user-02 | wl-redeem-u02-k01 | B | 201 | 1a9dd6bc51b0f978e39aa2e1608a1eba | 18.73 |
| 69 | load1 | primary | wl-user-05 | wl-redeem-u05-k08 | A | 201 | 869c9e47d01fa0f45d28a11432e74d8d | 17.28 |
| 70 | load1 | primary | wl-user-08 | wl-redeem-u08-k03 | B | 201 | 89c35b85a9c9f45518dfb666b402001e | 17.12 |
| 71 | load1 | primary | wl-user-03 | wl-redeem-u03-k00 | A | 200 | f848235ac01d07dd292d9d0bdcb6f8b2 | 16.98 |
| 72 | load1 | race-dup | wl-user-03 | wl-redeem-u03-k00 | A | 201 | 679bf8e6df0849655555dac38be8e775 | 14.24 |
| 73 | load1 | race-dup | wl-user-03 | wl-redeem-u03-k00 | A | 200 | 090531fea236e8c3b35dc643cc36d446 | 14.20 |
| 74 | load1 | primary | wl-user-06 | wl-redeem-u06-k00 | A | 201 | 9af56d32576c128ed7d0377168616911 | 34.66 |
| 75 | load1 | race-dup | wl-user-06 | wl-redeem-u06-k00 | A | 200 | 2d82031ff3260fbc63e2351a5cedbd29 | 34.88 |
| 76 | load1 | race-dup | wl-user-06 | wl-redeem-u06-k00 | A | 200 | 2bb7a8b7f6484424c3cf24d7ef36b21e | 33.59 |
| 77 | load1 | primary | wl-user-01 | wl-redeem-u01-k01 | B | 201 | 2bc51fbb5ec6443ce780021d85cb4f72 | 46.36 |
| 78 | load1 | primary | wl-user-06 | wl-redeem-u06-k08 | A | 201 | 61318849caf6b8b2afa9a4fa97ca398a | 46.88 |
| 79 | load1 | primary | wl-user-04 | wl-redeem-u04-k00 | A | 201 | 1d3ae943f6f59040c52854a1ac01c9d6 | 46.81 |
| 80 | load1 | race-dup | wl-user-04 | wl-redeem-u04-k00 | A | 200 | 98ef5464e5e29225b6076fc3aad53a23 | 47.00 |
| 81 | load1 | race-dup | wl-user-04 | wl-redeem-u04-k00 | A | 200 | 91cf023f460b28613a159ec281ea0d80 | 47.01 |
| 82 | load1 | primary | wl-user-03 | wl-redeem-u03-k06 | A | 201 | 7bc6152fce344b818d591fa4ac4dee70 | 25.51 |
| 83 | load1 | primary | wl-user-08 | wl-redeem-u08-k02 | A | 201 | 38c156020a5dda28d971b126282ce379 | 25.44 |
| 84 | load1 | primary | wl-user-04 | wl-redeem-u04-k07 | B | 201 | aea30a646b10d447fd982bf35bf80e52 | 24.51 |
| 85 | load1 | primary | wl-user-04 | wl-redeem-u04-k05 | B | 201 | 5ff7acbf66dafac622f30f7398920d15 | 12.19 |
| 86 | load1 | race-dup | wl-user-04 | wl-redeem-u04-k05 | B | 200 | 3b704cbfe8fdd049f4623de2981ef4ab | 11.32 |
| 87 | load1 | race-dup | wl-user-04 | wl-redeem-u04-k05 | B | 200 | a0dffbe1f9a96b26771e8fdd966c781b | 114.56 |
| 88 | load1 | primary | wl-user-03 | wl-redeem-u03-k02 | A | 201 | 8a6e4f3fc37c8bd445a97d62f0fc7266 | 112.43 |
| 89 | load1 | primary | wl-user-02 | wl-redeem-u02-k03 | B | 201 | 9de625790b97f6eb45dba858f39d01e2 | 111.16 |
| 90 | load1 | primary | wl-user-01 | wl-redeem-u01-k03 | B | 201 | 113e0b6a460dc4d955c236561cb064af | 110.67 |
| 91 | load1 | primary | wl-user-05 | wl-redeem-u05-k03 | B | 201 | 01d20af39e016bf5f7d0f755c9e18d80 | 114.67 |
| 92 | load1 | primary | wl-user-03 | wl-redeem-u03-k03 | B | 201 | a9051f81553cc1682e1b4a78cf554125 | 115.84 |
| 93 | load1 | primary | wl-user-06 | wl-redeem-u06-k09 | B | 201 | 818c492bfbbbf86731a0cb651ab5642d | 118.72 |
| 94 | load1 | primary | wl-user-01 | wl-redeem-u01-k06 | A | 201 | ba729eb1769ce0c3ec31423acfcd5306 | 122.87 |
| 95 | load1 | primary | wl-user-05 | wl-redeem-u05-k07 | B | 201 | 2efa6806717fae6b977e2ff98fbe86b7 | 117.13 |
| 96 | load1 | primary | wl-user-05 | wl-redeem-u05-k02 | A | 201 | cddbd5a63d9b43a0a6b91c54a26dce97 | 22.24 |
| 97 | load1 | primary | wl-user-08 | wl-redeem-u08-k09 | B | 201 | dfd724c032e85c646ce7b53d0e5c3fdc | 13.81 |
| 98 | load1 | primary | wl-user-05 | wl-redeem-u05-k04 | A | 201 | fc861723d1fa34754445dd570a3660a8 | 19.03 |
| 99 | load1 | primary | wl-user-03 | wl-redeem-u03-k04 | A | 201 | b24c230f717c0aad5d0e4be99fb02ee2 | 23.99 |
| 100 | load1 | primary | wl-user-08 | wl-redeem-u08-k00 | A | 201 | 521d0f9125624aa38b9e8becf3dee697 | 21.05 |
| 101 | load1 | race-dup | wl-user-08 | wl-redeem-u08-k00 | A | 200 | 01b8be7310391101e59d9b5b92d0d10e | 19.89 |
| 102 | load1 | race-dup | wl-user-08 | wl-redeem-u08-k00 | A | 200 | 7240d9950796a0f3b2a2d315404f0947 | 16.99 |
| 103 | load1 | primary | wl-user-06 | wl-redeem-u06-k03 | B | 201 | 44133d4f8c657f18ab8e0e4cd87ddb4a | 14.92 |
| 104 | load1 | primary | wl-user-01 | wl-redeem-u01-k08 | A | 201 | b42eb1f6f7e913eb1184a169416d2ae4 | 15.43 |
| 105 | load1 | primary | wl-user-05 | wl-redeem-u05-k01 | B | 201 | 996fac14dcf234f1effb31a0a0c40c25 | 26.28 |
| 106 | load1 | primary | wl-user-01 | wl-redeem-u01-k02 | A | 201 | a4ba43c37a476e03ed1cc7d7337fee2a | 24.20 |
| 107 | load1 | primary | wl-user-06 | wl-redeem-u06-k02 | A | 201 | c78f502becc7287cbd2a8427be18cfd0 | 24.25 |
| 108 | load1 | primary | wl-user-07 | wl-redeem-u07-k07 | B | 201 | 7925db99f4c2a985b7fa9759e7921fed | 21.59 |
| 109 | load1 | primary | wl-user-02 | wl-redeem-u02-k02 | A | 201 | a2913aaef334f63ebd8be3e7c8f5ec2c | 20.32 |
| 110 | load1 | primary | wl-user-04 | wl-redeem-u04-k03 | B | 201 | a4cda6674777b38b8c27ab58f8feb4b0 | 19.27 |
| 111 | load1 | primary | wl-user-04 | wl-redeem-u04-k02 | A | 201 | 9be8b8f8ea83ff995f81dbfafc24fc87 | 19.77 |
| 112 | load1 | primary | wl-user-02 | wl-redeem-u02-k09 | B | 201 | 96c45f0d066639decb4d468ac9ce7b9b | 19.80 |
| 113 | load1 | primary | wl-user-07 | wl-redeem-u07-k06 | A | 201 | c671d295f9ceedac0e65aa38ed2c5e4f | 21.00 |
| 114 | load1 | primary | wl-user-05 | wl-redeem-u05-k05 | B | 201 | c568418121dd4e559980efc54609d924 | 21.02 |
| 115 | load1 | race-dup | wl-user-05 | wl-redeem-u05-k05 | B | 200 | aacd0dc40e13fb7cc8c4121e666491ee | 10.25 |
| 116 | load1 | race-dup | wl-user-05 | wl-redeem-u05-k05 | B | 200 | f604220c82c88634b008aa519d550015 | 10.78 |
| 117 | load1 | primary | wl-user-01 | wl-redeem-u01-k00 | A | 201 | f592f6e3b044718482b35798f8288216 | 9.17 |
| 118 | load1 | race-dup | wl-user-01 | wl-redeem-u01-k00 | A | 200 | d93d47af0b86fc2c1979cb3abe818120 | 10.23 |
| 119 | load1 | race-dup | wl-user-01 | wl-redeem-u01-k00 | A | 200 | a719bdf68172f4475d9ed9f9a8d10281 | 10.23 |
| 120 | load1 | primary | wl-user-02 | wl-redeem-u02-k05 | B | 201 | a274b157841ec41ae7995204f4c8e96f | 9.19 |
| 121 | load1 | race-dup | wl-user-02 | wl-redeem-u02-k05 | B | 200 | b880d8685d05759f01373f0c9f6dd83b | 8.66 |
| 122 | load1 | race-dup | wl-user-02 | wl-redeem-u02-k05 | B | 200 | 1328503db5ddfc5579906462393c0137 | 8.63 |
| 123 | load1 | primary | wl-user-03 | wl-redeem-u03-k01 | B | 201 | 4304bed4fad5f48e71ff53165b850bd1 | 8.49 |
| 124 | load1 | primary | wl-user-02 | wl-redeem-u02-k06 | A | 201 | e4ded2b1244ef37a7c6a54a155cad494 | 7.42 |
| 125 | load1 | primary | wl-user-07 | wl-redeem-u07-k01 | B | 201 | 37526dfbe164577e753add2ed4cda7bb | 18.10 |
| 126 | load1 | primary | wl-user-02 | wl-redeem-u02-k08 | A | 201 | 233ea2aa58f83b7046c8ef72d9a4199d | 17.04 |
| 127 | load1 | primary | wl-user-04 | wl-redeem-u04-k09 | B | 201 | a4aadb31e53681ed38594624b4f9fc4d | 19.18 |
| 128 | load1 | primary | wl-user-08 | wl-redeem-u08-k04 | A | 201 | 02b4a00ece3b5ede8713c3f6433ea04e | 17.60 |
| 129 | load1 | primary | wl-user-01 | wl-redeem-u01-k07 | B | 201 | 171c88e31a957c5f8a55a63dd1b7bda8 | 18.29 |
| 130 | load1 | primary | wl-user-03 | wl-redeem-u03-k08 | A | 201 | 0aa255021af0d6f7e552a1a153b121a8 | 18.29 |
| 131 | load1 | primary | wl-user-08 | wl-redeem-u08-k05 | B | 201 | 63f79c03f9b1f14d89de14e4cf8448f3 | 18.83 |
| 132 | load1 | race-dup | wl-user-08 | wl-redeem-u08-k05 | B | 200 | 6565cee6c9730b005ff1a7f9a55ae2c8 | 18.82 |
| 133 | load1 | race-dup | wl-user-08 | wl-redeem-u08-k05 | B | 200 | 6f93b5f0b8746e8950bb571029432ee8 | 17.79 |
| 134 | load1 | primary | wl-user-02 | wl-redeem-u02-k00 | A | 201 | 7f9173842dbce4a5c7c0088faa62ce9d | 17.25 |
| 135 | load1 | race-dup | wl-user-02 | wl-redeem-u02-k00 | A | 200 | 0fce5562a74178a5495a364476ff8fd0 | 5.99 |
| 136 | load1 | race-dup | wl-user-02 | wl-redeem-u02-k00 | A | 200 | 52df2039f2e42efe1cc5b1014709ea37 | 6.51 |
| 137 | load1 | primary | wl-user-06 | wl-redeem-u06-k05 | B | 201 | 1f466c502364bb2f8755f9acc0bd611f | 5.97 |
| 138 | load1 | race-dup | wl-user-06 | wl-redeem-u06-k05 | B | 200 | 7c371b49a0da18418a2e360490742440 | 5.97 |
| 139 | load1 | race-dup | wl-user-06 | wl-redeem-u06-k05 | B | 200 | b364e6dd06d28bada7863c44a2a3d108 | 5.28 |
| 140 | load1 | primary | wl-user-04 | wl-redeem-u04-k01 | B | 201 | 41a37bab120d4a2134a98ec4de98865f | 4.76 |
| 141 | load1 | primary | wl-user-04 | wl-redeem-u04-k06 | A | 201 | 257a24ba77836f3682124c75eab6130b | 5.36 |
| 142 | load1 | primary | wl-user-07 | wl-redeem-u07-k03 | B | 201 | abe93d7c1c751b1db1ff3c6cb8380c35 | 4.22 |
| 143 | load1 | primary | wl-user-06 | wl-redeem-u06-k01 | B | 201 | 96bf2d6339d675cd474ff3f48312ba1f | 6.06 |
| 144 | load1 | primary | wl-user-05 | wl-redeem-u05-k06 | A | 201 | 240fa4f08f0f8c3d86d84c824caaa9cb | 6.05 |
| 145 | load1 | primary | wl-user-08 | wl-redeem-u08-k08 | A | 201 | c5381ec81bf4f246533b93ade0e6a087 | 9.19 |
| 146 | load1 | primary | wl-user-06 | wl-redeem-u06-k04 | A | 201 | 208d454fbbf045a06272b9e7a3826802 | 7.08 |
| 147 | load1 | primary | wl-user-02 | wl-redeem-u02-k07 | B | 201 | c51948e47510609c1a69043f0257eb1f | 7.60 |
| 148 | load1 | primary | wl-user-01 | wl-redeem-u01-k09 | B | 201 | 24027c364fc385cc59df44cd51633e7a | 8.94 |
| 149 | load1 | primary | wl-user-05 | wl-redeem-u05-k09 | B | 201 | be100e509b792687c1151a5d69b57f5b | 11.56 |
| 150 | load1 | primary | wl-user-03 | wl-redeem-u03-k07 | B | 201 | 8291baaea632b3930cc5414bd0ad59dc | 12.61 |
| 151 | load1 | primary | wl-user-06 | wl-redeem-u06-k06 | A | 201 | 41ec24d5f13e478182a813594e1c49cf | 8.94 |
| 152 | load1 | primary | wl-user-08 | wl-redeem-u08-k07 | B | 201 | 840714009cc4bce69e2f1750f83250af | 12.54 |
| 153 | load1 | primary | wl-user-03 | wl-redeem-u03-k05 | B | 201 | 418fbe847bf760db7703ff0c847f2562 | 12.36 |
| 154 | load1 | race-dup | wl-user-03 | wl-redeem-u03-k05 | B | 200 | f8a32d176c62e508e228e1c466cdaa55 | 12.37 |
| 155 | load1 | race-dup | wl-user-03 | wl-redeem-u03-k05 | B | 200 | 6b68285ef06a40496300d5fe83344871 | 11.92 |
| 156 | load1 | primary | wl-user-08 | wl-redeem-u08-k06 | A | 201 | 976f13c08e0e8d24a2a6afc45fc011c0 | 9.22 |
| 157 | load1 | primary | wl-user-07 | wl-redeem-u07-k04 | A | 201 | 37600e4c009f00d0b4c960ec346c1719 | 10.86 |
| 158 | load1 | primary | wl-user-05 | wl-redeem-u05-k00 | A | 201 | 15e1f18992f73fb512706368079e66d7 | 10.07 |
| 159 | load1 | race-dup | wl-user-05 | wl-redeem-u05-k00 | A | 200 | f715c0d8f87771b44e076a891bf7b0c1 | 9.54 |
| 160 | load1 | race-dup | wl-user-05 | wl-redeem-u05-k00 | A | 200 | e5a34d3e69415b40197c34a1ef0d90a6 | 9.01 |
| 161 | load2 | retry | wl-user-03 | wl-redeem-u03-k01 | B | 200 | aa820d342c96d90061694eff9319f95a | 2.51 |
| 162 | load2 | retry | wl-user-02 | wl-redeem-u02-k05 | B | 200 | 56b522a16ba51700e0c1baaf6c33e281 | 1.94 |
| 163 | load2 | retry | wl-user-04 | wl-redeem-u04-k09 | B | 200 | 94a47810054963d85e0da54db70c054b | 1.42 |
| 164 | load2 | retry | wl-user-04 | wl-redeem-u04-k07 | B | 200 | 2b795e621744a3583987bf57cde334a0 | 5.69 |
| 165 | load2 | retry | wl-user-06 | wl-redeem-u06-k05 | B | 200 | 5e0f414699d9cd6c0fd50e63a7157a43 | 6.76 |
| 166 | load2 | retry | wl-user-07 | wl-redeem-u07-k07 | B | 200 | 8e0f57d056d968b98720bf2e39001a78 | 4.10 |
| 167 | load2 | retry | wl-user-01 | wl-redeem-u01-k01 | B | 200 | 5563351cf274ec8781b37144caad3788 | 4.10 |
| 168 | load2 | retry | wl-user-08 | wl-redeem-u08-k03 | B | 200 | 1d7ea3d578a61616364e5b3af8d825ff | 4.63 |
| 169 | load2 | retry | wl-user-07 | wl-redeem-u07-k01 | B | 200 | aff908a686ed942510203890022fac11 | 1.42 |
| 170 | load2 | retry | wl-user-04 | wl-redeem-u04-k05 | B | 200 | fb1d52a951c70d770feea3408338615b | 4.10 |
| 171 | load2 | retry | wl-user-08 | wl-redeem-u08-k05 | B | 200 | b70809be49d0c021c021e9b36207d3e3 | 5.90 |
| 172 | load2 | retry | wl-user-03 | wl-redeem-u03-k03 | B | 200 | a17ce9352cf852b4329536a0ea6319e9 | 5.90 |
| 173 | load2 | retry | wl-user-07 | wl-redeem-u07-k03 | B | 200 | 9b6c779a98e1d6165ec5678f1f946598 | 6.45 |
| 174 | load2 | retry | wl-user-05 | wl-redeem-u05-k09 | B | 200 | fefe818a393ec00e1f0b3d2a0fe279b5 | 5.88 |
| 175 | load2 | retry | wl-user-06 | wl-redeem-u06-k09 | B | 200 | 864de94c7bb8cae869ee6b23761e4aec | 4.29 |
| 176 | load2 | retry | wl-user-04 | wl-redeem-u04-k01 | B | 200 | f02d553245989476d155265e48a14623 | 4.81 |
| 177 | load2 | retry | wl-user-06 | wl-redeem-u06-k07 | B | 200 | ceb801f36b0659c57a3ed4d71e22fd32 | 5.34 |
| 178 | load2 | retry | wl-user-03 | wl-redeem-u03-k05 | B | 200 | 343df32bc65e5dcc1c049afd9e621c43 | 4.81 |
| 179 | load2 | retry | wl-user-01 | wl-redeem-u01-k05 | B | 200 | 9eac9a3f3d57f19798a392b25027eec5 | 4.28 |
| 180 | load2 | retry | wl-user-07 | wl-redeem-u07-k05 | B | 200 | 6c23b642b8f92e886cdda1779308f995 | 3.21 |
| 181 | load2 | retry | wl-user-01 | wl-redeem-u01-k07 | B | 200 | c37fdce6a853f94908bebb37eb995d52 | 3.17 |
| 182 | load2 | retry | wl-user-05 | wl-redeem-u05-k05 | B | 200 | 1f746ab547ce829c9ab901c2b446a34e | 3.71 |
| 183 | load2 | retry | wl-user-05 | wl-redeem-u05-k07 | B | 200 | 19e87f556bba8a8a493ffb4f08dce547 | 2.64 |
| 184 | load2 | retry | wl-user-01 | wl-redeem-u01-k03 | B | 200 | 78a971a1f8002b44b744a1c56aefdd4e | 2.64 |
| 185 | load2 | retry | wl-user-03 | wl-redeem-u03-k07 | B | 200 | 4ac2b36acb7bd3445a5728d199a57df7 | 3.80 |
| 186 | load2 | retry | wl-user-07 | wl-redeem-u07-k09 | B | 200 | 5643fae46b9cbdb456f528cd98e4e472 | 3.28 |
| 187 | load2 | retry | wl-user-05 | wl-redeem-u05-k01 | B | 200 | c079849472926b50b57f3fe98d965fae | 3.28 |
| 188 | load2 | retry | wl-user-02 | wl-redeem-u02-k09 | B | 200 | 1848cd0d3d31d0a3faae52dad36a7e2b | 3.28 |
| 189 | load2 | retry | wl-user-02 | wl-redeem-u02-k03 | B | 200 | 89cf1d87a8987ca06864537173c849c9 | 3.29 |
| 190 | load2 | retry | wl-user-01 | wl-redeem-u01-k09 | B | 200 | fe061527b3e823dfdb03cac9407d8585 | 3.81 |
| 191 | load2 | retry | wl-user-08 | wl-redeem-u08-k09 | B | 200 | 3c85e0c1716e3ae1d177d2b706f44c26 | 3.28 |
| 192 | load2 | retry | wl-user-03 | wl-redeem-u03-k09 | B | 200 | 4a56a8078c280ee2a0951fae47edef36 | 3.27 |
| 193 | load2 | retry | wl-user-04 | wl-redeem-u04-k03 | B | 200 | c3b07f9fc61070560fdbba2f774895be | 3.79 |
| 194 | load2 | retry | wl-user-06 | wl-redeem-u06-k01 | B | 200 | cb3f5a71c9d8b82a87f96f3623e4b172 | 3.79 |
| 195 | load2 | retry | wl-user-02 | wl-redeem-u02-k01 | B | 200 | abe5227697b082ebf58d6e5d13528ccc | 2.63 |
| 196 | load2 | retry | wl-user-05 | wl-redeem-u05-k03 | B | 200 | 3ce929bce3221168fff0b267cf9828ed | 3.18 |
| 197 | load2 | retry | wl-user-06 | wl-redeem-u06-k03 | B | 200 | ab1d6da30f9d3499f19b812fb0d03cf9 | 3.17 |
| 198 | load2 | retry | wl-user-08 | wl-redeem-u08-k01 | B | 200 | 690a113edc05a4d21cf7b6697661e430 | 3.17 |
| 199 | load2 | retry | wl-user-08 | wl-redeem-u08-k07 | B | 200 | 1646efed65ef3ea498b970d788ff8f6b | 3.15 |
| 200 | load2 | retry | wl-user-02 | wl-redeem-u02-k07 | B | 200 | 8cc697e99b2830cfd939c749ffcb5dcf | 3.15 |
| 201 | reject | insufficient | wl-user-01 | wl-reject-ins-u01 | Pricey | 409 | a4537e0b6ed1fd7570ce7f6aead9e805 | 2.72 |
| 202 | reject | insufficient | wl-user-04 | wl-reject-ins-u04 | Pricey | 409 | 91d8091691ad0d7ce5aea25d412e52e2 | 2.72 |
| 203 | reject | out_of_stock | wl-user-01 | wl-reject-oos-u01 | Empty | 409 | 0fd202fd64218bff6404df0e0de9d066 | 2.19 |
| 204 | reject | key_conflict | wl-user-01 | wl-redeem-u01-k00 | B | 409 | 86845806a68ed665f543711a8f9e1cf4 | 3.91 |
| 205 | reject | insufficient | wl-user-02 | wl-reject-ins-u02 | Pricey | 409 | 7b3f61d849264e5fd1f866467e4d5db3 | 3.91 |
| 206 | reject | out_of_stock | wl-user-02 | wl-reject-oos-u02 | Empty | 409 | 65140cd2897c6ef02e6a48792b441254 | 3.91 |
| 207 | reject | key_conflict | wl-user-02 | wl-redeem-u02-k00 | B | 409 | 8522dcf65537d557b5251d0e54f6126d | 1.65 |
| 208 | reject | insufficient | wl-user-03 | wl-reject-ins-u03 | Pricey | 409 | dd86f4babefcd0c3351acaf4f105a445 | 5.50 |
| 209 | reject | out_of_stock | wl-user-03 | wl-reject-oos-u03 | Empty | 409 | f2bf4dd7138cddd9b600a35e0fb6e2af | 3.91 |
| 210 | reject | key_conflict | wl-user-03 | wl-redeem-u03-k00 | B | 409 | 562fecb68a11ad535984edcb07077216 | 3.91 |
| 211 | reject | out_of_stock | wl-user-04 | wl-reject-oos-u04 | Empty | 409 | 9b6600080c5315b4a0b24e7b9048eec5 | 3.32 |
| 212 | reject | key_conflict | wl-user-04 | wl-redeem-u04-k00 | B | 409 | 38dd4e28a3f7d7815b35f65f0680f272 | 3.32 |
| 213 | reject | insufficient | wl-user-05 | wl-reject-ins-u05 | Pricey | 409 | 817d6ece4f24bfd087e7aca8dad0d2e4 | 3.85 |
| 214 | reject | out_of_stock | wl-user-05 | wl-reject-oos-u05 | Empty | 409 | 03108a51c07d99139a109acfb4a59cb5 | 4.45 |
| 215 | reject | key_conflict | wl-user-05 | wl-redeem-u05-k00 | B | 409 | 3b00d0e95a1ed93c41af624a0124fe11 | 3.79 |
| 216 | reject | insufficient | wl-user-06 | wl-reject-ins-u06 | Pricey | 409 | ea24e70a2a3c62cc5f82f5a27d7d7cb2 | 2.66 |
| 217 | reject | out_of_stock | wl-user-06 | wl-reject-oos-u06 | Empty | 409 | 76b5128303d7ea8e83f6cdddb7fcfb94 | 3.26 |
| 218 | reject | key_conflict | wl-user-06 | wl-redeem-u06-k00 | B | 409 | 9e9bea0c0d38560e90627a915c193952 | 3.25 |
| 219 | reject | insufficient | wl-user-07 | wl-reject-ins-u07 | Pricey | 409 | 7aca8045e5771fb84cfdc597fed6f891 | 3.23 |
| 220 | reject | out_of_stock | wl-user-07 | wl-reject-oos-u07 | Empty | 409 | fdf9218a343f0ddb23e5d83bd9dc05b0 | 2.71 |
| 221 | reject | key_conflict | wl-user-07 | wl-redeem-u07-k00 | B | 409 | ae1473f2c0eea63609b8dbaec2708b09 | 2.71 |
| 222 | reject | insufficient | wl-user-08 | wl-reject-ins-u08 | Pricey | 409 | 274fd10bea07f5dbb50636b3b7c3dff1 | 3.24 |
| 223 | reject | out_of_stock | wl-user-08 | wl-reject-oos-u08 | Empty | 409 | 568a9a392dd1b114ea1b385e14afa62b | 2.18 |
| 224 | reject | key_conflict | wl-user-08 | wl-redeem-u08-k00 | B | 409 | 57979f205925b68a7a25659b93706ce7 | 1.58 |
| 225 | exhaust | stock_race | wl-user-03 | wl-exhaust-u03-k0 | C | 201 | 9217a1b8add0371f255ebb0f319d7e8f | 15.90 |
| 226 | exhaust | stock_race | wl-user-05 | wl-exhaust-u05-k1 | C | 201 | 52ed0d4c5458b6d95cad25e9296f6831 | 10.11 |
| 227 | exhaust | stock_race | wl-user-01 | wl-exhaust-u01-k1 | C | 201 | 360147cf49912bd23757b4b23895f8d6 | 14.86 |
| 228 | exhaust | stock_race | wl-user-05 | wl-exhaust-u05-k0 | C | 201 | 0103166c0a58397195a9e22ef2fa5cae | 13.27 |
| 229 | exhaust | stock_race | wl-user-04 | wl-exhaust-u04-k0 | C | 201 | b42cbb4c3e81a5064fc208b19c335305 | 11.71 |
| 230 | exhaust | stock_race | wl-user-02 | wl-exhaust-u02-k0 | C | 201 | 28c6b0972a428905632209fd7ecc6b09 | 8.03 |
| 231 | exhaust | stock_race | wl-user-07 | wl-exhaust-u07-k1 | C | 201 | 756c54c1c2e25dd41b5efd561d8ef2e0 | 2.17 |
| 232 | exhaust | stock_race | wl-user-08 | wl-exhaust-u08-k1 | C | 201 | af5b6f1a005d4cef97b25889c9743994 | 4.78 |
| 233 | exhaust | stock_race | wl-user-06 | wl-exhaust-u06-k1 | C | 201 | d061297dd0dc7a49c3f4f9baf8b84623 | 6.36 |
| 234 | exhaust | stock_race | wl-user-01 | wl-exhaust-u01-k0 | C | 201 | c0a5ce51cacbc595ef74777d942bb16f | 3.74 |
| 235 | exhaust | stock_race | wl-user-04 | wl-exhaust-u04-k1 | C | 201 | bcc08b50d4fc6224e1fab897ae352386 | 14.80 |
| 236 | exhaust | stock_race | wl-user-06 | wl-exhaust-u06-k0 | C | 201 | fbb3d3363200fb74a93d9ab20861082e | 14.61 |
| 237 | exhaust | stock_race | wl-user-08 | wl-exhaust-u08-k0 | C | 409 | 96a2cb26a534f812547f473ff1771611 | 13.57 |
| 238 | exhaust | stock_race | wl-user-03 | wl-exhaust-u03-k1 | C | 409 | caaf52a61c9f3f752815238e3f542bc5 | 13.50 |
| 239 | exhaust | stock_race | wl-user-02 | wl-exhaust-u02-k1 | C | 409 | 62f0a21e00ff2d60e408205fb7d433a2 | 11.83 |
| 240 | exhaust | stock_race | wl-user-07 | wl-exhaust-u07-k0 | C | 409 | 9ce79cc234e180e65ecb998314f7a34f | 10.27 |
