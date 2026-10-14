#!/usr/bin/env bash
# Startet Chromium im Vollbild-Kiosk auf die in der Config hinterlegte KAI-Adresse.
# Läuft in der Desktop-Sitzung des Auto-Login-Nutzers (per Autostart).
# WICHTIG: dauerhaftes Profil (--user-data-dir) → Service-Worker + IndexedDB der PWA bleiben über
# Neustarts erhalten, damit das Terminal OFFLINE genauso läuft wie online (Bordbuch-Prinzip).
set -u

CFG=/etc/kai-kiosk/config.json
URL=""
# Die KAI-Adresse wird WEITER UNTEN über den lokalen Dienst (/api/target) geholt — NICHT direkt aus
# config.json. Grund: die config.json gehört root (0600), der als "kiosk" laufende Browser-Starter darf
# sie nicht lesen (→ Adresse leer → fälschlich Konfig-Seite). Der als root laufende Dienst liefert die
# (unkritische) Adresse offen. Datei-Lesen bleibt nur als Rückfall, falls der Dienst sie mal nicht hat.

# Chromium heißt je nach OS-Fassung "chromium-browser" oder "chromium".
BIN=chromium-browser
command -v "$BIN" >/dev/null 2>&1 || BIN=chromium

PROFILE="${HOME:-/home/pi}/.config/kai-kiosk-chromium"
mkdir -p "$PROFILE"

# Bildschirmschoner/Energiesparen aus (Terminal soll immer an sein).
command -v xset >/dev/null 2>&1 && { xset s off; xset -dpms; xset s noblank; } 2>/dev/null
# Hintergrund der X-Sitzung auf KAI-Papier (#F3F1EA) setzen, damit die Wartezeit bis Chromium NICHT
# schwarz ist, sondern nahtlos zum Boot-Splash + zur hellen KAI-Anmeldemaske passt.
command -v xsetroot >/dev/null 2>&1 && xsetroot -solid "#F3F1EA" 2>/dev/null || true

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

# IMMER zuerst auf den lokalen Konfig-Dienst warten (kommt beim Boot in Sekunden). Sonst öffnet
# Chromium localhost, bevor der Dienst lauscht → "Diese Seite ist nicht erreichbar / localhost".
if command -v curl >/dev/null 2>&1; then
  for i in $(seq 1 60); do
    curl -fsS --max-time 2 -o /dev/null "http://127.0.0.1:8091/api/needs-login" 2>/dev/null && break
    sleep 1
  done
fi

# Chromium öffnet SOFORT den markierten KAI-Ladebildschirm des lokalen Dienstes (Logo + Spinner auf
# KAI-Papier). Diese Seite holt selbst die KAI-Adresse (/api/target), wartet per Erreichbarkeitsprüfung
# auf das Netz und leitet dann zu KAI weiter (bzw. zur Konfig-Seite, falls keine Adresse hinterlegt ist).
# So ist die Wartezeit bis zur Anmeldemaske nicht leeres Sandpapier, sondern KAI-Optik — und das frühere
# Netz-Warten mit schwarzem/leerem Bildschirm entfällt. Danach greift der Service-Worker (Offline).
URL="http://127.0.0.1:8091/boot"

exec "$BIN" \
  --kiosk --app="$URL" \
  --user-data-dir="$PROFILE" \
  --noerrdialogs --disable-infobars --disable-session-crashed-bubble \
  --disable-features=TranslateUI --no-first-run --fast --fast-start \
  --overscroll-history-navigation=0 --disable-pinch
