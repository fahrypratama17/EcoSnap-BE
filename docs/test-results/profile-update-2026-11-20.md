# Verifikasi ulang dan edit profil

Tanggal: 2026-11-20. Ruang C pada awal pemeriksaan: 2.824.339.456 byte (sekitar 2,8 GB). Tidak ada pembersihan cache/data untuk batch ini.

Baseline dijalankan sebelum perubahan melalui scripts/test-postgres-portable.ps1 memakai runtime lokal di LOCALAPPDATA/EcoSnapTestPostgres/minimal/pgsql/bin. Hasil exit 0: seluruh tes HTTP/service dan suite PostgreSQL akun/favorit/mitra/seed PASS, tanpa SKIP. PostgreSQL dihentikan dan cluster sementara dibersihkan.

Bagian belum lengkap pada tracker adalah AUTH-04 (PATCH profil). Ditambahkan PATCH /api/v1/me melalui account_profile_update.go pada service/repository/handler, tanpa migration atau perubahan frontend. Request hanya menerima full_name dan phone. Update mengunci row akun, menolak akun inactive, menghitung display_name dari nama lengkap, dan tidak mengubah wallet atau hak akun.

Tes HTTP awal gagal karena route PATCH belum tersedia. Setelah implementasi, test target PASS. Full suite PostgreSQL dijalankan kembali dengan -count=1 melalui runner yang sama, exit 0 tanpa SKIP:

- TestProfileUpdateHTTP PASS: partial update, batas input, penolakan role/email/balance/uid query/null/tipe salah, 401/403/413/415.
- TestAccountProfileUpdateIntegration PASS: nama dan nomor dipertahankan sesuai partial request, Unicode 100 rune dengan display 80 rune, dua update bersamaan pada field berbeda, retry, wallet tetap 250, data user lain tetap, missing/suspended ditolak.
- Suite akun sebelumnya tetap PASS, termasuk rollback dan 16 request paralel.
- Suite favorit/query mitra tetap PASS, termasuk 12 insert paralel dan isolasi antar-user.
- Command seed dua kali dan penolakan role user tetap PASS.

Runner terakhir mencetak server stopped dan membersihkan direktori cluster miliknya. Tidak ada perubahan Supabase, pemakaian ID token Firebase nyata, atau integrasi Android. Keberhasilan pengujian lokal tidak menandai deployment dan integrasi tersebut selesai.

Pemeriksaan akhir: go vet -p 1 -mod=readonly ./... dan go build -p 1 -mod=readonly ./... exit 0. Gofmt dan git diff --check lolos.
