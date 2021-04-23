// Package capabilities manage node capabilities list
//
package capabilities

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"runtime"
	"sort"
)

type (
	// T is the capabilities structure.
	T struct {
		path     string
		scanners []Scanner
		caps     []string
	}

	// Scanner is the interface for scanner capabilities register candidates
	// (driver or node)
	Scanner interface {
		Scan() ([]string, error)
	}
)

var (
	// ErrorNeedScan error mean capabilities needs scan
	ErrorNeedScan = errors.New("capabilities not yet scanned")
)

// New return capabilities object that use path as backend file
func New(path string) *T {
	return &T{path: path, scanners: []Scanner{}, caps: []string{}}
}

// Init fetch existing capabilities from its backend file
func (t *T) Init() (err error) {
	var data []byte
	var caps []string
	if data, err = ioutil.ReadFile(t.path); err != nil {
		return ErrorNeedScan
	}
	if err = json.Unmarshal(data, &caps); err != nil {
		return ErrorNeedScan
	}
	t.caps = caps
	return
}

// Register add a new Scanner
func (t *T) Register(s Scanner) {
	t.scanners = append(t.scanners, s)
}

// Scan refresh capabilities from its scanners and update backend file
func (t *T) Scan() (err error) {
	caps := make([]string, 0)
	runChan := make(chan int, runtime.GOMAXPROCS(0))
	resChan := make(chan []string)
	for _, s := range t.scanners {
		go runScanner(s, runChan, resChan)
	}
	for range t.scanners {
		sCaps := <-resChan
		for _, c := range sCaps {
			caps = append(caps, c)
		}
	}
	sort.Strings(caps)
	t.caps = caps
	if data, err := json.Marshal(caps); err != nil {
		return err
	} else {
		return ioutil.WriteFile(t.path, data, 0600)
	}
}

// Has return true if capability cap exists
func (t *T) Has(cap string) bool {
	for _, c := range t.caps {
		if c == cap {
			return true
		}
	}
	return false
}

func runScanner(sc Scanner, running chan int, result chan []string) {
	running <- 1
	defer func() { <-running }()
	scannerCaps, err := sc.Scan()
	if err != nil {
		result <- []string{}
		return
	}
	result <- scannerCaps
}
