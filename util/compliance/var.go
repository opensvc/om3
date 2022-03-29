package compliance

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type (
	Var struct {
		Name  string
		Value interface{}
		Class string
	}
	Vars []Var
)

const (
	VarPrefix = "OSVC_COMP_"
)

var (
	space = regexp.MustCompile(`\s+`)
)

func NewVars() Vars {
	return make(Vars, 0)
}

// MarshalJSON marshals the data as a quoted json string
func (t Var) MarshalJSON() ([]byte, error) {
	pivot := [3]interface{}{
		t.Name,
		t.Value,
		t.Class,
	}
	return json.Marshal(pivot)
}

// UnmarshalJSON unmashals a quoted json string to value
func (t *Var) UnmarshalJSON(b []byte) error {
	pivot := [3]interface{}{}
	err := json.Unmarshal(b, &pivot)
	if err != nil {
		return err
	}
	if s, ok := pivot[0].(string); ok {
		t.Name = s
	} else {
		return errors.Errorf("invalid var name type: %+v", pivot[0])
	}
	t.Value = pivot[1]
	switch class := pivot[2].(type) {
	case string:
		t.Class = class
	case nil:
		t.Class = "raw"
	default:
		return errors.Errorf("invalid var class type: %+v", pivot[2])
	}
	return nil
}

func (t Var) String() string {
	return fmt.Sprintf("%s=%s", t.EnvName(), t.EnvValue())
}

func (t Var) EnvValue() string {
	switch v := t.Value.(type) {
	case nil:
		return "None"
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	case string:
		if len(v) > 0 && space.MatchString(v) {
			return fmt.Sprintf("\"%s\"", v)
		} else {
			return fmt.Sprint(v)
		}
	default:
		return fmt.Sprint(v)
	}
}

func (t Var) EnvName() string {
	s := strings.ToUpper(t.Name)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return VarPrefix + s
}

func (t Vars) LoadEnv() {
	for _, s := range os.Environ() {
		pair := strings.SplitN(s, "=", 2)
		t = append(t, Var{pair[0], pair[1], ""})
	}
}

func (t Vars) Env() Env {
	m := make(Env)
	for _, v := range t {
		m[v.EnvName()] = v.EnvValue()
	}
	return m
}
