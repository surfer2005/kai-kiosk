package main

import (
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

// ensureLockdown sperrt die Openbox-Sitzung des NUC-Kiosks ab (2026-10-21, Live-Fund: ein Klick auf den
// freien Hintergrund öffnete das Openbox-Standardmenü mit „Terminal emulator", „Web browser" und „Exit" —
// damit hatte man am Terminal eine Shell und einen freien Browser). Wie alle System-Einstellungen hier
// beim Dienststart geschrieben, damit ein Selbst-Update auch ausgelieferte Geräte absichert.
//
//   - Openbox: leeres Hauptmenü, KEINE Tastenkürzel (Alt+F4, Alt+Tab, Arbeitsflächen …) und kein Menü
//     per Maus — nur „Klick fokussiert das Fenster" bleibt.
//   - X-Server: kein Wechsel auf die Textkonsolen (Strg+Alt+F1…F6) und kein Strg+Alt+Rücktaste.
//     Greift erst nach dem nächsten Neustart des Geräts.
//
// Fernwartung bleibt unberührt: SSH, VNC und die Konfig-Seite (:8091).
// Nur auf Geräten mit Openbox (NUC). Das Raspberry-Image nutzt einen anderen Desktop.
func ensureLockdown() {
	if _, err := os.Stat("/etc/xdg/openbox"); err != nil {
		return
	}
	changed := schreibeWennAnders("/etc/xdg/openbox/rc.xml", openboxRC, 0o644)
	changed = schreibeWennAnders("/etc/xdg/openbox/menu.xml", openboxMenu, 0o644) || changed

	// Eine eigene Openbox-Konfiguration im Benutzerverzeichnis hätte Vorrang — dort dieselbe Fassung.
	name := detectSSHUser()
	if u, err := user.Lookup(name); err == nil {
		dir := filepath.Join(u.HomeDir, ".config", "openbox")
		for _, f := range []string{"rc.xml", "menu.xml"} {
			p := filepath.Join(dir, f)
			if _, err := os.Stat(p); err == nil {
				inhalt := openboxRC
				if f == "menu.xml" {
					inhalt = openboxMenu
				}
				changed = schreibeWennAnders(p, inhalt, 0o644) || changed
			}
		}
	}

	if _, err := os.Stat("/etc/X11"); err == nil {
		_ = os.MkdirAll("/etc/X11/xorg.conf.d", 0o755)
		if schreibeWennAnders("/etc/X11/xorg.conf.d/10-kai-kiosk.conf", xorgKiosk, 0o644) {
			log.Printf("kiosk: Konsolenwechsel gesperrt — greift nach dem nächsten Neustart")
		}
	}

	if changed {
		log.Printf("kiosk: Openbox abgesichert (kein Menü, keine Tastenkürzel)")
		// Laufende Sitzung sofort übernehmen (best effort; sonst beim nächsten Start).
		cmd := exec.Command("su", "-s", "/bin/sh", name, "-c", "DISPLAY=:0 openbox --reconfigure")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("kiosk: openbox --reconfigure: %v (%s)", err, lastLine(out))
		}
	}
}

func schreibeWennAnders(pfad, inhalt string, mode os.FileMode) bool {
	if cur, err := os.ReadFile(pfad); err == nil && string(cur) == inhalt {
		return false
	}
	tmp := pfad + ".new"
	if err := os.WriteFile(tmp, []byte(inhalt), mode); err != nil {
		log.Printf("kiosk: %s nicht schreibbar: %v", pfad, err)
		return false
	}
	if err := os.Rename(tmp, pfad); err != nil {
		log.Printf("kiosk: %s nicht ersetzbar: %v", pfad, err)
		return false
	}
	return true
}

const openboxRC = `<?xml version="1.0" encoding="UTF-8"?>
<!-- KAI-Kiosk: von kai-kiosk geschrieben, Änderungen werden beim Dienststart überschrieben. -->
<openbox_config xmlns="http://openbox.org/3.4/rc">
  <focus><focusNew>yes</focusNew><followMouse>no</followMouse></focus>
  <desktops><number>1</number></desktops>
  <keyboard></keyboard>
  <mouse>
    <context name="Client">
      <mousebind button="Left" action="Press"><action name="Focus"/><action name="Raise"/></mousebind>
    </context>
  </mouse>
  <menu><file>/etc/xdg/openbox/menu.xml</file></menu>
</openbox_config>
`

const openboxMenu = `<?xml version="1.0" encoding="UTF-8"?>
<!-- KAI-Kiosk: absichtlich leer (kein Terminal, kein Browser, kein Beenden). -->
<openbox_menu xmlns="http://openbox.org/3.4/menu">
  <menu id="root-menu" label="KAI"></menu>
</openbox_menu>
`

const xorgKiosk = `# KAI-Kiosk: kein Wechsel auf Textkonsolen, kein Strg+Alt+Rücktaste (von kai-kiosk geschrieben)
Section "ServerFlags"
    Option "DontVTSwitch" "true"
    Option "DontZap" "true"
EndSection
`
