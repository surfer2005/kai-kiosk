package main

// Herzschlag an KAI: meldet minütlich Version + Geräte-Zustand (system-Block), damit das Kiosk-
// Terminal im Geräte-Monitoring von KAI auftaucht (online/offline, Temperatur, Platte, Version) —
// genau wie der Bordwächter. Nur ausgehend, best effort. Braucht ein in KAI registriertes Gerät:
// cfg.DeviceToken muss der Edge-Token eines edge_devices-Eintrags sein; sonst wird nichts gesendet.

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

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
	resp.Body.Close()
	if resp.StatusCode != 200 {
		kaiReachable = false
		log.Printf("heartbeat: http %d", resp.StatusCode)
		return
	}
	kaiReachable = true
}
