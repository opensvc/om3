package compliance

import (
	"fmt"

	"github.com/ybbus/jsonrpc"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/util/hostname"
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
	rulesets := make(Rulesets)
	err := t.collectorClient.CallFor(&rulesets, "comp_get_ruleset", hostname.Hostname())
	if err != nil {
		return nil, err
	}
	return rulesets, nil
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
	var err error
	data := make([]string, 0)
	if filter == "" {
		filter = "%"
	}
	err = t.collectorClient.CallFor(&data, "comp_list_rulesets", filter, hostname.Hostname())
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (t T) AttachRuleset(s string) error {
	var (
		response *jsonrpc.RPCResponse
		err      error
	)
	if t.objectPath.IsZero() {
		response, err = t.collectorClient.Call("comp_attach_ruleset", hostname.Hostname(), s)
	} else {
		response, err = t.collectorClient.Call("comp_attach_svc_ruleset", t.objectPath.String(), s)
	}
	if err != nil {
		return err
	}
	collector.LogSimpleResponse(response, t.log)
	return nil
}

func (t T) DetachRuleset(s string) error {
	var (
		response *jsonrpc.RPCResponse
		err      error
	)
	if t.objectPath.IsZero() {
		response, err = t.collectorClient.Call("comp_detach_ruleset", hostname.Hostname(), s)
	} else {
		response, err = t.collectorClient.Call("comp_detach_svc_ruleset", t.objectPath.String(), s)
	}
	if err != nil {
		return err
	}
	collector.LogSimpleResponse(response, t.log)
	return nil
}
