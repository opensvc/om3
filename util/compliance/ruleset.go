package compliance

import (
	"fmt"

	"opensvc.com/opensvc/util/hostname"
)

type (
	Rulesets map[string]Ruleset
	Ruleset  struct {
		Filter string
		Name   string
		Vars   []Var
	}
)

func (t T) GetRulesets() (Rulesets, error) {
	rulesets := make(Rulesets)
	err := t.collectorClient.CallFor(&rulesets, "comp_get_ruleset", hostname.Hostname())
	if err != nil {
		return nil, err
	}
	return rulesets, nil
}

func (t Ruleset) Render() string {
	return t.String()
}

func (t Ruleset) String() string {
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
	return t.String()
}

func (t Rulesets) String() string {
	buff := "rules:\n"
	for _, rset := range t {
		buff += fmt.Sprintf("%s", rset)
	}
	return buff
}
