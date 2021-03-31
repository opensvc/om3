package check

import (
	"encoding/json"
	"fmt"
	"os"
)

type (
	// Checker exposes what can be done with a check
	Checker interface {
		Check() (*ResultSet, error)
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
)

var checkers = make([]Checker, 0)

func Register(i interface{}) {
	c, ok := i.(Checker)
	if !ok {
		return
	}
	checkers = append(checkers, c)
}

func (r T) String() string {
	return fmt.Sprintf("<Check %s>", r.Name)
}

// Check returns a result list
func Check(r Checker) error {
	data, err := r.Check()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(data)
	return nil
}
