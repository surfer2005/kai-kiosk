package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBordmedienWeiterreichen(t *testing.T) {
	var angekommen []string
	geraet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/medien/befehl" || r.URL.Query().Get("token") != "abspiel" {
			http.Error(w, "unauthorized", 401)
			return
		}
		b, _ := io.ReadAll(r.Body)
		angekommen = append(angekommen, string(b))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer geraet.Close()
	os.Setenv("KAI_KIOSK_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	defer os.Unsetenv("KAI_KIOSK_CONFIG")
	bmListe, bmGeladen = nil, false
	bordmedienAusHerzschlag([]bordGeraet{
		{ID: 4, Name: "Bordmusik", Kind: "audio", ShipID: 17, URL: geraet.URL, Token: "abspiel"},
		{ID: 3, Name: "Bildschirm", Kind: "bildschirm", ShipID: 17, URL: geraet.URL, Token: "abspiel"},
		{ID: 9, Name: "Fremd", Kind: "audio", ShipID: 33, URL: geraet.URL, Token: "falsch"},
	})
	bmGeladen = false // vom Datenträger neu lesen (Neustart ohne Netz)
	mux := http.NewServeMux()
	(&App{cfg: &Config{}}).bordmedienRoutes(mux)

	senden := func(remote, body string) (*httptest.ResponseRecorder, map[string]any) {
		req := httptest.NewRequest("POST", "/api/bordmedien/befehl", strings.NewReader(body))
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var m map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		return rec, m
	}
	// ohne Gerät = Bordmusik des Schiffs (nur Audio, nur Schiff 17)
	_, m := senden("127.0.0.1:1", `{"ship_id":17,"befehl":{"art":"befehl","cmd":"vor"}}`)
	if m["geraete"] != float64(1) || len(angekommen) != 1 || !strings.Contains(angekommen[0], `"quelle":"terminal"`) {
		t.Fatalf("Bordmusik erwartet: %v %v", m, angekommen)
	}
	// bestimmter Bildschirm
	if _, m = senden("127.0.0.1:1", `{"ship_id":17,"geraet_id":3,"befehl":{"art":"steuerung","paused":true}}`); m["geraete"] != float64(1) {
		t.Fatalf("Bildschirm erwartet: %v", m)
	}
	// aus dem Netz gesperrt
	if rec, _ := senden("192.168.1.9:1", `{"ship_id":17,"befehl":{"art":"zeitplan"}}`); rec.Code != 403 {
		t.Fatalf("aus dem Netz muss gesperrt sein, war %d", rec.Code)
	}
}
