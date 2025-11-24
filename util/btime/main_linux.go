//go:build linux

package btime

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func bootTime() (uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		fmt.Println("a")
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ligne := scanner.Text()
		if strings.HasPrefix(ligne, "btime") {
			var btime uint64
			_, err := fmt.Sscanf(ligne, "btime %d", &btime)
			if err != nil {
				return 0, err
			}
			return btime, nil
		}
	}
	return 0, fmt.Errorf("btime not found in /proc/stat")
}
