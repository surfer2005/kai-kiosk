package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Fernzugriff/Identität (wie beim Bordwächter-Gerät): Hostname und das SSH-Passwort werden aus der
// Konfig angewandt (best effort, als root) — Gerät als <hostname>.local erreichbar. Der SSH-Benutzer
// wird ERKANNT (NUC-Image: "pi" künftig, Altbestand "kiosk"; Raspi: "pi"), damit Anzeige und chpasswd
// immer den echten Login-Benutzer treffen. Greift beim Dienststart UND beim Speichern.
var hostnameOK = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// detectSSHUser: der tatsächliche Desktop-/Login-Benutzer. Zuerst der lightdm-Autologin-Benutzer
// (NUC), sonst "pi" (Raspi / neue NUC-Images), mit /home als letztem Anhalt.
func detectSSHUser() string {
	matches, _ := filepath.Glob("/etc/lightdm/lightdm.conf.d/*.conf")
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "autologin-user=") {
				if u := strings.TrimSpace(strings.TrimPrefix(line, "autologin-user=")); u != "" {
					return u
				}
			}
		}
	}
	if _, err := os.Stat("/home/pi"); err == nil {
		return "pi"
	}
	return "pi"
}

func applyHostname(cfg *Config) {
	hn := strings.TrimSpace(cfg.Hostname)
	if hn == "" || !hostnameOK.MatchString(hn) {
		return
	}
	cur, _ := os.ReadFile("/etc/hostname")
	if strings.TrimSpace(string(cur)) == hn {
		return
	}
	if _, err := exec.LookPath("hostnamectl"); err == nil {
		if err := exec.Command("hostnamectl", "set-hostname", hn).Run(); err != nil {
			log.Printf("hostname: hostnamectl fehlgeschlagen: %v", err)
		}
	}
	_ = os.WriteFile("/etc/hostname", []byte(hn+"\n"), 0o644)
	if b, err := os.ReadFile("/etc/hosts"); err == nil {
		lines := strings.Split(string(b), "\n")
		found := false
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "127.0.1.1") {
				lines[i] = "127.0.1.1\t" + hn
				found = true
			}
		}
		if !found {
			lines = append(lines, "127.0.1.1\t"+hn)
		}
		_ = os.WriteFile("/etc/hosts", []byte(strings.Join(lines, "\n")), 0o644)
	}
	log.Printf("hostname: auf %q gesetzt (voll wirksam nach Neustart)", hn)
}

func applySSHPassword(cfg *Config) {
	pw := strings.TrimSpace(cfg.SSHPassword)
	if pw == "" {
		return
	}
	user := detectSSHUser()
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(user + ":" + pw + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("ssh: Passwort für %q konnte nicht gesetzt werden: %v (%s)", user, err, strings.TrimSpace(string(out)))
		return
	}
	log.Printf("ssh: Passwort für Benutzer %q gesetzt", user)
}

// sshInfo: Daten für den Fernzugriff-Block der Konfig-Seite.
func sshInfo(cfg *Config) map[string]any {
	hn := strings.TrimSpace(cfg.Hostname)
	ip := ""
	if ips := localIPv4s(); len(ips) > 0 {
		ip = ips[0]
	}
	pw := strings.TrimSpace(cfg.SSHPassword)
	if pw == "" {
		pw = "kai-terminal"
	}
	host := ip
	if hn != "" {
		host = hn + ".local"
	}
	user := detectSSHUser()
	return map[string]any{"user": user, "password": pw, "host": host, "ip": ip, "hostname": hn,
		"command": "ssh " + user + "@" + host}
}
