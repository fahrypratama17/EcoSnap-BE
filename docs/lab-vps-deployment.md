# VPS lab UB: Nginx dan autentikasi AI

Target saat ini: Ubuntu 26.04 x86-64, akun `member` dengan sudo, repo `/opt/ecosnap`, IP privat `10.34.211.188`. Go dan AI sudah berjalan sebagai `ecosnap-api`/`ecosnap-ai` via systemd di loopback 8080/8000. Panduan ini tidak menjalankan migration atau seed dan tidak mengubah database bersama.

## Batas demo

HTTP pada port 80 dipilih pemilik untuk demo **internal UB sementara**. Firebase Bearer tetap diverifikasi, tetapi **HTTP tidak mengenkripsi token/foto**; orang dengan akses jaringan dapat menyadapnya. Jangan memakai data sensitif atau membuka akses internet/port forwarding publik. HTTPS dengan sertifikat yang dipercaya Android wajib sebelum penggunaan publik. Tidak ada bypass autentikasi AI, shared secret di APK, atau key Groq di FE.

SSH/tunnel dan USB tidak diperlukan oleh aplikasi setelah HP dapat menjangkau port 80 VPS secara langsung. IP privat bukan jaminan akses: pengelola lab harus mengizinkan rute dari Wi-Fi UB yang dipakai HP. Nginx tidak menembus isolasi Wi-Fi, VLAN, ACL kampus, atau firewall hypervisor.

## 1. Perbarui source dan dependency di terminal VPS

```bash
cd /opt/ecosnap
git switch dev
git pull --ff-only origin dev
ai-service/.venv/bin/python -m pip install -r ai-service/requirements.txt
```

Jika Git melaporkan perubahan lokal/konflik, berhenti; jangan reset/clean. `.env` dan service-account JSON tetap lokal. Go binary tidak berubah oleh paket autentikasi AI ini; tidak perlu build/upload Go ulang jika binary sebelumnya sudah memuat dotenv dan `/health` bekerja.

Pastikan `/opt/ecosnap/.env` tetap berisi `HTTP_ADDR=127.0.0.1:8080`, DATABASE_URL asli, `GOOGLE_APPLICATION_CREDENTIALS="/opt/ecosnap/firebase-admin.json"`, dan `GROQ_API_KEY` baru. File Firebase harus berasal dari **project yang sama dengan login Android**. Jangan mengirim isi file ke chat/log/Git. Perbarui lewat `nano /opt/ecosnap/.env`, bukan perintah yang memasukkan credential ke history.

```bash
chmod 600 /opt/ecosnap/.env /opt/ecosnap/firebase-admin.json
sudo systemctl restart ecosnap-ai
sudo systemctl status ecosnap-ai --no-pager
```

AI memakai Firebase Admin untuk signature/expiry/audience/issuer ID token, bukan sekadar decode JWT. Tidak mengecek revocation pada setiap scan (sama seperti verifikasi token Go normal); masa berlaku ID token tetap berlaku. UID terverifikasi memisahkan cache idempotensi dan kuota per akun; IP proxy yang sama tidak menyatukan hasil pengguna. Tidak ada pemeriksaan profil/role PostgreSQL pada scan ini: akun Firebase valid dapat melakukan scan. Satu worker saja; cache/limiter belum dibagi antar replika.

## 2. Pasang Nginx

```bash
sudo apt update
sudo apt install -y nginx
sudo install -m 644 /opt/ecosnap/deploy/nginx/ecosnap-lab.conf /etc/nginx/sites-available/ecosnap-lab
sudo ln -sfn /etc/nginx/sites-available/ecosnap-lab /etc/nginx/sites-enabled/ecosnap-lab
sudo nginx -t
```

**Hanya jika `nginx -t` sukses**, aktifkan dan reload:

```bash
sudo systemctl enable --now nginx
sudo systemctl reload nginx
```

Tidak perlu menghapus site lain atau mematikan firewall. Jika port 80 sudah dimiliki layanan lain, atau ada `conflicting server name`, jangan menimpa konfigurasinya: koordinasikan dengan pengelola lab.

## 3. Firewall

```bash
sudo ufw status
```

Jika UFW aktif dan belum mengizinkan HTTP, minta rentang/subnet klien yang tepat kepada admin lab, lalu izinkan **hanya subnet itu**:

```bash
# Ganti SUBNET_KLIEN_UB dengan CIDR yang diberikan admin; jangan jalankan placeholder.
sudo ufw allow from SUBNET_KLIEN_UB to any port 80 proto tcp
```

