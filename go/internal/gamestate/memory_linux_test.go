//go:build linux

package gamestate

import (
	"os"
	"strconv"
	"strings"
)

func peakRSS() uint64 {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			f := strings.Fields(line)
			if len(f) > 1 {
				v, _ := strconv.ParseUint(f[1], 10, 64)
				return v * 1024
			}
		}
	}
	return 0
}
