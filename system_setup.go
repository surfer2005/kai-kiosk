package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

// ensureSystem zieht beim Dienststart die System-Einstellungen nach, die sonst NUR bei der
// Installation gesetzt würden (Zeitzone + deutsche Locale). Grund/Lehre (wie beim Bordwächter-Keyring):
// ein Selbst-Update tauscht nur die Binärdatei — die nuc/kiosk-setup.sh läuft dabei NICHT erneut.
// Ein vor dem Locale-Fix aufgesetzter NUC bliebe sonst dauerhaft auf US-Datum/UTC-Zeit, obwohl die
// neue Version es längst könnte. Alles best effort, idempotent, nur bei Bedarf (läuft als root).
func ensureSystem() {
	ensureTimezone()
	ensureGermanLocale()
}

// ensureTimezone setzt Europe/Berlin, falls noch nicht gesetzt (sonst zeigt die Uhr im Kiosk UTC).
func ensureTimezone() {
	const tz = "Europe/Berlin"
	cur, _ := os.ReadFile("/etc/timezone")
	if strings.TrimSpace(string(cur)) == tz {
		return
	}
	if _, err := exec.LookPath("timedatectl"); err == nil {
		if err := exec.Command("timedatectl", "set-timezone", tz).Run(); err == nil {
			log.Printf("system: Zeitzone auf %s gesetzt", tz)
			return
		}
	}
	// Rückfall ohne systemd-Tool: Symlink + /etc/timezone direkt.
	_ = os.Remove("/etc/localtime")
	if err := os.Symlink("/usr/share/zoneinfo/"+tz, "/etc/localtime"); err == nil {
		_ = os.WriteFile("/etc/timezone", []byte(tz+"\n"), 0o644)
		log.Printf("system: Zeitzone auf %s gesetzt (Symlink)", tz)
	}
}

// ensureGermanLocale erzeugt de_DE.UTF-8 (einmalig) und setzt sie als Standard — damit Chromium das
// native Datumsfeld (<input type="date">) als TT.MM.JJJJ zeigt und die Uhr deutsch formatiert.
func ensureGermanLocale() {
	if localeVorhanden() {
		return
	}
	if _, err := exec.LookPath("locale-gen"); err != nil {
		return // z. B. Pi-OS-Image regelt das anders; nichts erzwingen
	}
	// de_DE.UTF-8 in /etc/locale.gen freischalten (idempotent).
	if b, err := os.ReadFile("/etc/locale.gen"); err == nil {
		s := string(b)
		if !strings.Contains(s, "\nde_DE.UTF-8 UTF-8") && !strings.HasPrefix(s, "de_DE.UTF-8 UTF-8") {
			s = strings.ReplaceAll(s, "# de_DE.UTF-8 UTF-8", "de_DE.UTF-8 UTF-8")
			if !strings.Contains(s, "de_DE.UTF-8 UTF-8") {
				s += "\nde_DE.UTF-8 UTF-8\n"
			}
			_ = os.WriteFile("/etc/locale.gen", []byte(s), 0o644)
		}
	}
	if err := exec.Command("locale-gen", "de_DE.UTF-8").Run(); err != nil {
		log.Printf("system: locale-gen fehlgeschlagen: %v", err)
		return
	}
	_ = exec.Command("update-locale", "LANG=de_DE.UTF-8", "LANGUAGE=de_DE:de").Run()
	log.Printf("system: deutsche Locale (de_DE.UTF-8) erzeugt")
}

func localeVorhanden() bool {
	out, err := exec.Command("locale", "-a").Output()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, "de_de.utf8") || strings.Contains(s, "de_de.utf-8")
}
