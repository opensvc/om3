package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type (
	CompKeyvals struct {
		*Obj
	}
	CompKeyval struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value any    `json:"value"`
		path  string
	}
)

var (
	keyValResetMap     = map[string]int{}
	keyValpath         string
	keyValFileFmtCache []byte
	keyvalValidityMap  = map[string]string{}
	compKeyvalInfo     = ObjInfo{
		DefaultPrefix: "OSVC_COMP_KEYVAL_",
		ExampleValue: struct {
			path any
			keys any
		}{
			path: "/etc/ssh/sshd_config",
			keys: []CompKeyval{{
				Key:   "PermitRootLogin",
				Op:    "=",
				Value: "yes",
			}, {
				Key:   "PermitRootLogin",
				Op:    "reset",
				Value: "",
			},
			},
		},
		Description: `* Setup and verify keys in "key value" formatted configuration file.
* Example files: sshd_config, ssh_config, ntp.conf, ...
`,
		FormDefinition: `Desc: |
  A rule to set a list of parameters in simple keyword/value configuration file format. Current values can be checked as set or unset, or superior/inferior to their target value. By default, this object appends keyword/values not found, potentially creating duplicates. The 'reset' operator can be used to avoid such duplicates.
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: keyval
Inputs:
  -
    Id: key
    Label: Key
    DisplayModeTrim: 64
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help:
  -
    Id: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Default: "="
    Candidates:
      - reset
      - unset
      - "="
      - ">="
      - "<="
      - "IN"
    Help: The comparison operator to use to check the parameter current value. The 'reset' operator can be used to avoid duplicate occurrence of the same keyword (insert a key reset after the last key set to mark any additional key found in the original file to be removed). The IN operator verifies the current value is one of the target list member. On fix, if the check is in error, it sets the first target list member. A "IN" operator value must be a JSON formatted list.
  -
    Id: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string or integer
    Help: The configuration file parameter target value.
`,
	}
)

func init() {
	m["keyval_with_fpath"] = NewCompKeyvals
}

func NewCompKeyvals() interface{} {
	return &CompKeyvals{
		Obj: NewObj(),
	}
}

