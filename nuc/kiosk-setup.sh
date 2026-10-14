#!/bin/bash
# Läuft am Ende der unbeaufsichtigten Installation IM Zielsystem (preseed late_command, in-target).
# Richtet den KAI-Kiosk ein: Programm + Dienst, Auto-Login in Openbox, Kiosk-Browser, VNC.
# Die Dateien liegen unter /opt/kai-kiosk-setup/ (vom Installationsmedium kopiert).
set +e
SRC=/opt/kai-kiosk-setup
PREFIX=/usr/local/bin
CFG=/etc/kai-kiosk
KIOSK_USER=kiosk

install -Dm0755 "$SRC/kai-kiosk"        "$PREFIX/kai-kiosk"
install -Dm0755 "$SRC/kiosk-browser.sh" "$PREFIX/kiosk-browser.sh"
install -Dm0644 "$SRC/kai-kiosk.service" /etc/systemd/system/kai-kiosk.service
mkdir -p "$CFG"
[ -f "$CFG/config.json" ] || install -Dm0600 "$SRC/config.json" "$CFG/config.json"

# Auto-Login in eine Openbox-Sitzung (startet den Kiosk-Browser per /etc/xdg/openbox/autostart).
mkdir -p /etc/lightdm/lightdm.conf.d
cat >/etc/lightdm/lightdm.conf.d/50-kai-kiosk.conf <<EOF
[Seat:*]
autologin-user=$KIOSK_USER
autologin-user-timeout=0
autologin-session=openbox
user-session=openbox
EOF
# ⚠️ Debian/LightDM lässt den Auto-Login NUR zu, wenn der Benutzer in der Gruppe 'autologin' ist
# (PAM: /etc/pam.d/lightdm-autologin → pam_succeed_if user ingroup autologin). Fehlt die Gruppe,
# bleibt der NUC-Bildschirm am grauen Login-Prompt stehen und Chromium (die KAI-PWA) startet nie —
# obwohl die Konfig-Seite (:8091) über LAN weiter erreichbar ist.
groupadd -f autologin
usermod -aG autologin "$KIOSK_USER"
mkdir -p /etc/xdg/openbox
cat >/etc/xdg/openbox/autostart <<EOF
# KAI-Kiosk: Chromium im Vollbild starten
/usr/local/bin/kiosk-browser.sh &
EOF

# VNC-Fernwartung (ohne Passwort-UI-Integration; Passwortdatei optional unter /etc/kai-kiosk/vncpasswd).
cat >/etc/systemd/system/x11vnc.service <<EOF
[Unit]
Description=x11vnc (Fernwartung KAI-Kiosk)
After=display-manager.service
Wants=display-manager.service
[Service]
ExecStart=/bin/sh -c 'if [ -f /etc/kai-kiosk/vncpasswd ]; then exec x11vnc -display :0 -auth guess -forever -loop -noxdamage -repeat -rfbauth /etc/kai-kiosk/vncpasswd -rfbport 5900 -shared; else exec x11vnc -display :0 -auth guess -forever -loop -noxdamage -repeat -nopw -rfbport 5900 -shared; fi'
Restart=on-failure
RestartSec=3
[Install]
WantedBy=multi-user.target
EOF

ln -sf /usr/share/zoneinfo/Europe/Berlin /etc/localtime
echo "Europe/Berlin" >/etc/timezone

systemctl enable ssh 2>/dev/null || systemctl enable sshd 2>/dev/null
systemctl enable x11vnc.service 2>/dev/null
systemctl enable kai-kiosk.service 2>/dev/null
systemctl set-default graphical.target 2>/dev/null

# Sauberer Boot: grafischer Splash mit Spinner („Kringel") statt Textausgabe + GRUB-Menü (Nutzerwunsch).
command -v plymouth-set-default-theme >/dev/null 2>&1 || apt-get -y install plymouth plymouth-themes 2>/dev/null
plymouth-set-default-theme spinner 2>/dev/null || true
if [ -f /etc/default/grub ]; then
  sed -i 's/^GRUB_TIMEOUT=.*/GRUB_TIMEOUT=0/' /etc/default/grub || true
  if grep -q '^GRUB_TIMEOUT_STYLE=' /etc/default/grub; then
    sed -i 's/^GRUB_TIMEOUT_STYLE=.*/GRUB_TIMEOUT_STYLE=hidden/' /etc/default/grub
  else
    echo 'GRUB_TIMEOUT_STYLE=hidden' >>/etc/default/grub
  fi
  # still + Splash + kein blinkender Text-Cursor; Logo/Hersteller-Meldungen aus.
  if grep -q '^GRUB_CMDLINE_LINUX_DEFAULT=' /etc/default/grub; then
    sed -i 's/^GRUB_CMDLINE_LINUX_DEFAULT=.*/GRUB_CMDLINE_LINUX_DEFAULT="quiet splash loglevel=3 vt.global_cursor_default=0 plymouth.ignore-serial-consoles"/' /etc/default/grub
  else
    echo 'GRUB_CMDLINE_LINUX_DEFAULT="quiet splash loglevel=3 vt.global_cursor_default=0 plymouth.ignore-serial-consoles"' >>/etc/default/grub
  fi
  update-grub 2>/dev/null || grub-mkconfig -o /boot/grub/grub.cfg 2>/dev/null || true
fi
update-initramfs -u 2>/dev/null || true
exit 0
