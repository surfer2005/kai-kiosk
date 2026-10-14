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

# Beim Booten ist das LAN/DNS oft noch nicht da → Chromium zeigte sonst eine Fehlerseite und blieb
# darauf hängen. Deshalb auf die Erreichbarkeit der KAI-Adresse warten (max. ~60 s). Für die lokale
# Konfig-Seite wird nicht gewartet. Bleibt die KAI-Adresse unerreichbar, zeigen wir die Konfig-Seite
# (dort steht die aktuelle IP) statt einer Browser-Fehlerseite.
case "$URL" in
  http://localhost*|http://127.*) : ;;
  *)
    if command -v curl >/dev/null 2>&1; then
      ok=0; deadline=$(( $(date +%s) + 60 ))
      while :; do
        if curl -fsSL --max-time 4 -o /dev/null "$URL" 2>/dev/null; then ok=1; break; fi
        [ "$(date +%s)" -ge "$deadline" ] && break
        sleep 2
      done
      [ "$ok" = 1 ] || URL="http://localhost:8091"
    fi
  ;;
esac

exec "$BIN" \
  --kiosk --app="$URL" \
  --user-data-dir="$PROFILE" \
  --noerrdialogs --disable-infobars --disable-session-crashed-bubble \
  --disable-features=TranslateUI --no-first-run --fast --fast-start \
  --overscroll-history-navigation=0 --disable-pinch
