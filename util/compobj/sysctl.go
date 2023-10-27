package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type (
	CompSysctls struct {
		*Obj
	}
	CompSysctl struct {
		Key   string `json:"key"`
		Index *int   `json:"index"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

var (
	execSysctl     = func(key string) *exec.Cmd { return exec.Command("sysctl", key) }
	compSysctlInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_SYSCTL_",
		ExampleValue: CompSysctl{
			Key:   "vm.lowmem_reserve_ratio",
			Index: pti(1),
			Op:    "=",
			Value: 256,
		},
		Description: `* Verify a linux kernel parameter value is on target
* Live parameter value (sysctl executable)
* Persistent parameter value (/etc/sysctl.conf)
`,
		FormDefinition: `Desc: |
  A rule to set a list of Linux kernel parameters to be set in /etc/sysctl.conf. Current values can be checked as strictly equal, superior or equal, inferior or equal to their target value. Each field in a vectored value can be tuned independantly using the index key.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: sysctl

Inputs:
  -
    Id: key
    Label: Key
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The /etc/sysctl.conf parameter to check.

  -
    Id: index
    Label: Index
    DisplayModeLabel: idx
    LabelCss: action16
    Mandatory: Yes
    Default: 0
    Type: integer
    Help: The /etc/sysctl.conf parameter to check.

  -
    Id: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Default: "="
    Candidates:
      - "="
      - ">="
      - "<="
    Help: The comparison operator to use to check the parameter current value.

  -
    Id: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string or integer
    Help: The /etc/sysctl.conf parameter target value.
`,
	}
)

func init() {
	m["sysctl"] = NewCompSysctls
}

func NewCompSysctls() interface{} {
	return &CompSysctls{
		Obj: NewObj(),
	}
}

func (t *CompSysctls) Add(s string) error {
	var data []CompSysctl
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Key == "" {
			return fmt.Errorf("key is mandatory in dict : %s \n", s)
		}
		if rule.Index == nil {
			return fmt.Errorf("index is mandatory in dict : %s \n", s)
		}
		if *rule.Index < 0 {
			return fmt.Errorf("index must not be <0 in dict : %s \n", s)
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=") {
			return fmt.Errorf("operator must be =, >= or <= in dict : %s \n", s)
		}
		if rule.Value == nil {
			return fmt.Errorf("value must be in dict : %s \n", s)
		}
		t.Obj.Add(rule)
	}
	return nil
}

/*func (t CompSysctls) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSysctl)
		o := t.CheckSymlink(rule)
		e = e.Merge(o)
	}
	return e
}*/

func (t CompSysctls) checkRule(rule CompSysctl) ExitCode {
	values, err := t.getValues(rule, false)
	if err != nil {
		t.Errorf("error can't read in file /etc/sysctl.conf :%s\n", err)
		return ExitNok
	}
	if values == nil {
		values, err = t.getValues(rule, true)
	}
	if values == nil {
		t.Errorf("error can't find the key %s in the list of kernel parameter (/etc/sysctl.conf and live)\n", rule.Key)
		return ExitNok
	}
	if len(values) <= *rule.Index {
		t.VerboseInfof("index %d is out of range for key %s--> not ok\n", *rule.Index, rule.Key)
		return ExitNok
	}
	value := values[*rule.Index]
	switch rule.Value.(type) {
	case float64:
		newValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			t.Errorf("the value given is an int but can't convert the value %s found in /etc/sysctl.conf or in live for key %s\n", value, rule.Key)
			return ExitNok
		}
		if testOperator(newValue, rule.Value.(float64), rule.Op) {
			t.VerboseInfof("%s[%d] = %f target: %f operator: %s --> ok\n", rule.Key, *rule.Index, newValue, rule.Value, rule.Op)
			return ExitOk
		}
		t.VerboseInfof("%s[%d] = %f target: %f operator: %s --> not ok\n", rule.Key, *rule.Index, newValue, rule.Value, rule.Op)
		return ExitNok
	case string:
		if testOperator(value, rule.Value.(string), rule.Op) {
			t.VerboseInfof("%s[%d] = %s target: %s operator: %s --> ok\n", rule.Key, *rule.Index, value, rule.Value, rule.Op)
			return ExitOk
		}
		t.VerboseInfof("%s[%d] = %s target: %s operator: %s --> not ok\n", rule.Key, *rule.Index, value, rule.Value, rule.Op)
		return ExitNok
	default:
		t.Errorf("type of %s is not float64 or string\n", rule.Value)
		return ExitNok
	}
}

func testOperator[T float64 | string](value T, ruleValue T, operator string) bool {
	switch operator {
	case "=":
		return value == ruleValue
	case "<=":
		return value <= ruleValue
	case ">=":
		return value >= ruleValue
	default:
		return false
	}
}

func (t CompSysctls) getValues(rule CompSysctl, checkLive bool) ([]string, error) {
	var content []byte
	if checkLive {
		cmd := execSysctl(rule.Key)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("error can't read live key values :%w:%s", err, out)
		}
		content = out
	} else {
		var err error
		content, err = osReadFile("/etc/sysctl.conf")
		if err != nil {
			return nil, err
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		splitLine := strings.Fields(line)
		if len(splitLine) > 1 {
			if splitLine[0] == rule.Key {
				if splitLine[1] != "=" {
					continue
				}
				return splitLine[2:], nil
			}
		}
	}
	return nil, nil
}

/*func (t CompSysctls) Fix() ExitCode {
	t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		if e := t.FixSymlink(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}*/

func (t CompSysctls) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompSysctls) Info() ObjInfo {
	return compSysctlInfo
}
