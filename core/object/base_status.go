package object

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// ActionOptionsStatus is the options of the Start object method.
type ActionOptionsStatus struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsRefresh
	ObjectStatus Status
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsStatus) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsRefresh.init(cmd)
}

func (t *Base) statusFile() string {
	return filepath.Join(t.varDir(), "status.json")
}

// Status returns the service status dataset
func (t *Base) Status(options ActionOptionsStatus) (InstanceStatus, error) {
	var (
		data InstanceStatus
		err  error
	)
	if options.Refresh {
		data, err = t.statusEval()
		if err != nil {
			return data, err
		}
	} else {
		data, err = t.statusLoad()
		if err != nil {
			data, err = t.statusEval()
		}
		if err != nil {
			return data, err
		}
	}
	return data, nil
}

func (t *Base) statusEval() (InstanceStatus, error) {
	data := InstanceStatus{}
	err := errors.New("") // Simulate err to avoid dumping over status.json
	if err != nil {
		return data, err
	}
	t.statusDump(data)
	return data, nil
}

func (t *Base) statusDump(data InstanceStatus) error {
	p := t.statusFile()
	tmp := "." + p + ".swp"
	jsonFile, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(jsonFile)
	err = enc.Encode(data)
	if err != nil {
		t.log.Error().
			Str("file", tmp).
			Err(err).
			Msg("")
		return err
	}
	return os.Rename(tmp, p)
}

func (t *Base) statusLoad() (InstanceStatus, error) {
	data := InstanceStatus{}
	p := t.statusFile()
	jsonFile, err := os.Open(p)
	if err != nil {
		return data, err
	}
	defer jsonFile.Close()
	dec := json.NewDecoder(jsonFile)
	err = dec.Decode(&data)
	if err != nil {
		t.log.Error().
			Str("file", p).
			Err(err).
			Msg("")
		return data, err
	}
	return data, err
}
