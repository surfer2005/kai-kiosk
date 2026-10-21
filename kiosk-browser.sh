#!/usr/bin/env bash
# Startet Chromium im Vollbild-Kiosk auf die in der Config hinterlegte KAI-Adresse.
# Läuft in der Desktop-Sitzung des Auto-Login-Nutzers (per Autostart).
# WICHTIG: dauerhaftes Profil (--user-data-dir) → Service-Worker + IndexedDB der PWA bleiben über
# Neustarts erhalten, damit das Terminal OFFLINE genauso läuft wie online (Bordbuch-Prinzip).
set -u

# Deutsche Sprache/Region: damit native Felder wie das Datumsfeld (<input type="date">) als TT.MM.JJJJ
# und mit Montag als Wochenanfang erscheinen — nicht im US-Format MM/DD/YYYY. Chromium folgt --lang (s. u.);
# LANG/LANGUAGE stellen ICU/Locale passend ein, sofern die Locale erzeugt ist (kiosk-setup legt sie an).
export LANG="${LANG:-de_DE.UTF-8}"
export LANGUAGE="${LANGUAGE:-de_DE:de}"
export LC_ALL="${LC_ALL:-de_DE.UTF-8}"
export LC_TIME="${LC_TIME:-de_DE.UTF-8}"
# Auf die deutsche Locale warten (der Dienst erzeugt sie beim Boot per ensureSystem; ohne sie bleiben
# die nativen Datums-/Zeitfelder im US-Format). Max. ~30 s, dann trotzdem starten (--lang greift auch so).
for _i in $(seq 1 30); do locale -a 2>/dev/null | grep -qiE 'de_DE\.utf-?8' && break; sleep 1; done

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
# Hintergrund der X-Sitzung SOFORT auf KAI-Papier (#F3F1EA) setzen, damit die Wartezeit bis Chromium
# NICHT schwarz ist, sondern nahtlos zum Boot-Splash + zur hellen KAI-Anmeldemaske passt. Das KAI-Logo
# wird gleich darunter (nach Start des lokalen Dienstes) zusätzlich mittig daraufgelegt (s. u.).
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
# Rechtschreibprüfung DEUTSCH: --lang stellt nur die Oberfläche um; die Prüfsprache steht im Profil und
# blieb seit dem ersten Start (vor dem Sprachpaket) auf en-US. Fehlt das Wörterbuch, lädt Chromium
# de-DE-*.bdic beim nächsten Online-Start einmal nach und legt es im (dauerhaften) Profil ab.
d.setdefault('spellcheck',{})
d['spellcheck']['dictionaries']=['de-DE']
d['spellcheck']['dictionary']=''
d['spellcheck']['use_spelling_service']=False
d.setdefault('browser',{})
d['browser']['enable_spellchecking']=True
d.setdefault('intl',{})
d['intl']['accept_languages']='de-DE,de'
d['intl']['selected_languages']='de-DE,de'
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

# KAI-Logo mittig auf das Papier legen — füllt die Lücke, die der /boot-Ladebildschirm NICHT abdecken
# kann: die Sekunden zwischen "X ist da" und "Chromium malt die erste Seite" (Chromium-Kaltstart).
# Ohne das sieht man dort nur leeres Sandpapier. Das Logo liefert der lokale Dienst offen unter
# /boot-logo.png; wir legen es in einen beschreibbaren Cache (der Starter läuft als Benutzer "kiosk"
# und darf nicht nach /usr/local schreiben) und setzen es per feh als Hintergrundbild. Bevorzugt ein
# schon systemweit installiertes Logo. Fehlt feh (Altinstallation), bleibt es beim reinen Papier.
LOGO="/usr/local/share/kai-kiosk/logo.png"
if [ ! -f "$LOGO" ]; then
  LOGO="${HOME:-/home/kiosk}/.cache/kai-kiosk/logo.png"
  mkdir -p "$(dirname "$LOGO")"
  command -v curl >/dev/null 2>&1 && \
    curl -fsS --max-time 5 -o "$LOGO" "http://127.0.0.1:8091/boot-logo.png" 2>/dev/null || true
fi
if command -v feh >/dev/null 2>&1 && [ -s "$LOGO" ]; then
  feh --no-fehbg --bg-center --image-bg "#F3F1EA" "$LOGO" 2>/dev/null || true
fi

# Chromium öffnet SOFORT den markierten KAI-Ladebildschirm des lokalen Dienstes (Logo + Spinner auf
# KAI-Papier). Diese Seite holt selbst die KAI-Adresse (/api/target), wartet per Erreichbarkeitsprüfung
# auf das Netz und leitet dann zu KAI weiter (bzw. zur Konfig-Seite, falls keine Adresse hinterlegt ist).
# So ist die Wartezeit bis zur Anmeldemaske nicht leeres Sandpapier, sondern KAI-Optik — und das frühere
# Netz-Warten mit schwarzem/leerem Bildschirm entfällt. Danach greift der Service-Worker (Offline).
URL="http://127.0.0.1:8091/boot"

# „Zurück zu KAI"-Erweiterung laden, wenn der Dienst sie hingelegt hat (s. refreshPortalExt). Sie blendet
# auf fremden Portal-Seiten unten links einen „⟵ KAI"-Knopf ein und löscht bei der Abmeldung die
# Portal-Anmeldungen. Fehlt das Verzeichnis (Altstand), startet der Kiosk unverändert ohne Erweiterung.
EXT_ARGS=()
EXT_DIR=/usr/local/share/kai-kiosk/portal-ext
if [ -f "$EXT_DIR/manifest.json" ]; then
  EXT_ARGS=(--load-extension="$EXT_DIR" --disable-extensions-except="$EXT_DIR")
fi

# Nicht per exec: schließt jemand das Kiosk-Fenster (Strg+W, Strg+Umschalt+Q, Absturz), stünde sonst
# der leere Desktop da. Stattdessen nach 2 s neu starten — wieder auf den Ladebildschirm.
while true; do
  "$BIN" \
    --kiosk --app="$URL" \
    --lang=de-DE \
    --user-data-dir="$PROFILE" \
    "${EXT_ARGS[@]}" \
    --noerrdialogs --disable-infobars --disable-session-crashed-bubble \
    --disable-features=TranslateUI --no-first-run --fast --fast-start \
    --overscroll-history-navigation=0 --disable-pinch
  sleep 2
done