func (t *CompKeyvals) Add(s string) error {
	dataPath := struct {
		Path string       `json:"path"`
		Keys []CompKeyval `json:"keys"`
	}{}
	if err := json.Unmarshal([]byte(s), &dataPath); err != nil {
		return err
	}
	if dataPath.Path == "" {
		err := fmt.Errorf("path should be in the dict: %s", s)
		t.Errorf("%s\n", err)
		return err
	}
	keyValpath = dataPath.Path
	for _, rule := range dataPath.Keys {
		rule.path = keyValpath
		if rule.Key == "" {
			err := fmt.Errorf("key should be in the dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if rule.Op == "" {
			rule.Op = "="
		}
		if rule.Value == nil && (rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "IN") {
			err := fmt.Errorf("value should be set in the dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if !(rule.Op == "reset" || rule.Op == "unset" || rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "IN") {
			err := fmt.Errorf("op should be in: reset, unset, =, >=, <=, IN in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if rule.Op != "unset" {
			switch rule.Value.(type) {
			case string:
			//skip
			case float64:
			//skip
			default:
				if rule.Op != "IN" {
					err := fmt.Errorf("value should be an int or a string in dict: %s", s)
					t.Errorf("%s\n", err)
					return err
				}
				if _, ok := rule.Value.([]any); !ok {
					err := fmt.Errorf("value should be a list in dict: %s", s)
					t.Errorf("%s\n", err)
					return err
				}
				if len(rule.Value.([]any)) == 0 {
					err := fmt.Errorf("list should not be empty in dict: %s", s)
					t.Errorf("%s\n", err)
					return err
				}
				for _, val := range rule.Value.([]any) {
					if _, ok := val.(float64); ok {
						continue
					}
					if _, ok := val.(string); ok {
						continue
					}
					err := fmt.Errorf("the values in value should be string or int in dict: %s", s)
					t.Errorf("%s\n", err)
					return err
				}
			}
		}

		if _, ok := rule.Value.(float64); !ok && (rule.Op == "<=" || rule.Op == ">=") {
			err := fmt.Errorf("can't use >= and <= if the value is not an int in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}

		switch rule.Op {
		case "unset":
			if keyvalValidityMap[rule.Key] == "set" {
				keyvalValidityMap[rule.Key] = "invalid"
			} else if keyvalValidityMap[rule.Key] != "invalid" {
				keyvalValidityMap[rule.Key] = "unset"
			}
		case "reset":
			keyValResetMap[rule.Key] = 0
		default:
			if keyvalValidityMap[rule.Key] == "unset" {
				keyvalValidityMap[rule.Key] = "invalid"
			} else if keyvalValidityMap[rule.Key] != "invalid" {
				keyvalValidityMap[rule.Key] = "set"
			}
		}
		t.Obj.Add(rule)
	}
	t.filterRules()
	keyValpath = ""
	return nil
}

func (t *CompKeyvals) filterRules() {
	blacklisted := map[string]any{}
	newRules := []interface{}{}
	for _, rule := range t.Rules() {
		if keyvalValidityMap[rule.(CompKeyval).Key] != "invalid" {
			newRules = append(newRules, rule)
		} else if _, ok := blacklisted[rule.(CompKeyval).Key]; !ok {
			blacklisted[rule.(CompKeyval).Key] = nil
			t.Errorf("the key %s generate some conflicts (asking for a comparison operator and unset at the same time) the key is now blacklisted\n", rule.(CompKeyval).Key)
		}
	}
	t.Obj.rules = newRules
}

func (t CompKeyvals) loadCache() error {
	var err error
	keyValFileFmtCache, err = os.ReadFile(keyValpath)
	return err
}

func (t CompKeyvals) getValues(key string) []string {
	values := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(keyValFileFmtCache))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		splitLine := strings.SplitN(line, " ", 2)
		if len(splitLine) != 2 {
			continue
		}
		if splitLine[0] == key {
			values = append(values, strings.TrimSpace(splitLine[1]))
		}
	}
	return values
}

func (t CompKeyvals) checkOperator(rule CompKeyval, valuesFromFile []string) int {
	switch rule.Op {
	case "=":
		for i, value := range valuesFromFile {
			if _, ok := rule.Value.(float64); ok {
				floatValue, err := strconv.ParseFloat(value, 64)
				if err != nil {
					continue
				}
				if rule.Value.(float64) == floatValue {
					return i
				}
			} else {
				if rule.Value.(string) == value {
					return i
				}
			}
		}
	case ">=":
		for i, value := range valuesFromFile {
			floatValueFromFile, err := strconv.ParseFloat(value, 64)
			if err != nil {
				continue
			}
			if floatValueFromFile >= rule.Value.(float64) {
				return i
			}
		}
	case "<=":
		for i, value := range valuesFromFile {
			floatValueFromFile, err := strconv.ParseFloat(value, 64)
			if err != nil {
				continue
			}
			if floatValueFromFile <= rule.Value.(float64) {
				return i
			}
		}
	case "IN":
		for _, val := range rule.Value.([]any) {
			for i, valueFromFile := range valuesFromFile {
				if _, ok := val.(float64); ok {
					floatValue, err := strconv.ParseFloat(valueFromFile, 64)
					if err != nil {
						continue
					}
					if val.(float64) == floatValue {
						return i
					}
				} else {
					if val.(string) == valueFromFile {
						return i
					}
				}
			}
		}
	}
	return -1
}

func (t CompKeyvals) checkReset(rule CompKeyval) ExitCode {
	if rule.Op != "reset" {
		return ExitOk
	}
	valuesFromFile := t.getValues(rule.Key)
	if len(valuesFromFile) != keyValResetMap[rule.Key] {
		t.VerboseErrorf("%s: %s is set %d times, should be set %d times\n", keyValpath, rule.Key, len(valuesFromFile), keyValResetMap[rule.Key])
		return ExitNok
	}
	t.VerboseInfof("%s: %s is set %d times, on target\n", keyValpath, rule.Key, keyValResetMap[rule.Key])
	return ExitOk
}

func (t CompKeyvals) checkNoReset(rule CompKeyval) ExitCode {
	if rule.Op == "reset" {
		return ExitOk
	}
	valuesFromFile := t.getValues(rule.Key)
	if rule.Op == "unset" {
		if len(valuesFromFile) > 0 {
			t.VerboseErrorf("%s: %s is set and should not be set\n", keyValpath, rule.Key)
			return ExitNok
		}
		t.VerboseInfof("%s: %s is not set and should not be set\n", keyValpath, rule.Key)
		return ExitOk
	}
	if _, ok := keyValResetMap[rule.Key]; ok {
		keyValResetMap[rule.Key]++
	}
	if len(valuesFromFile) < 1 {
		t.VerboseErrorf("%s: %s is unset and should be set\n", keyValpath, rule.Key)
		return ExitNok
	}
	switch rule.Op {
	case "=":
		i := t.checkOperator(rule, valuesFromFile)
		if i == -1 {
			if _, ok := rule.Value.(float64); ok {
				t.VerboseErrorf("%s: %s has the following values: %s and one of these values should be equal to %d\n", keyValpath, rule.Key, valuesFromFile, int(rule.Value.(float64)))
			} else {
				t.VerboseErrorf("%s: %s has the following values: %s and one of these values should be equal to %s\n", keyValpath, rule.Key, valuesFromFile, rule.Value)
			}
			return ExitNok
		}
		if _, ok := rule.Value.(float64); ok {
			t.VerboseInfof("%s: %s has the following values: %s and one of these values should be equal to %d\n", keyValpath, rule.Key, valuesFromFile, int(rule.Value.(float64)))
		} else {
			t.VerboseInfof("%s: %s has the following values: %s and one of these values should be equal to %s\n", keyValpath, rule.Key, valuesFromFile, rule.Value)
		}
		return ExitOk
	case ">=":
		i := t.checkOperator(rule, valuesFromFile)
		if i == -1 {
			t.VerboseErrorf("%s: %s has the following values: %s and one of these values should be greater than or equal to %d\n", keyValpath, rule.Key, valuesFromFile, int(rule.Value.(float64)))
			return ExitNok
		}
		t.VerboseInfof("%s: %s has the following values: %s and one of these values should be greater than or equal to %d\n", keyValpath, rule.Key, valuesFromFile, int(rule.Value.(float64)))
		return ExitOk
	case "<=":
		i := t.checkOperator(rule, valuesFromFile)
		if i == -1 {
			t.VerboseErrorf("%s: %s has the following values: %s and one of these values should be less than or equal to %d\n", keyValpath, rule.Key, valuesFromFile, int(rule.Value.(float64)))
			return ExitNok
		}
		t.VerboseInfof("%s: %s has the following values: %s and one of these values should be less than or equal to %d\n", keyValpath, rule.Key, valuesFromFile, int(rule.Value.(float64)))
		return ExitOk
	default:
		i := t.checkOperator(rule, valuesFromFile)
		if i == -1 {
			t.VerboseErrorf("%s: %s has the following values: %s and one of these values should be in %s\n", keyValpath, rule.Key, valuesFromFile, t.formatList(rule.Value.([]any)))
			return ExitNok
		}
		t.VerboseInfof("%s: %s has the following values: %s and one of these values should be in %s\n", keyValpath, rule.Key, valuesFromFile, t.formatList(rule.Value.([]any)))
		return ExitOk
	}
}

func (t CompKeyvals) formatList(list []any) []string {
	newList := []string{}
	for _, val := range list {
		if fVal, ok := val.(float64); ok {
			newList = append(newList, strconv.FormatFloat(fVal, 'f', -1, 64))
		} else {
			newList = append(newList, val.(string))
		}
	}
	return newList
}

func (t CompKeyvals) fixRuleNoReset(rule CompKeyval) ExitCode {
	if err := t.loadCache(); err != nil {
		t.Errorf("%s\n", err)
	}
	if t.checkNoReset(rule) == ExitOk {
		return ExitOk
	}
	switch rule.Op {
	case "unset":
		return t.fixUnset(rule)
	default:
		return t.fixOperator(rule)
	}
}

func (t CompKeyvals) fixReset(rule CompKeyval) ExitCode {
	if err := t.loadCache(); err != nil {
		t.Errorf("%s\n", err)
	}
	if t.checkReset(rule) == ExitOk {
		return ExitOk
	}
	resetRules := []CompKeyval{}
	for i := 0; i < len(t.rules) && len(resetRules) < keyValResetMap[rule.Key]; i++ {
		ruleToAdd := t.rules[i].(CompKeyval)
		if ruleToAdd.Key == rule.Key && ruleToAdd.Op != "reset" && ruleToAdd.Op != "unset" {
			resetRules = append(resetRules, ruleToAdd)
		}
	}
	oldConfigFileStat, err := os.Stat(keyValpath)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newFile, err := os.CreateTemp(filepath.Dir(keyValpath), "newFile")
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newConfigFilePath := newFile.Name()
	oldConfigFile, err := os.Open(keyValpath)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	scanner := bufio.NewScanner(oldConfigFile)
	keyToResetCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.SplitN(line, " ", 2)[0] == rule.Key {
			if keyToResetCount < len(resetRules) {
				var stringValue string
				if resetRules[keyToResetCount].Op == "IN" {
					if fVal, ok := resetRules[keyToResetCount].Value.([]any)[0].(float64); ok {
						stringValue = strconv.FormatFloat(fVal, 'f', -1, 64)
					} else {
						stringValue = resetRules[keyToResetCount].Value.([]any)[0].(string)
					}
				} else {
					if fVal, ok := resetRules[keyToResetCount].Value.(float64); ok {
						stringValue = strconv.FormatFloat(fVal, 'f', -1, 64)
					} else {
						stringValue = resetRules[keyToResetCount].Value.(string)
					}
				}
				if _, err = newFile.Write([]byte(rule.Key + " " + stringValue + "\n")); err != nil {
					t.Errorf("%s\n", err)
					return ExitNok
				}
				keyToResetCount++
			}
		} else {
			if _, err = newFile.Write([]byte(line + "\n")); err != nil {
				t.Errorf("%s", err)
				return ExitNok
			}
		}
	}
	err = newFile.Close()
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if err = os.Chmod(newConfigFilePath, oldConfigFileStat.Mode()); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if sysInfos := oldConfigFileStat.Sys(); sysInfos != nil {
		if err = os.Chown(newConfigFilePath, int(sysInfos.(*syscall.Stat_t).Uid), int(sysInfos.(*syscall.Stat_t).Gid)); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	} else {
		t.Errorf("can't change the owner of the file %s\n", newConfigFilePath)
		return ExitNok
	}
	if err = oldConfigFile.Close(); err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}
	err = os.Rename(newConfigFilePath, keyValpath)
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Infof("reset all the old values of the key %s\n", rule.Key)
	return ExitOk
}

func (t CompKeyvals) fixOperator(rule CompKeyval) ExitCode {
	oldConfigFileStat, err := os.Stat(keyValpath)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newFile, err := os.CreateTemp(filepath.Dir(keyValpath), "newFile")
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newConfigFilePath := newFile.Name()
	newFileFmt := keyValFileFmtCache
	if len(newFileFmt) > 0 {
		if newFileFmt[len(newFileFmt)-1] != '\n' {
			newFileFmt = append(newFileFmt, '\n')
		}
	}
	var stringValue string
	if rule.Op == "IN" {
		if fVal, ok := rule.Value.([]any)[0].(float64); ok {
			stringValue = strconv.FormatFloat(fVal, 'f', -1, 64)
		} else {
			stringValue = rule.Value.([]any)[0].(string)
		}
	} else {
		if fVal, ok := rule.Value.(float64); ok {
			stringValue = strconv.FormatFloat(fVal, 'f', -1, 64)
		} else {
			stringValue = rule.Value.(string)
		}
	}
	newFileFmt = append(newFileFmt, []byte(rule.Key+" "+stringValue+"\n")...)
	if _, err = newFile.Write(newFileFmt); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if err = newFile.Close(); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if err = os.Chmod(newConfigFilePath, oldConfigFileStat.Mode()); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if sysInfos := oldConfigFileStat.Sys(); sysInfos != nil {
		if err = os.Chown(newConfigFilePath, int(sysInfos.(*syscall.Stat_t).Uid), int(sysInfos.(*syscall.Stat_t).Gid)); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	} else {
		t.Errorf("can't change the owner of the file %s\n", newConfigFilePath)
		return ExitNok
	}
	if err = os.Rename(newConfigFilePath, keyValpath); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	t.Infof("adding the key %s with value %s in file %s\n", rule.Key, stringValue, keyValpath)
	return ExitOk
}

func (t CompKeyvals) fixUnset(rule CompKeyval) ExitCode {
	oldConfigFileStat, err := os.Stat(keyValpath)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newConfigFile, err := os.CreateTemp(filepath.Dir(keyValpath), "newFile")
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newConfigFilePath := newConfigFile.Name()
	scanner := bufio.NewScanner(bytes.NewReader(keyValFileFmtCache))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.SplitN(line, " ", 2)[0] != rule.Key {
			if _, err = newConfigFile.Write([]byte(line)); err != nil {
				t.Errorf("%s\n", err)
				return ExitNok
			}
		}
	}
	err = newConfigFile.Close()
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if err = os.Chmod(newConfigFilePath, oldConfigFileStat.Mode()); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if sysInfos := oldConfigFileStat.Sys(); sysInfos != nil {
		if err = os.Chown(newConfigFilePath, int(sysInfos.(*syscall.Stat_t).Uid), int(sysInfos.(*syscall.Stat_t).Gid)); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	} else {
		t.Errorf("can't change the owner of the file %s\n", newConfigFilePath)
		return ExitNok
	}
	err = os.Rename(newConfigFilePath, keyValpath)
	t.Infof("unset the key %s in file %s\n", rule.Key, keyValpath)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	t.Infof("unset the key %s in file %s\n", rule.Key, keyValpath)
	return ExitOk
}

