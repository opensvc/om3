package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var (
	tgetProp  = CompZprops{}.getProp
	zpropZbin string
)

type (
	CompZprops struct {
		*Obj
	}
	CompZprop struct {
		Name  string `json:"name"`
		Prop  string `json:"prop"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

func (t *CompZprops) add(s string) error {
	var data []CompZprop
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Name == "" {
			err := fmt.Errorf("name should be in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if rule.Prop == "" {
			err := fmt.Errorf("prop should be in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=") {
			err := fmt.Errorf("op should equal to =, >= or <= dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if rule.Value == nil {
			err := fmt.Errorf("value should be in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		_, okString := rule.Value.(string)
		_, okFloat64 := rule.Value.(float64)
		if !(okString || okFloat64) {
			err := fmt.Errorf("value should be a string or an int in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		if okString && (rule.Op == ">=" || rule.Op == "<=") {
			err := fmt.Errorf("op should be = if value is a string in dict: %s", s)
			t.Errorf("%s\n", err)
			return err
		}
		t.Obj.Add(rule)
	}
	return nil
}

func (t CompZprops) getProp(rule CompZprop) (string, error) {
	cmd := exec.Command(zpropZbin, "get", rule.Prop, rule.Name, "-Ho", "value")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}
	return strings.Trim(string(output), "\n"), nil
}

func (t CompZprops) checkZbin() ExitCode {
	_, err := exec.LookPath(zpropZbin)
	if err != nil {
		t.Errorf("%s not found\n", zpropZbin)
		return ExitNok
	}
	return ExitOk
}

func (t CompZprops) checkOperator(rule CompZprop) ExitCode {
	strCurrentValue, err := tgetProp(rule)
	var float64CurrentValue float64
	var strTargetValue string
	switch rule.Value.(type) {
	case string:
		strTargetValue = rule.Value.(string)
	default:
		strTargetValue = strconv.FormatFloat(rule.Value.(float64), 'f', -1, 64)
	}
	if strCurrentValue == "-" && !(rule.Op == "=" && strTargetValue == "-") {
		t.VerboseErrorf("property %s current value is not %s %s (the value is not set)\n", rule.Prop, rule.Op, strTargetValue)
		return ExitNok
	}
	if _, ok := rule.Value.(float64); ok {
		float64CurrentValue, err = strconv.ParseFloat(strCurrentValue, 64)
		if err != nil {
			t.Errorf("error when trying to convert in float64 %s: %s\n", strCurrentValue, err)
			return ExitNok
		}
	}
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	isCorrect := false
	switch rule.Op {
	case "=":
		if strCurrentValue == strTargetValue {
			isCorrect = true
		}
	case "<=":
		if float64CurrentValue <= rule.Value.(float64) {
			isCorrect = true
		}
	default:
		if float64CurrentValue >= rule.Value.(float64) {
			isCorrect = true
		}
	}
	if isCorrect {
		t.VerboseInfof("property %s current value %s is %s %s, on target\n", rule.Prop, strCurrentValue, rule.Op, strTargetValue)
		return ExitOk
	}
	t.VerboseErrorf("property %s current value %s is not %s %s\n", rule.Prop, strCurrentValue, rule.Op, strTargetValue)
	return ExitNok
}

func (t CompZprops) fixRule(rule CompZprop) ExitCode {
	if fVal, ok := rule.Value.(float64); ok {
		rule.Value = strconv.FormatFloat(fVal, 'f', -1, 64)
	}
	cmd := exec.Command(zpropZbin, "set", rule.Prop+"="+rule.Value.(string), rule.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s: %s\n", err, output)
		return ExitNok
	}
	t.Infof("set the property %s of the %s %s to %s\n", rule.Prop, zpropZbin, rule.Name, rule.Value.(string))
	return ExitOk
}

func (t CompZprops) Check() ExitCode {
	t.SetVerbose(true)
	if t.checkZbin() == ExitNok {
		return ExitNok
	}
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompZprop)
		e = e.Merge(t.checkOperator(rule))
	}
	return e
}

func (t CompZprops) Fix() ExitCode {
	t.SetVerbose(false)
	if t.checkZbin() == ExitNok {
		return ExitNok
	}
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompZprop)
		if t.checkOperator(rule) == ExitNok {
			e = e.Merge(t.fixRule(rule))
		}
	}
	return e
}

func (t CompZprops) Fixable() ExitCode {
	return ExitNotApplicable
}
