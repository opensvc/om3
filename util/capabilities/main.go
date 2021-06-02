// Package capabilities maintain global capabilities
//
// Scan() use registered scanners functions to update capabilities list, then
// store this capabilities list on filesystem.
//
// Has(cap) use capabilities file to verify if cap exists
//
// A global list of registered scanner functions may be Registered to scanner
// list.
// Registered scanners are called to refresh capabilities during Scan()
//
package capabilities

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"runtime"
	"sort"

	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// scanner func may be registered by drivers or other components
	scanner func() ([]string, error)
)

var (
	// ErrorNeedScan error mean capabilities needs scan
	ErrorNeedScan = errors.New("capabilities not yet scanned")

	scanners []scanner
	caps     []string
)

// Register add new s scanner function to scanners list
func Register(s scanner) {
	scanners = append(scanners, s)
}

// Data return copy of capabilities
func Data() []string {
	return []string(cache())
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
	newCaps := make([]string, 0)
	runChan := make(chan int, runtime.GOMAXPROCS(0))
	resChan := make(chan []string)
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
func cache() []string {
	if caps != nil {
		return caps
	}
	newCaps, err := Load()
	if err != nil {
		caps = []string{}
		return caps
	}
	caps = newCaps
	return caps
}

func save(newCaps []string) error {
	if data, err := json.Marshal(newCaps); err != nil {
		return err
	} else {
		return ioutil.WriteFile(getPath(), data, 0600)
	}
}

// Load fetch existing capabilities from its backend file
func Load() (loadedCaps []string, err error) {
	var data []byte
	if data, err = ioutil.ReadFile(getPath()); err != nil {
		return loadedCaps, ErrorNeedScan
	}
	if err = json.Unmarshal(data, &loadedCaps); err != nil {
		return loadedCaps, ErrorNeedScan
	}
	return
}

func runScanner(sc scanner, running chan int, result chan []string) {
	running <- 1
	defer func() { <-running }()
	scannerCaps, err := sc()
	if err != nil {
		result <- []string{}
		return
	}
	result <- scannerCaps
}

func getPath() string {
	return rawconfig.Node.Paths.Var + "/capabilities.json"
}
