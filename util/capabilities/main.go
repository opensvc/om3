// Package capabilities maintain global capabilities
//
// Scan() use registered scanners functions to update capabilities list, then
// store this capabilities list on filesystem.
//
// # Has(cap) use capabilities file to verify if cap exists
//
// A global list of registered scanner functions may be Registered to scanner
// list.
// Registered scanners are called to refresh capabilities during Scan()
package capabilities

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"sort"

	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// scanner func may be registered by drivers or other components
	scanner func() ([]string, error)

	// L is a list of capabilities expressed as strings
	L []string
)

var (
	// ErrorNeedScan error mean capabilities needs scan
	ErrorNeedScan = errors.New("capabilities not yet scanned")

	scanners []scanner
	caps     L
)

// Render is a human rendered for node capabilities
func (t L) Render() string {
	s := ""
	for _, c := range t {
		s = s + c + "\n"
	}
	return s
}

// Register add new s scanner function to scanners list
func Register(s scanner) {
	scanners = append(scanners, s)
}

// Data return copy of capabilities
func Data() L {
	return cache()
}

// Has return true if capability cap exists
func Has(cap string) bool {
	for _, c := range cache() {
		if c == cap {
			return true
		}
	}
	return false
}

// Scan refresh capabilities from the scanners function calls, then
// it update capabilities list stored on file system
func Scan() error {
	newCaps := make(L, 0)
	runChan := make(chan int, runtime.GOMAXPROCS(0))
	resChan := make(chan L)
	for _, s := range scanners {
		go runScanner(s, runChan, resChan)
	}
	for range scanners {
		sCaps := <-resChan
		for _, c := range sCaps {
			newCaps = append(newCaps, c)
		}
	}
	sort.Strings(newCaps)
	if err := save(newCaps); err != nil {
		return err
	}
	caps = newCaps
	return nil
}

// lazy loader for capabilities list stored on file system
func cache() L {
	if caps != nil {
		return caps
	}
	newCaps, err := Load()
	if err != nil {
		caps = L{}
		return caps
	}
	caps = newCaps
	return caps
}

func save(newCaps L) error {
	if data, err := json.Marshal(newCaps); err != nil {
		return err
	} else {
		return os.WriteFile(getPath(), data, 0600)
	}
}

// Load fetch existing capabilities from its backend file
func Load() (loadedCaps L, err error) {
	var data []byte
	if data, err = os.ReadFile(getPath()); err != nil {
		return loadedCaps, ErrorNeedScan
	}
	if err = json.Unmarshal(data, &loadedCaps); err != nil {
		return loadedCaps, ErrorNeedScan
	}
	return
}

func runScanner(sc scanner, running chan int, result chan L) {
	running <- 1
	defer func() { <-running }()
	scannerCaps, err := sc()
	if err != nil {
		result <- L{}
		return
	}
	result <- L(scannerCaps)
}

func getPath() string {
	return rawconfig.Paths.Var + "/capabilities.json"
}
