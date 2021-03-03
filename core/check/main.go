package check

import (
	"encoding/json"
	"fmt"
	"os"
)

type (
	// Interface exposes what can be done with a check
	Interface interface {
		Check() ([]*Result, error)
	}

	// Type is the check type
	Type struct {
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

func (r Type) String() string {
	return fmt.Sprintf("<Check %s>", r.Name)
}

// Check returns a result list
func Check(r Interface) error {
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
