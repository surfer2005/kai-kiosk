package main

// Bordmedien ohne Netz (Festlegung 2026-10-24): am Schiffsterminal bleiben die Knöpfe unter „Bordmedien“
// auch ohne Verbindung zu KAI bedienbar. KAI (Seite im Browser, https) darf nichts per http ins Bord-Netz
// schicken — nur ans eigene Gerät. Deshalb schickt die Seite den Knopfdruck an DIESEN Dienst (localhost),
// und der reicht ihn direkt an Bildschirm bzw. Bordmusik weiter (nie über den Bordwächter). Welche Geräte
// es an Bord gibt (Adresse + Bedien-Schlüssel), meldet KAI mit dem Herzschlag; das Terminal merkt es sich
// für den Fall ohne Netz. Für die Freigabe „Zugriff aufs lokale Netz“ setzt ensureLokalesNetz die
// Chromium-Richtlinie für die KAI-Adresse.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type bordGeraet struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // bildschirm | audio
	ShipID int    `json:"ship_id"`
	URL    string `json:"url"`
	Token  string `json:"token"`
}

// Geräteliste liegt in einer EIGENEN Datei neben der Konfiguration (nicht in config.json): sie trägt die
// Bedien-Schlüssel, und die Konfig-Seite liefert config.json im Bord-Netz aus.
var (
	bmMu      sync.Mutex
	bmListe   []bordGeraet
	bmGeladen bool
)

func bordmedienPfad() string { return filepath.Join(filepath.Dir(configPath()), "bordmedien.json") }

func bordmedienListe() []bordGeraet {
	bmMu.Lock()
	defer bmMu.Unlock()
	if !bmGeladen {
		if b, err := os.ReadFile(bordmedienPfad()); err == nil {
			_ = json.Unmarshal(b, &bmListe)
		}
		bmGeladen = true
	}
	return bmListe
}

// bordmedienAusHerzschlag übernimmt die Geräteliste (nur bei Änderung; fehlt sie, bleibt die alte).
func bordmedienAusHerzschlag(liste []bordGeraet) {
	if liste == nil {
		return
	}
	alt, _ := json.Marshal(bordmedienListe())
	neu, _ := json.Marshal(liste)
	if bytes.Equal(alt, neu) {
		return
	}
	bmMu.Lock()
	bmListe = liste
	bmMu.Unlock()
	if err := os.WriteFile(bordmedienPfad(), neu, 0o600); err != nil {
		log.Printf("bordmedien: Geräteliste nicht gespeichert: %v", err)
	}
}

// zielGeraete: ein bestimmtes Gerät, (ohne geraet_id) die Bordmusik oder (-1) alle Geräte des Schiffs.
func zielGeraete(liste []bordGeraet, shipID, geraetID int) []bordGeraet {
	out := []bordGeraet{}
	for _, g := range liste {
		if g.ShipID != shipID {
			continue
		}
		// geraet_id > 0: genau dieses Gerät; 0: die Bordmusik; < 0: alle Geräte des Schiffs (Ereignis-Knopf)
		if (geraetID > 0 && g.ID == geraetID) || (geraetID == 0 && g.Kind == "audio") || geraetID < 0 {
			out = append(out, g)
		}
	}
	return out
}

func nurVomGeraet(w http.ResponseWriter, r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	return true
}

var bordHTTP = &http.Client{Timeout: 3 * time.Second}

func (a *App) bordmedienRoutes(mux *http.ServeMux) {
	// Knopfdruck aus „Bordmedien“ (KAI-Seite auf diesem Terminal) → Bildschirm/Bordmusik im Bord-Netz
	mux.HandleFunc("/api/bordmedien/befehl", func(w http.ResponseWriter, r *http.Request) {
		if !nurVomGeraet(w, r) {
			return
		}
		var b struct {
			ShipID   int             `json:"ship_id"`
			GeraetID int             `json:"geraet_id"`
			Befehl   json.RawMessage `json:"befehl"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&b); err != nil || len(b.Befehl) == 0 {
			http.Error(w, "ungültig", 400)
			return
		}
		var bef map[string]any
		_ = json.Unmarshal(b.Befehl, &bef)
		bef["quelle"] = "terminal"
		body, _ := json.Marshal(bef)
		ziele := zielGeraete(bordmedienListe(), b.ShipID, b.GeraetID)
		ok, fehler := 0, []string{}
		for _, g := range ziele {
			resp, err := bordHTTP.Post(strings.TrimRight(g.URL, "/")+"/api/medien/befehl?token="+url.QueryEscape(g.Token), "application/json", bytes.NewReader(body))
			if err != nil {
				fehler = append(fehler, g.Name+": nicht erreichbar")
				continue
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 {
				fehler = append(fehler, fmt.Sprintf("%s: HTTP %d", g.Name, resp.StatusCode))
				continue
			}
			ok++
		}
		writeJSON(w, map[string]any{"ok": ok > 0, "geraete": ok, "von": len(ziele), "fehler": fehler})
	})
	// Was läuft an Bord (für die Anzeige ohne Netz)
	mux.HandleFunc("/api/bordmedien/stand", func(w http.ResponseWriter, r *http.Request) {
		if !nurVomGeraet(w, r) {
			return
		}
		var ship int
		_, _ = fmt.Sscanf(r.URL.Query().Get("ship_id"), "%d", &ship)
		out := []map[string]any{}
		for _, g := range bordmedienListe() {
			if g.ShipID != ship {
				continue
			}
			e := map[string]any{"id": g.ID, "name": g.Name, "kind": g.Kind, "erreichbar": false}
			if resp, err := bordHTTP.Get(strings.TrimRight(g.URL, "/") + "/api/medien/stand?token=" + url.QueryEscape(g.Token)); err == nil {
				var st map[string]any
				if resp.StatusCode == 200 && json.NewDecoder(resp.Body).Decode(&st) == nil {
					e["erreichbar"], e["stand"] = true, st["stand"]
				}
				resp.Body.Close()
			}
			out = append(out, e)
		}
		writeJSON(w, map[string]any{"geraete": out})
	})
}

// ensureLokalesNetz: Chromium-Freigabe „Zugriff aufs lokale Netz“ für die KAI-Adresse (verwaltete
// Richtlinie, ohne Rückfrage am Bildschirm). Best effort, als root.
func ensureLokalesNetz(c *Config) {
	u, err := url.Parse(strings.TrimSpace(c.KaiURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return
	}
	pol, _ := json.MarshalIndent(map[string]any{"LocalNetworkAccessAllowedForUrls": []string{u.Scheme + "://" + u.Host}}, "", "  ")
	for _, dir := range []string{"/etc/chromium/policies/managed", "/etc/chromium-browser/policies/managed"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		p := filepath.Join(dir, "kai-lokales-netz.json")
		if alt, err := os.ReadFile(p); err == nil && bytes.Equal(alt, pol) {
			continue
		}
		if err := os.WriteFile(p, pol, 0o644); err != nil {
			log.Printf("kiosk: Chromium-Richtlinie %s nicht schreibbar: %v", p, err)
		}
	}
}
