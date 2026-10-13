#!/usr/bin/env bash
# Startet Chromium im Vollbild-Kiosk auf die in der Config hinterlegte KAI-Adresse.
# Läuft in der Desktop-Sitzung des Auto-Login-Nutzers (per Autostart).
# WICHTIG: dauerhaftes Profil (--user-data-dir) → Service-Worker + IndexedDB der PWA bleiben über
# Neustarts erhalten, damit das Terminal OFFLINE genauso läuft wie online (Bordbuch-Prinzip).
set -u

CFG=/etc/kai-kiosk/config.json
URL=""
if [ -r "$CFG" ]; then
  URL=$(python3 -c "import json,sys;print(json.load(open('$CFG')).get('kai_url',''))" 2>/dev/null)
fi
# Keine Adresse gesetzt → lokale Konfig-Seite öffnen, damit man die Adresse eintragen kann
# (die echte KAI-Adresse steht bewusst nicht fest in der Quelle, s. README).
[ -n "$URL" ] || URL="http://localhost:8091"

# Chromium heißt je nach OS-Fassung "chromium-browser" oder "chromium".
BIN=chromium-browser
command -v "$BIN" >/dev/null 2>&1 || BIN=chromium

PROFILE="${HOME:-/home/pi}/.config/kai-kiosk-chromium"
mkdir -p "$PROFILE"

# Bildschirmschoner/Energiesparen aus (Terminal soll immer an sein).
command -v xset >/dev/null 2>&1 && { xset s off; xset -dpms; xset s noblank; } 2>/dev/null

# Absturz-/Wiederherstellungs-Dialoge der letzten Sitzung unterdrücken (sonst bleibt ein Banner stehen).
sed -i 's/"exited_cleanly":false/"exited_cleanly":true/; s/"exit_type":"[^"]*"/"exit_type":"Normal"/' \
  "$PROFILE/Default/Preferences" 2>/dev/null || true

exec "$BIN" \
  --kiosk --app="$URL" \
  --user-data-dir="$PROFILE" \
  --noerrdialogs --disable-infobars --disable-session-crashed-bubble \
  --disable-features=TranslateUI --no-first-run --fast --fast-start \
  --overscroll-history-navigation=0 --disable-pinch
