package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config: alle Einstellungen des KAI-Kiosk-Terminals. Als JSON gespeichert, über die lokale
// Web-Oberfläche gepflegt. Bewusst flach + mit Defaults, damit "flashen → eintragen → läuft".
// Dieselbe Machart wie das Bordwächter-Gerät, nur ohne GPS/SIFA/GPIO.
type Config struct {
	KaiURL      string `json:"kai_url"`      // Adresse, die der Kiosk im Vollbild öffnet (PWA-Anmeldemaske)
	Listen      string `json:"listen"`       // lokale Konfig-UI/API, z. B. ":8091"
	UIPassword  string `json:"ui_password"`  // Schutz der lokalen Konfig-Seite (leer = offen im LAN)
	VNCPassword string `json:"vnc_password"` // Passwort für den VNC-Fernzugriff (leer = unverändert lassen)
	DeviceToken string `json:"device_token"` // Geräte-Token (vertrauenswürdiges Terminal) — greift, sobald KAI die Token-Auth hat
	DeviceName  string `json:"device_name"`  // Anzeigename im KAI-Monitoring (z. B. "Hafenbüro Konstanz") — unabhängig vom Hostnamen
	Hostname    string `json:"hostname"`     // Gerätename (hostnamectl + /etc/hosts); Gerät danach als <hostname>.local erreichbar
	SSHPassword string `json:"ssh_password"` // Passwort des SSH-Benutzers "pi" (leer = unverändert / Image-Standard "kai-terminal")
	// Uhr nach KAI stellen: das Terminal liest periodisch die Serverzeit (Date-Header der KAI-Adresse)
	// und korrigiert bei Abweichung seine Systemuhr. So folgt die ganze Flotte EINER Zeit (der KAI-
	// Serveruhr) und nichts läuft auseinander — auch dort, wo kein NTP erreichbar ist. Vorgabe: an.
	TimeSync *bool `json:"time_sync"`

	// Netzwerk: DHCP (Vorgabe) oder feste IP. Wird als NetworkManager-Keyfile geschrieben und
	// beim Start angewandt (reines Datei-Schreiben, kein nmcli/Shell).
	Network struct {
		Mode      string `json:"mode"`      // "dhcp" | "static"
		Interface string `json:"interface"` // z. B. "eth0"
		Address   string `json:"address"`   // CIDR, z. B. "192.168.1.50/24"
		Gateway   string `json:"gateway"`   // z. B. "192.168.1.1"
		DNS       string `json:"dns"`       // Komma-getrennt, z. B. "192.168.1.1,1.1.1.1"
	} `json:"network"`
}

// timeSyncOn: Uhr-nach-KAI ist standardmäßig an (fehlendes Feld in alten Configs = an).
func (c *Config) timeSyncOn() bool { return c.TimeSync == nil || *c.TimeSync }

func defaultConfig() *Config {
	c := &Config{
		// Bewusst LEER: die KAI-Adresse steht nicht in der (öffentlichen) Quelle, sondern wird pro
		// Terminal gesetzt — über die Konfig-Seite oder die kai-kiosk.json auf der Boot-Partition.
		// Ist keine Adresse gesetzt, öffnet der Kiosk die lokale Konfig-Seite zum Eintragen.
		Listen: ":8091",
	}
	c.Network.Mode = "dhcp"
	c.Network.Interface = "eth0"
	c.Hostname = "kai-kiosk"
	return c
}

func configPath() string {
	if p := os.Getenv("KAI_KIOSK_CONFIG"); p != "" {
		return p
	}
	if _, err := os.Stat("/etc/kai-kiosk"); err == nil {
		return "/etc/kai-kiosk/config.json"
	}
	return "config.json"
}

var cfgMu sync.Mutex

func loadConfig() *Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	c := defaultConfig()
	b, err := os.ReadFile(configPath())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, c) // unbekannte/fehlende Felder behalten die Defaults
	if c.Listen == "" {
		c.Listen = ":8091"
	}
	return c
}

func saveConfig(c *Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	p := configPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
