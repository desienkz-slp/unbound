# Walkthrough - Fix unbound-checkconf gagal (install.sh + engine Go)

## Gejala
Saat install di fresh Ubuntu, unbound-checkconf gagal pada /etc/unbound/unbound.conf:
  error: unknown keyword ''serve-expired-client-timeout''
  error: stray '':''
  error: unknown keyword ''1800''

## Akar Masalah
serve-expired-client-timeout dan serve-expired-reply-ttl baru ada di Unbound >= 1.9.0.
Fresh Ubuntu lama (18.04 -> unbound 1.6.7) menolak keyword tsb. Keyword tak dikenal
dipecah parser jadi token terpisah -> 3 error dari 1 baris.

## Perbaikan (2 lapis, kompatibel semua versi)
### 1. install.sh (bootstrap config)
- Deteksi versi Unbound (unbound-checkconf -h / fallback unbound -V) + helper version_ge (sort -V).
- Emit serve-expired-client-timeout HANYA jika versi >= 1.9.0 via SERVE_EXPIRED_CLIENT_TIMEOUT_DIRECTIVE.

### 2. pkg/configengine (engine Go runtime)
- generator.go: tambah unboundSupportsServeExpiredAdvanced() -> deteksi `unbound -V`,
  cache via sync.Once, return major>1 || (major==1 && minor>=9). Gagal deteksi -> false (aman).
- GeneratorContext.SupportsServeExpiredAdvanced di-set saat build context.
- templates.go: serve-expired-client-timeout & serve-expired-reply-ttl digate
  {{- if .SupportsServeExpiredAdvanced }}.
- generator_test.go: prime cache (serveExpiredAdvancedOK=true) agar test deterministik
  tanpa bergantung apakah unbound terpasang di host.

## Verifikasi
- bash -n install.sh: OK; line endings LF murni; no BOM.
- go build ./...: OK
- go vet ./pkg/configengine: OK
- go test ./...: 14 paket OK (semua lulus)
- gofmt: 3 file yang diedit sudah bersih (validator.go pre-existing unformatted, tidak disentuh).

## Hasil
Config aman dirender di Unbound versi berapa pun (lama maupun >= 1.9.0),
baik saat install awal maupun saat daemon regenerate config di kemudian hari.
