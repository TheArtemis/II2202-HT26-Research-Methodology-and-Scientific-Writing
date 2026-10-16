package collect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SampleHostLoad reads /proc/loadavg and MemAvailable (Linux). On non-Linux it
// returns a sample with only a timestamp (load fields zero, ok=false).
func SampleHostLoad() (HostLoadSample, bool) {
	s := HostLoadSample{TSNS: time.Now().UnixNano()}
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return s, false
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		return s, false
	}
	s.Load1, _ = strconv.ParseFloat(fields[0], 64)
	s.Load5, _ = strconv.ParseFloat(fields[1], 64)
	s.Load15, _ = strconv.ParseFloat(fields[2], 64)
	s.MemAvailableKB = readMemAvailableKB()
	return s, true
}

func readMemAvailableKB() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseInt(fields[1], 10, 64)
				return v
			}
		}
	}
	return 0
}

// AppendHostLoadJSONL appends one sample to path.
func AppendHostLoadJSONL(path string, s HostLoadSample) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// FormatLoadSample is a debug helper.
func FormatLoadSample(s HostLoadSample) string {
	return fmt.Sprintf("load1=%.2f mem_avail_kb=%d", s.Load1, s.MemAvailableKB)
}
