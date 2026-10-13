package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed kiosk.html
var kioskHTML []byte

// Quelle für den Self-Update ("Aktualisieren"-Knopf): die neueste Release-Binärdatei des Kiosk-Repos.
const updateBase = "https://github.com/surfer2005/kai-kiosk/releases/latest/download"
const ghLatestAPI = "https://api.github.com/repos/surfer2005/kai-kiosk/releases/latest"

// reexecSoon ersetzt den laufenden Prozess nach kurzer Verzögerung durch sich selbst (liest die Config
// neu). Die Verzögerung lässt die HTTP-Antwort noch raus.
func reexecSoon() {
	go func() {
		time.Sleep(400 * time.Millisecond)
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		_ = syscall.Exec(exe, os.Args, os.Environ())
		os.Exit(0) // nur erreichbar, wenn Exec fehlschlägt → systemd startet neu
	}()
}

// selfUpdate lädt die passende Release-Binärdatei und schiebt sie per Rename über die eigene Datei.
func selfUpdate() error {
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "armv7"
	}
	url := updateBase + "/kai-kiosk-" + arch
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cl := &http.Client{Timeout: 120 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Download fehlgeschlagen (HTTP %d) von %s", resp.StatusCode, url)
	}
	tmp := exe + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Versionsprüfung: liegt in GitHub eine neuere Version als die laufende vor? 1 h zwischengespeichert;
// bei Offline/Fehler bleibt der letzte bekannte Stand.
var (
	relMu  sync.Mutex
	relTag string
	relAt  time.Time
)

func latestReleaseTag() string {
	relMu.Lock()
	if relTag != "" && time.Since(relAt) < time.Hour {
		t := relTag
		relMu.Unlock()
		return t
	}
	relMu.Unlock()
	cl := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", ghLatestAPI, nil)
	req.Header.Set("User-Agent", "kai-kiosk")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := cl.Do(req)
	if err != nil {
		relMu.Lock()
		t := relTag
		relMu.Unlock()
		return t
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var j struct {
			TagName string `json:"tag_name"`
		}
		if json.NewDecoder(resp.Body).Decode(&j) == nil && j.TagName != "" {
			relMu.Lock()
			relTag = j.TagName
			relAt = time.Now()
			relMu.Unlock()
			return j.TagName
		}
	}
	relMu.Lock()
	t := relTag
	relMu.Unlock()
	return t
}

// verNewer: ist latest (z. B. "v0.1.2") höher als current ("0.1.1")? Nur Zahlenteile, führendes v egal.
func verNewer(latest, current string) bool {
	norm := func(s string) []int {
		s = strings.TrimPrefix(strings.TrimSpace(s), "v")
		parts := strings.Split(s, ".")
		out := make([]int, len(parts))
		for i, p := range parts {
			out[i], _ = strconv.Atoi(p)
		}
		return out
	}
	a, b := norm(latest), norm(current)
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// authOK: gültige Cookie-Sitzung ODER Basic-Auth mit dem UI-Passwort. Kein WWW-Authenticate (kein
// Browser-Popup); die Seite zeigt ihre eigene Anmeldemaske. Leeres UI-Passwort = offen im LAN.
func (a *App) authOK(r *http.Request) bool {
	ui := a.getCfg().UIPassword
	if ui == "" {
		return true
	}
	if c, err := r.Cookie("kk_session"); err == nil && validSession(c.Value) {
		return true
	}
	if _, pass, ok := r.BasicAuth(); ok && pass == ui {
		return true
	}
	return false
}

func (a *App) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authOK(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Die Konfig-Seite selbst ist nicht geschützt (nur HTML/JS); alle Daten/Aktionen dahinter sind es.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(kioskHTML)
	})

	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var in struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		ui := a.getCfg().UIPassword
		if ui == "" || in.Password == ui {
			http.SetCookie(w, &http.Cookie{
				Name: "kk_session", Value: newSession(), Path: "/",
				HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL / time.Second),
			})
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		writeJSON(w, map[string]any{"ok": false, "error": "Passwort falsch"})
	})

	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("kk_session"); err == nil {
			dropSession(c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "kk_session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
		writeJSON(w, map[string]any{"ok": true})
	})

	// Ob ein UI-Passwort gesetzt ist (steuert, ob die Seite eine Anmeldemaske zeigt) — ohne Login lesbar.
	mux.HandleFunc("/api/needs-login", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"needsLogin": a.getCfg().UIPassword != "", "authed": a.authOK(r)})
	})

	mux.HandleFunc("/api/config", a.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var c Config
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			// Listen bleibt, was läuft (erst nach Neustart änderbar) — sonst sperrt man sich aus.
			c.Listen = a.getCfg().Listen
			if err := a.applyConfig(&c); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		writeJSON(w, a.getCfg())
	}))

	// Nur Netzwerk setzen (feste IP/DHCP). Wirkt nach Geräte-Neustart.
	mux.HandleFunc("/api/network", a.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var in struct{ Mode, Interface, Address, Gateway, DNS string }
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		a.mu.Lock()
		a.cfg.Network.Mode = in.Mode
		a.cfg.Network.Interface = in.Interface
		a.cfg.Network.Address = in.Address
		a.cfg.Network.Gateway = in.Gateway
		a.cfg.Network.DNS = in.DNS
		cfg := a.cfg
		a.mu.Unlock()
		if err := applyNetwork(cfg); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err := saveConfig(cfg); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}))

	// Aktualisieren: neueste Release-Binärdatei holen, über die eigene schieben, neu starten.
	mux.HandleFunc("/api/update", a.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		if err := selfUpdate(); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		reexecSoon()
	}))

	mux.HandleFunc("/api/update-check", a.guard(func(w http.ResponseWriter, r *http.Request) {
		latest := latestReleaseTag()
		writeJSON(w, map[string]any{
			"current":         version,
			"latest":          latest,
			"updateAvailable": latest != "" && verNewer(latest, version),
		})
	}))

	// Konfig-Dienst neu starten (liest Config neu) — ohne Geräte-Neustart.
	mux.HandleFunc("/api/restart", a.guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		reexecSoon()
	}))

	// Gerät (Raspberry) komplett neu starten — nötig, damit eine geänderte URL/feste IP greift.
	mux.HandleFunc("/api/reboot", a.guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			syscall.Sync()
			_ = syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
		}()
	}))

	return mux
}
