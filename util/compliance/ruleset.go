package compliance

import (
	"fmt"
	"net/http"
)

type (
	Rulesets map[string]Ruleset
	Ruleset  struct {
		Filter string
		Name   string
		Vars   Vars
	}
)

func (t T) GetRulesets() (Rulesets, error) {
	data, err := t.GetData([]string{})
	if err != nil {
		return nil, err
	}
	return data.Rsets, nil
}

func (t Ruleset) GetString(name string) string {
	if s, ok := t.Get(name).(string); ok {
		return s
	} else {
		return ""
	}
}

func (t Ruleset) Get(name string) interface{} {
	if t.Vars == nil {
		return nil
	}
	for _, v := range t.Vars {
		if v.Name == name {
			return v.Value
		}
	}
	return nil
}

func (t Ruleset) Render() string {
	buff := fmt.Sprintf(" %s", t.Name)
	if t.Filter != "" {
		buff += fmt.Sprintf(" (%s)", t.Filter)
	}
	buff += "\n"
	for _, v := range t.Vars {
		buff += fmt.Sprintf("  %s\n", v)
	}
	return buff
}

func (t Rulesets) Render() string {
	buff := "rulesets:\n"
	for _, rset := range t {
		buff += rset.Render()
	}
	return buff
}

func (t T) ListRulesets(filter string) ([]string, error) {
	return t.listNames("ruleset", filter)
}

func (t T) AttachRuleset(s string) error {
	return t.attachment(http.MethodPost, "ruleset", s)
}

func (t T) DetachRuleset(s string) error {
	return t.attachment(http.MethodDelete, "ruleset", s)
}
