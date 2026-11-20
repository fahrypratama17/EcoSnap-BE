# Verifikasi QR setoran dan klaim

Tanggal: 20 November 2026. Branch: feat/deposit-creation. Keputusan: D-18 (QR bearer, pengklaim pertama), D-19 (terkunci setelah QR terbit), D-05/D-06 (10 menit, reissue). Tidak ada merge/push.

Perintah: `./scripts/test-postgres-portable.ps1 -PostgresBin …` — exit 0, seluruh tes PASS, 0 SKIP, server stopped. `go vet` exit 0; gofmt bersih. Dua baris log seed pada output adalah skenario penolakan yang memang diuji.

| Tes | Perilaku | Hasil |
|---|---|---|
| issue_locks_deposit_and_stores_only_hash | Token 43 karakter, TTL 10 menit, warning ada, status qr_issued, token mentah tidak tersimpan; mitra lain/user/mitra pending ditolak. | PASS |
| preview_does_not_mutate | Preview menampilkan total/nama mitra/item; wallet dan status tetap; mitra ditolak. | PASS |
| reissue_revokes_old_token | Token lama → qr_revoked, histori 2 token, token baru dapat diklaim. | PASS |
| expiry_boundary_and_unknown_tokens | Tepat pada expires_at → qr_expired tanpa kredit; token kosong/pendek/tidak dikenal → qr_invalid; reissue setelah expired berhasil diklaim. | PASS |
| mitra_cannot_claim | Mitra pemilik dan mitra lain → claim_forbidden. | PASS |
| first_of_parallel_claimers_wins_once | 8 klaim paralel dari 2 user: satu pemenang, kredit 455 sekali, wallet = ledger, replay pemenang 200 tanpa kredit ulang, reissue setelah klaim ditolak. | PASS |
| reissue_racing_claim_credits_at_most_once | 5 putaran klaim vs reissue bersamaan: klaim duluan → reissue ditolak dan tidak ada token hidup; reissue duluan → klaim qr_revoked tanpa kredit. | PASS |
| ledger_is_the_wallet_source | Semua wallet = jumlah ledger. | PASS |
| TestDepositQRHTTP | 201 issue + warning, preview 200, token di query/field asing/format salah ditolak, klaim 201, replay 200, pengklaim kedua 409, user tidak dapat issue. | PASS |
| Suite sebelumnya | Termasuk migration 00001-00007 Up/Down/reapply. | PASS |

Batas: data sintetis PostgreSQL portable 17.12; belum Supabase, token Firebase nyata, Android scan, maupun version tracking Goose. wallet.balance INTEGER.
