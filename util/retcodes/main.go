package retcodes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/status"
)

var (
	baseExitToStatusMap = map[int]status.T{
		0: status.Up,
		1: status.Down,
	}
	stringToStatus = map[string]status.T{
		"up":    status.Up,
		"down":  status.Down,
		"warn":  status.Warn,
		"n/a":   status.NotApplicable,
		"undef": status.Undef,
	}
)

type (
	RetCodes map[int]status.T
)

// Parse return exitCodeToStatus map
//
// invalid entry rules are dropped
func Parse(retCodes string) (RetCodes, error) {
	if len(retCodes) == 0 {
		return baseExitToStatusMap, nil
	}
	dropMessages := make([]string, 0)
	m := make(map[int]status.T)
	for _, rule := range strings.Fields(retCodes) {
		dropMessage := fmt.Sprintf("retcodes invalid rule '%v'", rule)
		ruleSplit := strings.Split(rule, ":")
		if len(ruleSplit) != 2 {
			dropMessages = append(dropMessages, dropMessage)
			continue
		}
		code, err := strconv.Atoi(ruleSplit[0])
		if err != nil {
			dropMessages = append(dropMessages, dropMessage)
			continue
		}
		statusValue, ok := stringToStatus[ruleSplit[1]]
		if !ok {
			dropMessages = append(dropMessages, dropMessage)
			continue
		}
		m[code] = statusValue
	}
	var err error
	if len(dropMessages) > 0 {
		err = fmt.Errorf("%s", strings.Join(dropMessages, "\n"))
	}
	if len(m) == 0 {
		return baseExitToStatusMap, err
	}
	return m, err
}

func (t RetCodes) Status(exitCode int) status.T {
	if s, ok := t[exitCode]; ok {
		return s
	}
	return status.Warn
}
