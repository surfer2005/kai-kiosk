package main

import (
	"log"
	"net/http"
	"sync"
)

const version = "0.1.20"

type App struct {
	mu  sync.Mutex
	cfg *Config
}

func (a *App) getCfg() *Config { a.mu.Lock(); defer a.mu.Unlock(); return a.cfg }

// applyConfig speichert die neue Config und wendet Netzwerk + VNC an. Die Kiosk-Ziel-URL liest der
// Browser-Starter beim (nächsten) Start aus der config.json — eine URL-/IP-Änderung wird daher mit
// einem Geräte-Neustart wirksam (Knopf in der Konfig-Seite).
// applyConfig speichert, wendet Netzwerk an und setzt (best effort) das VNC-Passwort. Rückgabe: die
// VNC-Status-Notiz (für die Konfig-Seite) + ein Fehler, falls Speichern/Netzwerk scheitert.
func (a *App) applyConfig(n *Config) (string, error) {
	if err := saveConfig(n); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.cfg = n
	a.mu.Unlock()
	if err := applyNetwork(n); err != nil {
		return "", err
	}
	return applyVNC(n), nil
}

func main() {
	cfg := loadConfig()
	ensureSystem() // Zeitzone + deutsche Locale beim Start nachziehen (greift auch nach Selbst-Update)
	app := &App{cfg: cfg}

	// Feste IP + VNC schon beim Start anwenden — so greift eine vorab auf die Boot-Partition gelegte
	// kai-kiosk.json (Flotten-Rollout) sofort beim ersten Boot, ohne erst die Konfig-Seite zu öffnen.
	if err := applyNetwork(cfg); err != nil {
		log.Printf("netzwerk: %v", err)
	}
	applyVNC(cfg)

	go app.timeLoop() // Systemuhr an der KAI-Serveruhr halten (eine Zeit für die ganze Flotte)

	log.Printf("KAI-Kiosk %s — Konfig-Oberfläche auf %s (Ziel: %s)", version, cfg.Listen, cfg.KaiURL)
	if err := http.ListenAndServe(cfg.Listen, app.routes()); err != nil {
		log.Fatal(err)
	}
}
