package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

// applyVNC setzt das VNC-Passwort für den Fernzugriff — backend-abhängig, best effort.
// Gibt eine kurze Status-Notiz zurück (für die Konfig-Seite), damit ein Fehlschlag SICHTBAR wird
// statt still im Journal zu landen (genau das verdeckte bisher, dass am NUC gar kein RealVNC läuft).
//   - NUC (Debian + x11vnc, s. nuc/kiosk-setup.sh): Passwortdatei /etc/kai-kiosk/vncpasswd über
//     `x11vnc -storepasswd` schreiben und x11vnc.service neu starten (die Unit liest die Datei beim Start).
//   - Raspberry Pi OS (RealVNC): `vncpasswd -service`.
// Leer = unverändert lassen.
const vncPassFile = "/etc/kai-kiosk/vncpasswd"

func applyVNC(cfg *Config) string {
	pw := strings.TrimSpace(cfg.VNCPassword)
	if pw == "" {
		return "" // leer = unverändert lassen
	}

	// 1) NUC/Debian: x11vnc (so richtet es nuc/kiosk-setup.sh ein).
	if _, err := exec.LookPath("x11vnc"); err == nil {
		if out, err := exec.Command("x11vnc", "-storepasswd", pw, vncPassFile).CombinedOutput(); err != nil {
			log.Printf("vnc: x11vnc -storepasswd fehlgeschlagen: %v (%s)", err, strings.TrimSpace(string(out)))
			return "VNC: Passwortdatei konnte nicht geschrieben werden"
		}
		_ = os.Chmod(vncPassFile, 0o600)
		// Dienst neu starten, damit er die Datei (statt -nopw) übernimmt. Fehlt die Unit (z. B. Pi), egal.
		_ = exec.Command("systemctl", "restart", "x11vnc.service").Run()
		log.Printf("vnc: x11vnc-Passwort gesetzt")
		return "VNC-Passwort gesetzt (x11vnc)"
	}

	// 2) Raspberry Pi OS: RealVNC.
	if _, err := exec.LookPath("vncpasswd"); err == nil {
		cmd := exec.Command("vncpasswd", "-service")
		cmd.Stdin = strings.NewReader(pw + "\n" + pw + "\n")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("vnc: vncpasswd fehlgeschlagen: %v (%s)", err, strings.TrimSpace(string(out)))
			return "VNC: RealVNC-Passwort konnte nicht gesetzt werden"
		}
		log.Printf("vnc: RealVNC-Passwort gesetzt")
		return "VNC-Passwort gesetzt (RealVNC)"
	}

	log.Printf("vnc: weder x11vnc noch RealVNC gefunden — VNC-Passwort nicht gesetzt")
	return "VNC: kein Server gefunden (x11vnc/RealVNC) — Passwort nicht gesetzt"
}
