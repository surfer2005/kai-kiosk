package main

// Geräte-Protokoll des Kiosk: lokale Ereignisse/Fehler + Neustarts werden gepuffert (auch offline, auf
// Platte) und beim Wiederverbinden an KAI gemeldet (POST /api/edge/log). Zentral im Monitoring lesbar,
// statt per SSH ins systemd-Journal. Nur ausgehend, best effort.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type logEntry struct {
	At         string `json:"at"`
	Level      string `json:"level"`
	Message    string `json:"message"`
	Key        string `json:"key"`
	WasOffline bool   `json:"was_offline"`
}

var (
	logMu        sync.Mutex
	logBuf       []logEntry
	logSeq       int64
	kaiReachable = true
)

func devlogPath() string {
	if p := os.Getenv("KAI_KIOSK_DEVLOG"); p != "" {
		return p
	}
	if _, err := os.Stat("/etc/kai-kiosk"); err == nil {
		return "/etc/kai-kiosk/devlog.json"
	}
	return "devlog.json"
}

func persistLog() { // Aufrufer hält logMu
	data, err := json.Marshal(logBuf)
	if err != nil {
		return
	}
	p := devlogPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	tmp := p + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, p)
	}
}

func loadLog() {
	data, err := os.ReadFile(devlogPath())
	if err != nil {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	_ = json.Unmarshal(data, &logBuf)
}

func logEvent(level, msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	logSeq++
	logBuf = append(logBuf, logEntry{
		At:         time.Now().UTC().Format(time.RFC3339),
		Level:      level,
		Message:    msg,
		Key:        fmt.Sprintf("%d-%d", time.Now().UnixNano(), logSeq),
		WasOffline: !kaiReachable,
	})
	if len(logBuf) > 500 {
		logBuf = logBuf[len(logBuf)-500:]
	}
	persistLog()
}

func postLog(base, token string, entries []logEntry) error {
	body, _ := json.Marshal(map[string]any{"entries": entries})
	req, err := http.NewRequest("POST", base+"/api/edge/log", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Edge-Token", token)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("log http %d", resp.StatusCode)
	}
	return nil
}

func (a *App) devlogLoop() {
	loadLog()
	logEvent("info", "KAI-Kiosk gestartet, Version "+version)
	for {
		cfg := a.getCfg()
		base := strings.TrimRight(strings.TrimSpace(cfg.KaiURL), "/")
		token := strings.TrimSpace(cfg.DeviceToken)
		if base != "" && token != "" {
			logMu.Lock()
			batch := append([]logEntry(nil), logBuf...)
			logMu.Unlock()
			if len(batch) > 0 {
				if postLog(base, token, batch) == nil {
					sent := make(map[string]bool, len(batch))
					for _, e := range batch {
						sent[e.Key] = true
					}
					logMu.Lock()
					rest := logBuf[:0]
					for _, e := range logBuf {
						if !sent[e.Key] {
							rest = append(rest, e)
						}
					}
					logBuf = append([]logEntry(nil), rest...)
					persistLog()
					logMu.Unlock()
				}
			}
		}
		time.Sleep(60 * time.Second)
	}
}
