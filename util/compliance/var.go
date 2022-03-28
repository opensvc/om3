package compliance

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type (
	Var [2]interface{}
)

const (
	VarPrefix = "OSVC_COMP_"
)

var (
	space = regexp.MustCompile(`\s+`)
)

func (t Var) String() string {
	var s string
	switch v := t.Value().(type) {
	case nil:
		s = "None"
	case time.Time:
		s = v.Format("2006-01-02 15:04:05")
	case string:
		if len(v) > 0 && space.MatchString(v) {
			s = fmt.Sprintf("\"%s\"", v)
		} else {
			s = fmt.Sprint(v)
		}
	default:
		s = fmt.Sprint(v)
	}
	return fmt.Sprintf("%s=%s", t.Name(), s)
}

func (t Var) Name() string {
	s, _ := t[0].(string)
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return VarPrefix + s
}

func (t Var) Value() interface{} {
	return t[1]
}
