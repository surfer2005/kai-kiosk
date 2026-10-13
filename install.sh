#!/usr/bin/env bash
# KAI-Kiosk auf einem vorhandenen Raspberry Pi (OS Desktop) installieren:
# Konfig-Dienst + Browser-Starter + Autostart. Aufruf: sudo bash install.sh
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
PREFIX=/usr/local/bin
CFG=/etc/kai-kiosk

mkdir -p "$CFG"

# Binary: lokal gebautes ./kai-kiosk bevorzugt, sonst aus Quellcode bauen (Go nötig).
if [ -f "$HERE/kai-kiosk" ]; then
  install -m 0755 "$HERE/kai-kiosk" "$PREFIX/kai-kiosk"
elif command -v go >/dev/null 2>&1; then
  echo "Baue aus Quellcode …"
  ( cd "$HERE" && CGO_ENABLED=0 go build -o "$PREFIX/kai-kiosk" . )
else
  echo "FEHLER: weder ./kai-kiosk (Release) noch Go-Toolchain gefunden." >&2
  exit 1
fi

install -m 0755 "$HERE/kiosk-browser.sh" "$PREFIX/kiosk-browser.sh"

# Beispiel-Config nur anlegen, wenn noch keine da ist.
if [ ! -f "$CFG/config.json" ] && [ -f "$HERE/config.example.json" ]; then
  cp "$HERE/config.example.json" "$CFG/config.json"
fi

install -m 0644 "$HERE/systemd/kai-kiosk.service" /etc/systemd/system/kai-kiosk.service
install -Dm0644 "$HERE/autostart/kai-kiosk.desktop" /etc/xdg/autostart/kai-kiosk.desktop
systemctl daemon-reload
systemctl enable --now kai-kiosk

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Fertig. Konfig-Oberfläche:  http://${IP:-<pi>}:8091"
echo "Der Kiosk-Browser startet in der nächsten Desktop-Sitzung (Auto-Login nötig)."
