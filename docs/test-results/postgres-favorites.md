# Hasil pengujian favorit dan PostgreSQL

Tanggal: 2026-11-17. Lingkungan: Windows, PostgreSQL portable EDB 17.12 (diverifikasi melalui postgres --version), cluster sementara di loopback. Database aplikasi/Supabase tidak digunakan.

Perintah yang benar-benar dijalankan:

```powershell
./scripts/test-postgres-portable.ps1 -PostgresBin "$env:LOCALAPPDATA/EcoSnapTestPostgres/minimal/pgsql/bin"
```

Hasil akhir exit 0. TestAccountRepositoryIntegration PASS (9 subtest), TestPartnerFavoritesIntegration PASS (4 subtest), TestPartnerSeedCommandIntegration PASS; HTTP dan service juga PASS. Tidak ada SKIP pada run ini. Full suite sebelum environment database tersedia menghasilkan 22 tes biasa PASS, sementara suite database masih SKIP pada saat itu. Bukti terakhir menggantikan batas verifikasi lama untuk PostgreSQL lokal.

Yang dibuktikan:

- Akun/settings/wallet serta usaha pending tersimpan dan dapat dibaca.
- Enam belas retry pendaftaran paralel menghasilkan satu pembuatan akun.
- Kegagalan wallet dan detail usaha tidak meninggalkan data parsial; retry dapat membuat akun baru.
- Konflik pendaftaran tidak mengubah profil lama.
- Dua belas penambahan favorit paralel menghasilkan satu favorit per user/lokasi.
- User lain tidak dapat membaca atau menghapus favorit caller.
- Target hidden tidak dapat ditambah/tidak ditampilkan, tetapi tetap dapat dihapus idempotent.
- Akun missing/suspended ditolak pada operasi favorit.
- Query daftar/detail/kategori PostgreSQL dan filter area/material/owner status bekerja pada fixture.
- Seed CLI dua kali menghasilkan satu lokasi, empat kategori, tujuh jam, dan tiga material; seed akun user ditolak tanpa menaikkan role.
- SQL migration 00001-00004 dapat dibalik dan diterapkan lagi pada database disposable.

Run pertama menemukan dua masalah pada harness tes, bukan kegagalan favorit:

1. Destructive Down/Up membuat ulang enum sehingga cache tipe pada koneksi lama menyimpan OID yang sudah tidak ada. Fixture sekarang memanggil pool.Reset sebelum melanjutkan query setelah reapply. Ini meniru restart/reconnect aplikasi; tidak menjanjikan rollback schema destruktif aman terhadap server yang terus berjalan.
2. ConnConfig.ConnString menyimpan string awal, walaupun field Database pada config disalin/diubah. Child process seed semula menyambung database postgres admin. Fixture sekarang membangun URL dari Host/Port/User/Password/Database aktual. Password tes tidak dicatat pada laporan.

Server tes berhenti dengan output server stopped, dan direktori cluster tes dibersihkan. Runtime portable tetap berada di cache lokal untuk pengujian berikutnya. Tidak ada Windows service, migration Supabase, seed Supabase, perubahan Android, atau penggunaan credential Firebase nyata.

Batas: tidak menguji client RLS anon/authenticated, version tracking Goose CLI, Firebase signature/expiry/project nyata, emulator/perangkat Android, maupun deployment jaringan. Status aplikasi live masih memerlukan migration 00002-00004 di database development dan pengujian token nyata.

Suite dijalankan ulang setelah pemeriksaan HTTP diubah agar mengamati hasil melalui GET endpoint. Run terakhir juga exit 0, tanpa SKIP; server berhenti dan cluster dibersihkan. `go vet -p 1 -mod=readonly ./...` dan `go build -p 1 -mod=readonly ./...` selesai exit 0; gofmt dan pemeriksaan link lokal dokumen tidak menemukan masalah.
