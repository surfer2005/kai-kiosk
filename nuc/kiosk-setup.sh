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
exit 0
