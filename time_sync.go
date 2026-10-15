package main

import (
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"
)

// timeLoop hält die Systemuhr des Terminals an der KAI-Serveruhr — damit die ganze Flotte EINER Zeit
// folgt und nichts auseinanderläuft (Nutzerwunsch). Es wird KEINE eigene KAI-Schnittstelle gebraucht:
// jede HTTP-Antwort trägt einen `Date:`-Header mit der Serverzeit; den lesen wir von der konfigurierten
// KAI-Adresse und stellen die Uhr, wenn sie um mehr als die Schwelle abweicht.
//
// Bewusst nur bei GROSSER Abweichung (NTP läuft, dann ist die Uhr ohnehin richtig — nicht dagegen
// anregeln). Der Dienst läuft als root, kann die Uhr also stellen. Ist keine KAI-Adresse gesetzt oder
// die Synchronisation abgeschaltet, passiert nichts.
const (
	timeSyncInterval  = 15 * time.Minute
	timeSyncThreshold = 20 * time.Second
)

func (a *App) timeLoop() {
	// kurz nach dem Start einmal prüfen (frisch gebootetes Terminal ohne RTC/NTP), dann im Intervall.
	time.Sleep(20 * time.Second)
	for {
		a.syncTimeFromKai()
		time.Sleep(timeSyncInterval)
	}
}

func (a *App) syncTimeFromKai() {
	cfg := a.getCfg()
	if !cfg.timeSyncOn() || cfg.KaiURL == "" {
		return
	}
	cl := &http.Client{Timeout: 8 * time.Second}
	// HEAD reicht — wir wollen nur den Date-Header, nicht den Seiteninhalt.
	req, err := http.NewRequest("HEAD", cfg.KaiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "kai-kiosk")
	resp, err := cl.Do(req)
	if err != nil {
		return // offline: dann eben nicht, kein Lärm
	}
	resp.Body.Close()
	d := resp.Header.Get("Date")
	if d == "" {
		return
	}
	srv, err := http.ParseTime(d) // RFC1123 (UTC), Sekundenauflösung
	if err != nil {
		return
	}
	drift := time.Since(srv)
	if drift < 0 {
		drift = -drift
	}
	if drift < timeSyncThreshold {
		return // Uhr stimmt (±Schwelle) — nichts tun
	}
	// Uhr auf die KAI-Zeit stellen. `date -s @<unix>` setzt die Epoch (zeitzonenunabhängig); die
	// Anzeige-Zeitzone bleibt die eingestellte (Europe/Berlin, s. ensureSystem).
	ts := strconv.FormatInt(srv.Unix(), 10)
	if out, err := exec.Command("date", "-s", "@"+ts).CombinedOutput(); err != nil {
		log.Printf("zeit: konnte Uhr nicht stellen: %v (%s)", err, out)
		return
	}
	// Falls eine Hardware-Uhr (RTC) da ist, die neue Zeit festschreiben (best effort).
	_ = exec.Command("hwclock", "--systohc").Run()
	log.Printf("zeit: Systemuhr nach KAI gestellt (Abweichung war %s)", drift.Round(time.Second))
}