Jangan `ufw disable`, jangan buka 8000/8080 ke jaringan, dan jangan `ufw enable` secara membabi buta karena bisa memutus SSH. Firewall lab/hypervisor juga mungkin perlu izin admin. Nginx meneruskan ke service loopback; tidak mengubah bind Go/FastAPI menjadi 0.0.0.0.

## 4. Verifikasi di VPS

```bash
curl --fail --max-time 10 http://10.34.211.188/health
curl --fail --max-time 10 http://10.34.211.188/ai/health
curl -i --max-time 15 -X POST http://10.34.211.188/api/scan
```

Dua health harus `200 {"status":"ok"}`. Scan tanpa token harus **401 `unauthorized`**, bukan 415/missing_image atau inferensi Groq. Ini menguji penolakan akses, bukan scan sukses.

Nginx routing (semua path dipertahankan kecuali alias health):

| Path | Tujuan |
|---|---|
| `/api/v1/*`, `/health` | Go `127.0.0.1:8080` |
| `/api/scan`, `/docs`, `/openapi.json`, `/redoc` | AI `127.0.0.1:8000` |
| `/ai/health` | AI `/health` |

Batas body 10 MiB + 64 KiB, buffering request nonaktif, upstream timeout baca 120 detik, tidak ada retry upstream otomatis. Error yang dibuat Nginx sendiri (mis. 413/502/504) bisa HTML, bukan envelope JSON aplikasi; klien harus menangani kegagalan transport/proxy dengan aman. Limit/budget AI tidak dihilangkan. Log akses Nginx dimatikan, tidak ada header Authorization yang dicatat.

## 5. Tes dari HP di Wi-Fi UB (tanpa laptop/tunnel)

Buka browser HP: `http://10.34.211.188/health` dan `http://10.34.211.188/ai/health`. Jika tidak bisa dibuka tetapi curl VPS sukses, masalahnya jaringan/firewall/routing lab; mengubah APK tidak akan memperbaikinya.

Setelah kedua URL bisa dibuka, di laptop **EcoSnap-FE/local.properties**:

```properties
API_BASE_URL=http://10.34.211.188/
AI_SCAN_BASE_URL=http://10.34.211.188/
```

Build/Run ulang **debug APK terbaru** (bukan APK lama yang belum mengirim token). Kedua properti adalah origin tanpa `/api/v1` atau `/api/scan`. Debug cleartext hanya diizinkan untuk host yang dikonfigurasi; release tetap menuntut HTTPS. Login Firebase seperti biasa, lalu scan. Klien mengambil ID token dan mengirim Bearer ke AI; satu refresh pada 401, tidak mengirim Groq key. Logout/login jika token tidak dapat diperbarui. Pastikan storage HP cukup saat install; sebelumnya terjadi `INSTALL_FAILED_INSUFFICIENT_STORAGE`.

Di Swagger `http://10.34.211.188/docs`, **Authorize** menerima Firebase ID token, bukan Groq key, password akun, atau custom token. Jangan menyalin token ke chat/screenshot. Untuk pengguna aplikasi, lebih mudah tes lewat login Android agar SDK yang menangani token.

## Operasi dan diagnosis

Server tetap berjalan ketika SSH ditutup; setelah deploy langsung, tunnel dan adb reverse tidak dibutuhkan.

```bash
sudo systemctl status nginx ecosnap-api ecosnap-ai --no-pager
sudo journalctl -u ecosnap-ai -n 40 --no-pager
sudo journalctl -u ecosnap-api -n 40 --no-pager
sudo tail -n 30 /var/log/nginx/error.log
```

- `401 unauthorized`: login/token/project tidak cocok atau token kedaluwarsa; belum memanggil Groq.
- `503 auth_unavailable`: konfigurasi Firebase, akses sertifikat Google atau verifikasi tidak tersedia; bukan bukti key Groq salah.
- `503 missing_api_key`: isi key Groq baru di `.env`, restart AI.
- `429`: ikuti Retry-After; jangan spam scan.
- `504 provider_timeout`: jalur pemrosesan/provider timeout; jangan langsung menaikkan semua deadline.
- `502` dari Nginx: cek listener/service AI/Go sebelum mengubah FE.

Health tidak menguji key, kuota, kualitas model, akses DB runtime per endpoint, atau Firebase token nyata. Bukti deployment VPS dan scan HP harus berasal dari hasil perintah/perangkat pengguna, bukan tes lokal sintetis.
