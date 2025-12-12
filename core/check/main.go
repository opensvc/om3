package check

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
)

type (
	// Checker exposes what can be done with a check
	Checker interface {
		Check(objs []interface{}) (*ResultSet, error)
	}

	// T is the check type
	T struct {
		Name string
	}

	// Result is the structure eventually collected for aggregation.
	Result struct {
		DriverGroup string `json:"type"`
		DriverName  string `json:"driver"`
		Path        string `json:"path"`
		Instance    string `json:"instance"`
		Unit        string `json:"unit"`
		Value       int64  `json:"value"`
	}

	header interface {
		Head() string
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
)

var checkers = make([]Checker, 0)

// UnRegisterAll unregister all registered checkers
func UnRegisterAll() {
	checkers = make([]Checker, 0)
}

func Register(c Checker) {
	checkers = append(checkers, c)
}

func (r T) String() string {
	return fmt.Sprintf("<Check %s>", r.Name)
}

// Check returns a result list
func Check(r Checker, objs []interface{}) error {
	data, err := r.Check(objs)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(data)
	return nil
}

// ObjectPathClaimingDir returns the first object using the directory
func ObjectPathClaimingDir(p string, objs []interface{}) string {
	for _, obj := range objs {
		h, ok := obj.(header)
		if !ok {
			continue
		}
		if h.Head() == p {
			return fmt.Sprint(obj)
		}
	}
	for _, obj := range objs {
		b, ok := obj.(resourceLister)
		if !ok {
			continue
		}
		for _, r := range b.Resources() {
			h, ok := r.(header)
			if !ok {
				continue
			}
			if v, err := r.Provisioned(); err != nil {
				continue
			} else if v == provisioned.False {
				continue
			}
			if h.Head() == p {
				return fmt.Sprint(obj)
			}
		}
	}
	return ""
}
