//go:build linux

package bootid

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func scan() (string, error) {
	p := "/proc/sys/kernel/random/boot_id"
	b, err := os.ReadFile(p)
	if err == nil {
		s := string(b)
		s = strings.TrimRight(s, "\n\r")
		return s, nil
	}
	p = "/proc/stat"
	file, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer file.Close()
	s := bufio.NewScanner(file)
	for s.Scan() {
		lineFields := strings.Fields(s.Text())
		if len(lineFields) == 2 && lineFields[0] == "btime" {
			return lineFields[1], nil
		}
	}
	return "", fmt.Errorf("unable to format a boot id from /proc/sys/kernel/random/boot_id nor /proc/stat")
}
