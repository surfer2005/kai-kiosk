package main

// Geräte-Zustand des Kiosk-Terminals: CPU-Temperatur, CPU-Auslastung, Last, RAM, Platte, Laufzeit.
// Aus /proc und /sys (Linux, kein externes Paket, CGO aus) — übernommen aus dem Bordwächter, damit
// Kiosk und Bordwächter denselben system-Block an KAI melden (Geräte-Monitoring). cpuTemp fällt auf
// der x86-/NUC-Hardware ggf. weg (kein thermal_zone0) → Feld wird dann weggelassen, kein Fehler.

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SystemData struct {
	CPUTempC   *float64 `json:"cpu_temp_c,omitempty"`
	CPUPct     *float64 `json:"cpu_pct,omitempty"`
	Load1      *float64 `json:"load1,omitempty"`
	MemPct     *float64 `json:"mem_pct,omitempty"`
	MemTotalMB *int     `json:"mem_total_mb,omitempty"`
	DiskPct    *float64 `json:"disk_pct,omitempty"`
	UptimeSec  int      `json:"uptime_sec"`
}

func parseTempMilli(b string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if err != nil {
		return 0, false
	}
	return v / 1000.0, true
}

func parseLoadavg1(s string) (float64, bool) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseUptimeSec(s string) (int, bool) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, false
	}
	return int(v), true
}

func parseMeminfoPct(content string) (pct float64, totalMB int, ok bool) {
	var total, avail float64
	haveT, haveA := false, false
	for _, ln := range strings.Split(content, "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "MemTotal:":
			if v, err := strconv.ParseFloat(f[1], 64); err == nil {
				total, haveT = v, true
			}
		case "MemAvailable:":
			if v, err := strconv.ParseFloat(f[1], 64); err == nil {
				avail, haveA = v, true
			}
		}
	}
	if !haveT || !haveA || total <= 0 {
		return 0, 0, false
	}
	return (total - avail) / total * 100.0, int(total / 1024.0), true
}

func cpuBusyTotal(content string) (busy, total float64, ok bool) {
	for _, ln := range strings.Split(content, "\n") {
		if !strings.HasPrefix(ln, "cpu ") {
			continue
		}
		f := strings.Fields(ln)[1:]
		var sum, idle float64
		for i, v := range f {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			sum += n
			if i == 3 || i == 4 {
				idle += n
			}
		}
		return sum - idle, sum, sum > 0
	}
	return 0, 0, false
}

func readFileStr(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func cpuTemp() (float64, bool) {
	if s := readFileStr("/sys/class/thermal/thermal_zone0/temp"); s != "" {
		return parseTempMilli(s)
	}
	return 0, false
}

func diskPct(path string) (float64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, false
	}
	total := float64(st.Blocks) * float64(st.Bsize)
	free := float64(st.Bavail) * float64(st.Bsize)
	if total <= 0 {
		return 0, false
	}
	return (total - free) / total * 100.0, true
}

// collectSystem erstellt eine Momentaufnahme. CPU-% über eine kurze zweite Probe.
func collectSystem() *SystemData {
	s := &SystemData{}
	if t, ok := cpuTemp(); ok {
		s.CPUTempC = &t
	}
	if l, ok := parseLoadavg1(readFileStr("/proc/loadavg")); ok {
		s.Load1 = &l
	}
	if up, ok := parseUptimeSec(readFileStr("/proc/uptime")); ok {
		s.UptimeSec = up
	}
	if pct, mb, ok := parseMeminfoPct(readFileStr("/proc/meminfo")); ok {
		s.MemPct = &pct
		s.MemTotalMB = &mb
	}
	if d, ok := diskPct("/"); ok {
		s.DiskPct = &d
	}
	if b1, t1, ok1 := cpuBusyTotal(readFileStr("/proc/stat")); ok1 {
		time.Sleep(500 * time.Millisecond)
		if b2, t2, ok2 := cpuBusyTotal(readFileStr("/proc/stat")); ok2 && t2 > t1 {
			pct := (b2 - b1) / (t2 - t1) * 100.0
			if pct < 0 {
				pct = 0
			}
			s.CPUPct = &pct
		}
	}
	return s
}
