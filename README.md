# shttpd - Server HTTP Sederhana untuk Termux

`shttpd` adalah server file HTTP C++ ringan yang dirancang untuk berjalan di lingkungan Termux di Android. Server ini menyediakan antarmuka web untuk menjelajah, mengunggah, mengunduh, mengganti nama, memindahkan, dan menghapus file di perangkat Anda.

## Fitur

*   Backend C++ murni tanpa ketergantungan eksternal (selain compiler C++ dan utilitas `zip` standar).
*   Antarmuka web frontend yang responsif untuk manajemen file.
*   Mendukung operasi file dasar:
    *   Melihat daftar file dan folder.
    *   Mengunggah file (termasuk beberapa file dan unggahan folder seret & lepas).
    *   Mengunduh file.
    *   Membuat folder baru.
    *   Mengganti nama file dan folder.
    *   Memindahkan dan menyalin file/folder.
    *   Menghapus file dan folder (termasuk penghapusan rekursif).
    *   Membuat arsip ZIP dari file dan folder yang dipilih.
*   Dapat dikonfigurasi melalui argumen baris perintah untuk port dan direktori root.

## Kompilasi di Termux

Anda memerlukan compiler C++ seperti `clang++` atau `g++`. `clang++` biasanya sudah terinstal di Termux. Jika tidak, Anda dapat menginstalnya:

```bash
pkg install clang
```

Untuk mengkompilasi server, jalankan perintah berikut di direktori root proyek:

```bash
clang++ shttp.cpp -o shttp -std=c++17 -lstdc++fs
```

*   `shttp.cpp`: File sumber utama.
*   `-o shttp`: Menentukan nama file output yang dapat dieksekusi.
*   `-std=c++17`: Menggunakan standar C++17, yang diperlukan untuk fitur `std::filesystem`.
*   `-lstdc++fs`: Menautkan pustaka sistem file C++.

## Menjalankan Server

Setelah kompilasi berhasil, Anda akan memiliki file yang dapat dieksekusi bernama `shttp`. Jalankan dari direktori root proyek.

**Sintaks Dasar:**

```bash
./shttp [opsi]
```

### Opsi

*   `-p <port>`: Menjalankan server pada port tertentu. Jika tidak ditentukan, defaultnya adalah **8080**.
*   `-d </path/to/directory>`: Menyajikan file dari direktori tertentu. Jika tidak ditentukan, defaultnya adalah direktori kerja saat ini (`.`).

### Contoh

1.  **Menjalankan dengan pengaturan default (port 8080, direktori saat ini):**

    ```bash
    ./shttp
    ```

2.  **Menjalankan pada port 9000:**

    ```bash
    ./shttp -p 9000
    ```

3.  **Menyajikan file dari direktori `storage/downloads`:**

    ```bash
    ./shttp -d /data/data/com.termux/files/home/storage/downloads
    ```
    *(Catatan: Pastikan Anda menggunakan path absolut dan benar ke direktori penyimpanan Anda)*

4.  **Menjalankan pada port 8081 dan menyajikan direktori `documents`:**

    ```bash
    ./shttp -p 8081 -d ./documents
    ```

Setelah server berjalan, buka browser web di perangkat Anda (atau perangkat lain di jaringan yang sama) dan navigasikan ke `http://<ip-address-android-anda>:<port>`. Misalnya: `http://192.168.1.5:8080`.
