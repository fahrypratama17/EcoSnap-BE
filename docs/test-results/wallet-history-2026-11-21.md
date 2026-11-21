# Verifikasi saldo dan riwayat poin

Tanggal: 21 November 2026. Branch: feat/wallet-history dari feat/deposit-creation (cd1ba52). Tidak ada merge/push.

Perintah: `./scripts/test-postgres-portable.ps1 -PostgresBin …` — exit 0, seluruh tes PASS, 0 SKIP. `gofmt` dan `go vet` bersih. Dua baris log seed adalah skenario penolakan yang memang diuji.

| Tes | Perilaku | Hasil |
|---|---|---|
| wallet_matches_own_ledger | 5 klaim × 150 = saldo 750 = total_earned = jumlah riwayat; tiap entri membawa nama mitra/berat; urutan terbaru di atas. | PASS |
| pages_are_stable_with_ties_and_new_entries | Timestamp identik dipaksa; limit 2; klaim baru masuk di tengah scroll → 5 entri lama tetap muncul tepat sekali; refresh menampilkan 6. | PASS |
| history_is_per_user | User lain hanya melihat miliknya; user tanpa riwayat mendapat array kosong. | PASS |
| partner_deposits_filter_and_ownership | Mitra pemilik melihat 7 claimed / 1 qr_issued / 1 created lewat paging; filter status; claimed_at hanya pada claimed; mitra lain kosong; user, mitra pending, profil hilang ditolak. | PASS |
| invalid_queries_and_inactive_accounts | limit 0/101, cursor rusak, status pada riwayat poin, status tidak dikenal → invalid; akun suspended ditolak. | PASS |
| TestWalletHTTP | Default limit 20, cursor next/null, parameter asing/ganda/limit non-angka/uid ditolak. | PASS |
| Suite sebelumnya | Akun, mitra, estimator, setoran, QR/klaim, migration 00001-00007. | PASS |

Batas: data sintetis PostgreSQL portable 17.12; belum Supabase/Firebase nyata/Android. Tidak ada migration baru; index yang dipakai sudah ada di 00006/00007.
