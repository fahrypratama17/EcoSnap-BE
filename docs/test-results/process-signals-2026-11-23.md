# Shutdown proses cmd/app — 23 November 2026

## Cakupan

Executable `cmd/app` asli dibangun dengan `go build -p 1 -mod=readonly`, tanpa perubahan kode produksi atau injeksi verifier ke aplikasi. PostgreSQL 17.12 portable, cluster trust **loopback-only** disposable milik harness, migration 00001–00012 diterapkan dengan Goose. Tidak memakai environment database development atau credential Firebase asli.

Firebase Admin SDK berjalan dalam **mode emulator**, memakai JWT sintetis dan stand-in HTTP account lookup di loopback. SDK tetap melakukan lookup akun dalam mode emulator; fixture hanya mengembalikan akun sintetis aktif. Ini bukan emulator Firebase resmi, verifikasi token produksi, tes revoked/disabled, maupun integrasi Firebase nyata.

Harness Python sementara `.proof-signal/main.py` sudah dihapus setelah verifikasi. Perintah akhir `python .proof-signal/main.py` exit **0**. Hasil mentah disimpan lokal di `%TEMP%/ecosnap-signal-proof-2026-11-23.json`. Durasi diukur `time.perf_counter()` dan dikonversi ke milidetik melalui Python eval; bukan batas waktu yang dijanjikan.

## Hasil

| Skenario proses Windows | Sinyal OS | Exit | Durasi shutdown (ms) | Request aktif | Koneksi aplikasi tersisa di PostgreSQL |
|---|---|---|---|---|---|
| Idle setelah `/health` dan pembacaan profil sukses | CTRL_C_EVENT | 0 | 4.795 | Tidak ada | 0 |
| Pembacaan profil sedang menunggu lock tabel `accounts` | CTRL_BREAK_EVENT | 0 | 271.511 | Selesai HTTP 200 sesudah lock dilepas | 0 |

Kedua proses mencatat `Sinyal berhenti diterima, server HTTP berhenti dengan rapi` dan `Pool PostgreSQL ditutup`. Listener tidak dapat dihubungi setelah exit.

Untuk request aktif, harness menahan lock dalam transaksi PostgreSQL terpisah dan memastikan query aplikasi terlihat dengan `wait_event_type='Lock'` sebelum mengirim sinyal. Sesudah sinyal, listener menolak koneksi baru sementara proses masih hidup dan request belum selesai. Lock baru dilepas setelah keadaan tersebut teramati; request kemudian mendapat HTTP 200 dan `X-Request-ID`, lalu proses keluar 0. Jadi hasil bukan sekadar mematikan proses idle.

Console helper dibuat terpisah (`CREATE_NEW_CONSOLE`); anggota console diperiksa hanya berisi proses milik harness (2 saat idle, 3 dengan blocker psql). Ctrl+C ditujukan ke console terisolasi. Kasus Ctrl+Break memakai process group aplikasi sendiri agar blocker tidak ikut terbunuh dan melepaskan lock sebelum waktunya. Terminal pengguna dan proses pihak lain tidak menerima sinyal.

Cluster dihentikan lewat `pg_ctl ... -m fast -w stop`; direktori cluster beserta executable sementara dihapus. Hasil akhir `cluster_stopped_and_removed=true`.

## Percobaan harness yang gagal

1. `subprocess.run(..., capture_output=True)` untuk `pg_ctl start` menggantung karena postmaster Windows mempertahankan pipe. Cluster terbukti ready, bukan kegagalan startup backend. Output perintah dialihkan ke temporary file dan stdin ke DEVNULL; cluster milik percobaan ini dipakai ulang lalu dihentikan/dihapus.
2. Token emulator tanpa endpoint lookup mendapat 401: SDK `verifyIDToken` tetap memanggil `checkRevokedOrDisabled` jika `isEmulator=true`. Ditambahkan fixture lookup HTTP loopback yang dinyatakan di atas; tidak mengubah auth produksi.
3. Guard console pertama mengharapkan dua proses pada kasus aktif, padahal blocker psql adalah proses ketiga. Guard diperbaiki agar memeriksa PID milik harness secara tepat; grup aplikasi diisolasi agar sinyal tidak mengenai blocker. Percobaan gagal dibersihkan, bukan dianggap proof sukses.

Tidak ditemukan bug kode produksi dalam verifikasi ini. Tidak menjalankan ulang full suite/build/vet seluruh repo; build executable dan dua skenario proses di atas merupakan pemeriksaan sempit batch ini.

## Batas bukti

- Hanya Windows Ctrl+C/Ctrl+Break. **SIGTERM Linux belum diuji**, termasuk service manager/container atau hosting publik. FND-06 tetap BERJALAN untuk batas ini.
- Grace 10 detik tidak diuji sampai habis pada proses executable ini. Drain/forced-close melalui context tetap merujuk [tes lifecycle sebelumnya](server-foundation-2026-11-22.md), bukan diklaim sebagai injeksi sinyal baru.
- `Pool.Close()` tetap dapat menunggu pinjaman; hasil ini tidak menjamin batas total proses saat koneksi bocor atau dependency bermasalah.
- Bukan Firebase/Supabase/Android nyata dan tidak menerapkan migration remote.
