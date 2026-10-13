# KAI-Kiosk — Terminal-Image für KAI (Raspberry Pi / NUC)

Ein reproduzierbares Image, das einen Raspberry Pi (oder NUC) zu einem **KAI-Terminal** macht:
beim Start **Auto-Login → Chromium im Vollbild-Kiosk** auf die KAI-PWA (Anmeldemaske sofort sichtbar).
Mit **SSH + VNC** (Fernwartung), **GitHub-Selbst-Update** und **fester IP** — alles über eine kleine
lokale Konfig-Seite einstellbar. Gleiche Machart wie das Bordwächter-Gerät, nur ohne GPS/SIFA/GPIO.

> Nur Go-Standardbibliothek → ein statisches Binary, wartungsarm.

## PWA / Offline
Der Kiosk-Browser nutzt ein **dauerhaftes Chromium-Profil** (`--user-data-dir`), damit
**Service-Worker + IndexedDB** der KAI-PWA über Neustarts erhalten bleiben. Einmal online geladen,
läuft das Terminal **offline genauso wie das Bordbuch** (App-Shell aus dem Cache, Offline-Login,
Schreib-Warteschlange). Voraussetzung ist ein fester Origin — läuft die Instanz unter einem
Unterpfad (z. B. `…/test/`), muss dessen Service-Worker-Sperrliste gesetzt sein.

## Sicherheit: die KAI-Adresse steht nicht im Repo
Dieses Repo ist öffentlich — die interne Terminal-Adresse (und erst recht ein Test-Pfad) wird
deshalb **nicht** fest hinterlegt. Die Adresse wird **pro Terminal** gesetzt:
- in der **Konfig-Seite** (`http://<ip>:8091`), oder
- vorab per **`kai-kiosk.json`** auf der Boot-Partition (Flotten-Rollout), oder
- beim **Image-Bau** per `workflow_dispatch`-Eingabe/Repo-Secret (falls man sie ins Image backen will).

Ist keine Adresse gesetzt, öffnet der Kiosk automatisch die lokale Konfig-Seite zum Eintragen.

## Konfig-Oberfläche (`http://<ip>:8091`)
Komfortabel statt JSON-Tippen — mit Anmeldung (falls Passwort gesetzt):
- **KAI-Adresse** (pro Terminal einzutragen — steht bewusst **nicht** in dieser öffentlichen Quelle)
- **Netzwerk** — DHCP oder **feste IP** (Adresse/Gateway/DNS)
- **VNC-Passwort**, **Konfig-Passwort**, **Geräte-Token**
- **Software** — „Nach Update suchen" / „Jetzt aktualisieren & neu starten" (zieht `releases/latest`)
- **Gerät** — Dienst- bzw. Gerät-Neustart

Für den Flotten-Rollout ohne Tastatur: eine Datei **`kai-kiosk.json`** auf die Boot-Partition legen
(gleiches Format wie `config.example.json`) — wird beim ersten Start übernommen und die feste IP
sofort angewandt.

## Der Geräte-Token
Der **Geräte-Token** markiert dieses Terminal bei KAI als **vertrauenswürdiges Gerät**: von einem
registrierten Terminal reicht dann **Benutzer + Passwort** (kein zweiter Faktor), während unbekannte
Geräte/Netze **2FA** brauchen. Vorteil gegenüber „per IP vertrauen": der Token hängt am konkreten
Gerät und lässt sich einzeln widerrufen.
⚠️ Diese Token-/2FA-Auth **gibt es in KAI noch nicht** — das Feld ist vorbereitet und greift, sobald
die Auth serverseitig gebaut ist. Der Kiosk funktioniert auch ohne (zeigt die normale Anmeldemaske).

## Bauen
```bash
cd edge/kai-kiosk
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o kai-kiosk .   # für Raspberry Pi (64-bit)
```

## Installieren (auf vorhandenem Pi, OS Desktop)
```bash
sudo bash install.sh
```

## Image (GitHub Actions)
- `.github/workflows/image.yml` nimmt das offizielle RPi-OS-Desktop-Image, injiziert Dienst +
  Browser-Starter + Autostart + Firstrun (SSH/VNC an, Desktop-Autologin, Hostname `kai-kiosk`,
  TZ Europe/Berlin). Release per Tag `img-*` → `kai-kiosk-<datum>.img.xz`. Login `pi` / `kai-kiosk`.
- `.github/workflows/build.yml` baut die Binaries (arm64/amd64/armv7), Release per Tag `v*`.

⚠️ **Image-Release NICHT „Latest":** der „Aktualisieren"-Knopf zieht
`releases/latest/download/kai-kiosk-<arch>` — in einem Image-Release liegt kein Binary (→ 404).
Nach einem `img-*`-Tag daher das jüngste **Binary**-Release mit `gh release edit vX --latest`
als „Latest" setzen (gleiche Falle wie beim Bordwächter-Gerät).

## Quelle
Dieses Verzeichnis ist ein eigenständiges Go-Modul und wird in das GitHub-Repo
`surfer2005/kai-kiosk` gespiegelt (Releases + Image dort).
