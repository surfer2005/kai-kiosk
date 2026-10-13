package main

import (
	"log"
	"os/exec"
	"strings"
)

// applyVNC setzt das VNC-Passwort für den Fernzugriff — best effort beim Start. RPi OS nutzt RealVNC;
// `vncpasswd -service` setzt das Dienst-Passwort. Schlägt es fehl (anderes VNC/kein Tool), wird es
// nur protokolliert, der Dienst läuft weiter.
// ⚠️ An echter RPi-OS-Fassung verifizieren (RealVNC vs. wayvnc) — dieselbe Vorsicht wie bei den
// Hardware-Pfaden im Bordwächter.
func applyVNC(cfg *Config) {
	pw := strings.TrimSpace(cfg.VNCPassword)
	if pw == "" {
		return // leer = unverändert lassen
	}
	if _, err := exec.LookPath("vncpasswd"); err != nil {
		log.Printf("vnc: vncpasswd nicht gefunden — VNC-Passwort nicht gesetzt (%v)", err)
		return
	}
	cmd := exec.Command("vncpasswd", "-service")
	cmd.Stdin = strings.NewReader(pw + "\n" + pw + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("vnc: vncpasswd fehlgeschlagen: %v (%s)", err, strings.TrimSpace(string(out)))
		return
	}
	log.Printf("vnc: Passwort gesetzt")
}
