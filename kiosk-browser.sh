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
# Hinweis: ist keine Adresse gesetzt, wird weiter unten (nach dem Warten auf den lokalen Dienst)
# die Konfig-Seite geöffnet, damit man die Adresse eintragen kann.

# Chromium heißt je nach OS-Fassung "chromium-browser" oder "chromium".
BIN=chromium-browser
command -v "$BIN" >/dev/null 2>&1 || BIN=chromium

PROFILE="${HOME:-/home/pi}/.config/kai-kiosk-chromium"
mkdir -p "$PROFILE"

# Bildschirmschoner/Energiesparen aus (Terminal soll immer an sein).
command -v xset >/dev/null 2>&1 && { xset s off; xset -dpms; xset s noblank; } 2>/dev/null

# Absturz-/Wiederherstellungs-Dialoge der letzten Sitzung unterdrücken (sonst bleibt ein Banner stehen)
# UND Chromium-Passwortmanager/„Passwort speichern?"-Abfrage am Terminal aus.
mkdir -p "$PROFILE/Default"
python3 - "$PROFILE/Default/Preferences" <<'PY' 2>/dev/null || true
import json,sys
p=sys.argv[1]
try: d=json.load(open(p))
except Exception: d={}
d.setdefault('profile',{})
d['profile']['exit_type']='Normal'
d['profile']['exited_cleanly']=True
d['profile']['password_manager_enabled']=False
d.setdefault('credentials_enable_service',False)
d['credentials_enable_service']=False
d.setdefault('credentials_enable_autosignin',False)
d['credentials_enable_autosignin']=False
json.dump(d,open(p,'w'))
PY

# 1) IMMER zuerst auf den lokalen Konfig-Dienst warten (kommt beim Boot in Sekunden). Sonst öffnet
#    Chromium localhost, bevor der Dienst lauscht → "Diese Seite ist nicht erreichbar / localhost".
if command -v curl >/dev/null 2>&1; then
  for i in $(seq 1 60); do
    curl -fsS --max-time 2 -o /dev/null "http://127.0.0.1:8091/api/needs-login" 2>/dev/null && break
    sleep 1
  done
fi

# 2) Keine KAI-Adresse hinterlegt → Konfig-Seite (dort Adresse eintragen). Sonst KAI DIREKT öffnen
#    (der Service-Worker macht das Terminal offline-fähig). Dem Netz beim Boot kurz Zeit geben, aber
#    NICHT auf localhost zurückfallen — das würde den Offline-Betrieb brechen und fälschlich die
#    Konfig-Seite zeigen. Ist beim allerersten Start kein Netz da, zeigt Chromium kurz einen Fehler
#    und lädt KAI, sobald das Netz steht (danach greift der Offline-Cache).
if [ -z "$URL" ]; then
  URL="http://localhost:8091"
elif command -v curl >/dev/null 2>&1; then
  for i in $(seq 1 20); do
    curl -fsSL --max-time 3 -o /dev/null "$URL" 2>/dev/null && break
    sleep 1
  done
fi

exec "$BIN" \
  --kiosk --app="$URL" \
  --user-data-dir="$PROFILE" \
  --noerrdialogs --disable-infobars --disable-session-crashed-bubble \
  --disable-features=TranslateUI --no-first-run --fast --fast-start \
  --overscroll-history-navigation=0 --disable-pinch
