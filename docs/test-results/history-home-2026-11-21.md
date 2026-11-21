# Verifikasi edge case, riwayat, dan beranda

Tanggal: 21 November 2026. Branch: feat/history-home dari dev (d57bd5a). Keputusan D-22. Tidak ada push.

Perintah: `./scripts/test-postgres-portable.ps1 -PostgresBin …` — exit 0, seluruh tes PASS, 0 SKIP. `go vet` bersih.

Run pertama gagal pada kasus 0 poin: subtes sebelumnya menaikkan tarif plastik owner-a menjadi 999/kg, sehingga 0,006 kg menghasilkan 5 poin dan setoran tersimpan. Berat uji diganti 0,001 kg (0,999 → 0); kode produksi tidak diubah.

| Tes | Perilaku | Hasil |
|---|---|---|
| TestDepositIntegration/authorization_and_rejections_leave_no_rows | Setoran 0 poin ditolak `ErrZeroPoints` tanpa baris. | PASS |
| TestDepositQRClaimIntegration/preview_does_not_mutate | Preview membawa `server_time` di antara waktu terbit dan kedaluwarsa. | PASS |
| TestFreshAuthRejectsRevokedTokens | Token yang lolos verifikasi biasa tetapi dicabut ditolak 401 pada guard klaim/redeem. | PASS |
| TestHistoryAndHomeIntegration/history_filters_by_type_and_time | Filter `type`, `from` inklusif, `to` eksklusif; filter waktu riwayat setoran mitra. | PASS |
| …/claimer_sees_deposit_detail_only | Detail setoran untuk pengklaim; user lain, mitra, setoran belum diklaim → 404. | PASS |
| …/user_home_totals_match_sources | Saldo, jumlah dan berat klaim, voucher aktif (kedaluwarsa tidak dihitung), 3 aktivitas terbaru; user baru semua nol. | PASS |
| …/partner_home_counts_by_status | Hitungan created/qr_issued/claimed, berat dan poin terklaim; mitra pending tetap bisa membaca; user biasa 403. | PASS |
| TestWalletHTTP | Parameter type/from/to divalidasi, `+` ter-encode, rentang terbalik ditolak, rute home/detail. | PASS |
| Suite sebelumnya | Seluruh tes lama, migration 00001-00008. | PASS |

Tidak diuji: Firebase nyata (revocation memakai stub), Supabase transaction pooler (mode exec hanya dipilih berdasarkan port 6543), Android.
