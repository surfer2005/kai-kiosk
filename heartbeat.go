package main

// Herzschlag an KAI: meldet minütlich Version + Geräte-Zustand (system-Block), damit das Kiosk-
// Terminal im Geräte-Monitoring von KAI auftaucht (online/offline, Temperatur, Platte, Version) —
// genau wie der Bordwächter. Nur ausgehend, best effort. Braucht ein in KAI registriertes Gerät:
// cfg.DeviceToken muss der Edge-Token eines edge_devices-Eintrags sein; sonst wird nichts gesendet.

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Aktualisierung aus KAI (Nutzervorgabe 2026-10-22: alle Geräte direkt aus KAI aktualisieren, am Gerät
// bleibt der Knopf „Aktualisieren“ daneben bestehen). KAI kann das Gerät nicht ansprechen — das Gerät fragt
// alle 15 s nach (GET /api/edge/auftrag) und bekommt die Anforderung zusätzlich mit dem Herzschlag.
var updateLaeuft sync.Mutex

func (a *App) kaiRequest(method, path string) (*http.Response, error) {
	cfg := a.getCfg()
	base := strings.TrimRight(strings.TrimSpace(cfg.KaiURL), "/")
	token := strings.TrimSpace(cfg.DeviceToken)
	if base == "" || token == "" {
		return nil, io.EOF
	}
	req, err := http.NewRequest(method, base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Edge-Token", token)
	return (&http.Client{Timeout: 15 * time.Second}).Do(req)
}

func (a *App) updateAusKai() {
	if !updateLaeuft.TryLock() {
		return
	}
	defer updateLaeuft.Unlock()
	if resp, err := a.kaiRequest("POST", "/api/edge/update-gestartet"); err == nil {
		resp.Body.Close()
	}
	if err := selfUpdate(); err != nil {
		logEvent("error", "Aktualisierung (aus KAI) fehlgeschlagen: "+err.Error())
		return
	}
	logEvent("info", "Aktualisierung aus KAI geladen, Neustart (bisher Version "+version+")")
	reexecSoon()
}

func (a *App) auftragLoop() {
	time.Sleep(30 * time.Second)
	for {
		if resp, err := a.kaiRequest("GET", "/api/edge/auftrag"); err == nil {
			var au struct {
				Update bool `json:"update"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&au)
			resp.Body.Close()
			if au.Update {
				go a.updateAusKai()
			}
		}
		time.Sleep(15 * time.Second)
	}
}

func (a *App) heartbeatLoop() {
	time.Sleep(20 * time.Second) // Netz/Zeit erst hochkommen lassen
	for {
		a.sendHeartbeat()
		time.Sleep(60 * time.Second)
	}
}

func (a *App) sendHeartbeat() {
	cfg := a.getCfg()
	base := strings.TrimRight(strings.TrimSpace(cfg.KaiURL), "/")
	token := strings.TrimSpace(cfg.DeviceToken)
	if base == "" || token == "" {
		return // ohne Ziel/Token kein Herzschlag
	}
	body, err := json.Marshal(map[string]any{
		"version": version,
		"kind":    "kiosk", // Gerätetyp fürs Monitoring (Kiosk vs. Bordwächter)
		"name":    strings.TrimSpace(cfg.DeviceName), // Anzeigename im KAI-Monitoring (aus der Config)
		"system":  collectSystem(),
		"status":  "Kiosk aktiv",
	})
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", base+"/api/edge/hello", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Edge-Token", token)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		kaiReachable = false
		log.Printf("heartbeat: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		kaiReachable = false
		log.Printf("heartbeat: http %d", resp.StatusCode)
		return
	}
	kaiReachable = true
	var hr struct {
		UpdateRequested bool `json:"update_requested"`
	}
	if json.NewDecoder(resp.Body).Decode(&hr) == nil && hr.UpdateRequested {
		go a.updateAusKai()
	}
}