func (t CompKeyvals) checkPathExistence() ExitCode {
	_, err := os.Stat(keyValpath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Errorf("the file %s does not exist\n", keyValpath)
			return ExitNok
		}
		t.Errorf("%s\n", err)
		return ExitNok
	}
	return ExitOk
}

func (t CompKeyvals) updateFilePath(rule CompKeyval) ExitCode {
	if keyValpath != rule.path {
		keyValpath = rule.path
		if t.checkPathExistence() == ExitNok {
			return ExitNok
		}
		if err := t.loadCache(); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	}
	return ExitOk
}

func (t CompKeyvals) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompKeyval)
		if t.updateFilePath(rule) == ExitNok {
			return ExitNok
		}
		o := t.checkNoReset(rule)
		e = e.Merge(o)
	}
	for _, i := range t.Rules() {
		rule := i.(CompKeyval)
		if t.updateFilePath(rule) == ExitNok {
			return ExitNok
		}
		o := t.checkReset(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompKeyvals) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompKeyval)
		if t.updateFilePath(rule) == ExitNok {
			return ExitNok
		}
		e = e.Merge(t.fixRuleNoReset(rule))
	}
	for _, i := range t.Rules() {
		rule := i.(CompKeyval)
		if t.updateFilePath(rule) == ExitNok {
			return ExitNok
		}
		e = e.Merge(t.fixReset(rule))
	}
	return e
}

func (t CompKeyvals) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompKeyvals) Info() ObjInfo {
	return compKeyvalInfo
}
