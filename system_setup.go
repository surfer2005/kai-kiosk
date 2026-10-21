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
	ensureChromiumGerman()
	refreshKioskScript()
	refreshPortalExt()
	ensureLockdown()
}

// portalExtDir ist das Verzeichnis, aus dem kiosk-browser.sh die „Zurück zu KAI"-Erweiterung lädt.
const portalExtDir = "/usr/local/share/kai-kiosk/portal-ext"

// refreshPortalExt schreibt die in die Binärdatei eingebettete Chromium-Erweiterung beim Dienststart auf
// die Platte — gleiche Lehre wie refreshKioskScript: ein Selbst-Update tauscht nur die Binärdatei, also
// muss die Erweiterung aus dem Programm kommen, nicht aus der Installation. Idempotent (nur schreiben,
// wenn Inhalt abweicht), best effort, als root.
func refreshPortalExt() {
	if err := os.MkdirAll(portalExtDir, 0o755); err != nil {
		log.Printf("kiosk: portal-ext-Verzeichnis nicht anlegbar: %v", err)
		return
	}
	changed := false
	for _, name := range []string{"manifest.json", "content.js", "background.js"} {
		data, err := portalExtFS.ReadFile("portal-ext/" + name)
		if err != nil {
			log.Printf("kiosk: portal-ext/%s fehlt in der Binärdatei: %v", name, err)
			continue
		}
		dst := portalExtDir + "/" + name
		if cur, err := os.ReadFile(dst); err == nil && string(cur) == string(data) {
			continue
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			log.Printf("kiosk: portal-ext/%s nicht schreibbar: %v", name, err)
			continue
		}
		changed = true
	}
	if changed {
		log.Printf("kiosk: Portal-Erweiterung auf den Programmstand gebracht (%s)", portalExtDir)
	}
}

// ensureChromiumGerman stellt das deutsche Chromium-Sprachpaket sicher. Debians `chromium` liefert nur
// en-US; die deutschen Oberflächen-/Picker-Texte und das Datums-/Zeitformat der NATIVEN Felder stecken
// in `chromium-l10n` (de.pak). Fehlt es, ist die Oberfläche englisch, egal was an --lang/LANG gesetzt ist
// (Live-Fall NUC: Kalender „Clear/Today", Datum MM/DD/YYYY). Nur nachinstallieren, wenn de.pak fehlt und
// apt + Netz da sind — so repariert ein Selbst-Update auch schon ausgelieferte Terminals.
func ensureChromiumGerman() {
	ensureGermanDictionary()
	for _, p := range []string{
		"/usr/lib/chromium/locales/de.pak",
		"/usr/lib/chromium-browser/locales/de.pak",
	} {
		if _, err := os.Stat(p); err == nil {
			return // schon vorhanden
		}
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return
	}
	log.Printf("chromium: deutsches Sprachpaket (chromium-l10n) fehlt — installiere nach")
	cmd := exec.Command("apt-get", "install", "-y", "chromium-l10n")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("chromium: chromium-l10n-Installation fehlgeschlagen: %v (%s)", err, lastLine(out))
		return
	}
	log.Printf("chromium: chromium-l10n installiert — deutsche Oberfläche nach dem nächsten Kiosk-Start")
}

// ensureGermanDictionary legt das deutsche Hunspell-Wörterbuch (hunspell-de-de) nach — Rückfall für die
// Rechtschreibprüfung, falls Chromium das Wörterbuch nicht selbst laden kann (kein Netz beim ersten Start).
// Die Prüfsprache selbst setzt kiosk-browser.sh im Profil (spellcheck.dictionaries = de-DE).
func ensureGermanDictionary() {
	if _, err := os.Stat("/usr/share/hunspell/de_DE.dic"); err == nil {
		return
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return
	}
	log.Printf("chromium: deutsches Wörterbuch (hunspell-de-de) fehlt — installiere nach")
	cmd := exec.Command("apt-get", "install", "-y", "hunspell-de-de")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("chromium: hunspell-de-de-Installation fehlgeschlagen: %v (%s)", err, lastLine(out))
	}
}

func lastLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 200 {
		s = s[len(s)-200:]
	}
	return s
}

// refreshKioskScript schreibt das in die Binärdatei eingebettete kiosk-browser.sh beim Dienststart auf
// die Platte — damit das Start-Skript IMMER zum Programmstand passt (u. a. `--lang=de-DE` + Locale-
// Variablen für deutsche Datums-/Zeitfelder). Vorher kam es nur best-effort aus dem Netz (raw.github),
// lief ein Update nicht durch, blieb die alte Fassung ohne --lang aktiv (Live-Meldung: US-Datum/AM-PM).
// Lehre wie beim Bordwächter-Keyring/Kiosk-Skript. Greift nach dem nächsten (Neu-)Start der Sitzung.
func refreshKioskScript() {
	const dst = "/usr/local/bin/kiosk-browser.sh"
	if len(kioskBrowserSH) == 0 {
		return
	}
	if cur, err := os.ReadFile(dst); err == nil && string(cur) == string(kioskBrowserSH) {
		return // schon aktuell
	}
	tmp := dst + ".new"
	if err := os.WriteFile(tmp, kioskBrowserSH, 0o755); err != nil {
		log.Printf("kiosk: kiosk-browser.sh konnte nicht geschrieben werden: %v", err)
		return
	}
	if err := os.Rename(tmp, dst); err != nil {
		log.Printf("kiosk: kiosk-browser.sh konnte nicht ersetzt werden: %v", err)
		return
	}
	log.Printf("kiosk: kiosk-browser.sh auf den Programmstand gebracht")
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
