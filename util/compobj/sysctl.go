package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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

func (t CompSysctls) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSysctl)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

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
		splitLine := strings.Split(line, "=")
		if len(splitLine) != 2 {
			continue
		}
		if strings.TrimSpace(splitLine[0]) == rule.Key {
			return strings.Fields(splitLine[1]), nil
		}
	}
	return nil, nil
}

func (t CompSysctls) modifyKeyInConfFile(rule CompSysctl) (bool, error) {
	changeDone := false
	configFileOldContent, err := os.ReadFile("/etc/sysctl.conf")
	configFileNewContent := []byte{}
	if err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(configFileOldContent))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		splitLine := strings.Split(line, "=")
		if len(splitLine) != 2 {
			continue
		}
		if strings.TrimSpace(splitLine[0]) == rule.Key {
			if changeDone {
				t.Infof("sysctl: remove redundant key %s", rule.Key)
				continue
			}
			values := strings.Fields(splitLine[1])
			if len(values) > *rule.Index {
				if _, ok := rule.Value.(string); !ok {
					rule.Value = strconv.FormatFloat(rule.Value.(float64), 'f', -1, 64)
				}
				values[*rule.Index] = rule.Value.(string)
				changeDone = true
			}
			line = strings.TrimSpace(splitLine[0]) + " ="
			for _, value := range values {
				line += " " + value
			}
		}
		configFileOldContent = append(configFileNewContent, []byte(line)...)
		configFileNewContent = append(configFileNewContent, byte('\n'))
	}
	if !changeDone {
		return changeDone, nil
	}
	f, err := os.Create("/etc/sysctl.conf")
	if err != nil {
		t.Errorf("can't open the file %s in write mode :%s\n", "/etc/sysctl.conf", err)
		return false, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Errorf("error when trying to close file %s :%s\n", "/etc/sysctl.conf", err)
		}
	}()
	_, err = f.Write(configFileNewContent)
	if err != nil {
		t.Errorf("error when trying to write in %s :%s\n", "/etc/sysctl.conf", err)
		return false, err
	}
	return changeDone, nil
}

func (t CompSysctls) addKeyInConfFile(rule CompSysctl) error {
	values, err := t.getValues(rule, true)
	if err != nil {
		return err
	}
	if values == nil {
		return fmt.Errorf("error can't find the key %s in the list of kernel parameter (/etc/sysctl.conf and live", rule.Key)
	}
	if len(values) < *rule.Index {
		return fmt.Errorf("can't modify the key %s index %d out of range", rule.Key, *rule.Index)
	}
	if _, ok := rule.Value.(string); !ok {
		rule.Value = strconv.FormatFloat(rule.Value.(float64), 'f', -1, 64)
	}
	values[*rule.Index] = rule.Value.(string)
	lineToAdd := rule.Key
	for _, value := range values {
		lineToAdd += " " + value
	}
	lineToAdd += "\n"
	f, err := os.OpenFile("/etc/sysctl.conf", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Errorf("error when trying to close file %s :%s\n", "/etc/sysctl.conf", err)
		}
	}()
	_, err = f.Write([]byte(lineToAdd))
	if err != nil {
		return err
	}
	return nil
}

func (t CompSysctls) fixRule(rule CompSysctl) ExitCode {
	if t.checkRule(rule) == ExitNok {
		changeDone, err := t.modifyKeyInConfFile(rule)
		if err != nil {
			t.Errorf("error when trying to modify /etc/sysctl.conf :", err)
			return ExitNok
		}
		if !changeDone {
			t.Infof("did not find key in /etc/sysctl.conf, trying to read live parameters and to add the new parameters in /etc/sysctl.conf")
			err = t.addKeyInConfFile(rule)
			if err != nil {
				t.Errorf("%s", err)
				return ExitNok
			}
		}
	}
	return ExitOk
}

func (t CompSysctls) Fix() ExitCode {
	t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompSysctl)
		if e := t.fixRule(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}

func (t CompSysctls) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompSysctls) Info() ObjInfo {
	return compSysctlInfo
}
