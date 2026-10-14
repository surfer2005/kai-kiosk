#!/usr/bin/env bash
# KAI-Kiosk auf einem Intel-NUC (x86/amd64) mit frisch installiertem Debian 12 / Ubuntu einrichten.
# Macht aus einem Standard-Linux ein KAI-Terminal: minimaler Desktop (Openbox) mit Auto-Login, der
# Chromium im Vollbild auf die KAI-Adresse startet, dazu SSH + VNC (x11vnc) für die Fernwartung und
# den KAI-Kiosk-Konfigdienst auf Port 8091 — identisch zur Raspberry-Pi-Fassung.
#
# Aufruf auf dem NUC:   sudo bash install-nuc.sh
# (Holt das amd64-Programm aus dem Release, wenn ./kai-kiosk nicht daneben liegt.)
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then echo "Bitte mit sudo starten: sudo bash install-nuc.sh" >&2; exit 1; fi
HERE="$(cd "$(dirname "$0")" && pwd)"
PREFIX=/usr/local/bin
CFG=/etc/kai-kiosk
KIOSK_USER="${KIOSK_USER:-kiosk}"
RELEASE_BIN="https://github.com/surfer2005/kai-kiosk/releases/download/v0.1.0/kai-kiosk-amd64"

echo ">> Pakete installieren …"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
# Minimaler Grafik-Stack + Chromium + Fernwartung. chromium (Debian) bzw. chromium-browser (Ubuntu).
apt-get install -y --no-install-recommends \
  xserver-xorg xinit openbox lightdm \
  x11vnc openssh-server network-manager \
  unclutter curl ca-certificates python3 \
  || true
# Browser: Debian hat 'chromium', Ubuntu 'chromium-browser' (teils Snap). Beides versuchen.
apt-get install -y chromium 2>/dev/null || apt-get install -y chromium-browser 2>/dev/null || true

echo ">> Kiosk-Benutzer '$KIOSK_USER' anlegen (falls nötig) …"
if ! id "$KIOSK_USER" >/dev/null 2>&1; then
  useradd -m -s /bin/bash "$KIOSK_USER"
  echo "$KIOSK_USER:kai-kiosk" | chpasswd
fi

echo ">> KAI-Kiosk-Programm + Dateien installieren …"
if [ -f "$HERE/kai-kiosk" ]; then
  install -m 0755 "$HERE/kai-kiosk" "$PREFIX/kai-kiosk"
else
  echo "   ./kai-kiosk nicht gefunden — amd64-Programm aus dem Release laden …"
  curl -fL --retry 3 -o /tmp/kai-kiosk "$RELEASE_BIN"
  install -m 0755 /tmp/kai-kiosk "$PREFIX/kai-kiosk"
fi
install -m 0755 "$HERE/kiosk-browser.sh" "$PREFIX/kiosk-browser.sh"
install -Dm644 "$HERE/systemd/kai-kiosk.service" /etc/systemd/system/kai-kiosk.service
mkdir -p "$CFG"
[ -f "$CFG/config.json" ] || cp "$HERE/config.example.json" "$CFG/config.json"

echo ">> Auto-Login in eine Openbox-Sitzung …"
mkdir -p /etc/lightdm/lightdm.conf.d
cat >/etc/lightdm/lightdm.conf.d/50-kai-kiosk.conf <<EOF
[Seat:*]
autologin-user=$KIOSK_USER
autologin-user-timeout=0
autologin-session=openbox
user-session=openbox
EOF
# Openbox startet beim Anmelden den Kiosk-Browser (Openbox liest /etc/xdg/openbox/autostart).
mkdir -p /etc/xdg/openbox
cat >/etc/xdg/openbox/autostart <<EOF
# KAI-Kiosk: Chromium im Vollbild starten
/usr/local/bin/kiosk-browser.sh &
EOF

echo ">> VNC (x11vnc) für die Fernwartung …"
cat >/etc/systemd/system/x11vnc.service <<EOF
[Unit]
Description=x11vnc (Fernwartung KAI-Kiosk)
After=display-manager.service
Wants=display-manager.service
[Service]
# Passwortschutz: wenn /etc/kai-kiosk/vncpasswd existiert, wird es genutzt, sonst offen im LAN.
ExecStart=/bin/sh -c 'if [ -f /etc/kai-kiosk/vncpasswd ]; then exec x11vnc -display :0 -auth guess -forever -loop -noxdamage -repeat -rfbauth /etc/kai-kiosk/vncpasswd -rfbport 5900 -shared; else exec x11vnc -display :0 -auth guess -forever -loop -noxdamage -repeat -nopw -rfbport 5900 -shared; fi'
Restart=on-failure
RestartSec=3
[Install]
WantedBy=multi-user.target
EOF

echo ">> Zeitzone + Dienste aktivieren …"
ln -sf /usr/share/zoneinfo/Europe/Berlin /etc/localtime
echo "Europe/Berlin" >/etc/timezone
systemctl daemon-reload
systemctl enable ssh || systemctl enable sshd || true
systemctl enable x11vnc.service || true
systemctl enable kai-kiosk.service
systemctl set-default graphical.target || true

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Fertig. Nach einem Neustart startet das Terminal automatisch im Vollbild."
echo "  Konfig-Oberfläche:  http://${IP:-<nuc>}:8091   (KAI-Adresse / feste IP eintragen)"
echo "  SSH:                ssh $KIOSK_USER@${IP:-<nuc>}   (Passwort: kai-kiosk — bitte ändern)"
echo "  VNC:                ${IP:-<nuc>}:5900"
echo
echo "Jetzt neu starten:  sudo reboot"
