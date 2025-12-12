package compliance

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/ybbus/jsonrpc"
)

type (
	Modulesets      map[string]Moduleset
	Moduleset       []ModulesetModule
	ModulesetModule struct {
		Name    string
		AutoFix bool
	}
	ModulesetRulesetRelations map[string][]string
	ModulesetRelations        map[string][]string
)

func (t ModulesetModule) String() string {
	return t.Name
}

func (t Moduleset) String() string {
	l := make([]string, len(t))
	for i, m := range t {
		l[i] = fmt.Sprint(m)
	}
	return strings.Join(l, ",")
}

// MarshalJSON marshals the data as a quoted json string
func (t ModulesetModule) MarshalJSON() ([]byte, error) {
	pivot := [2]interface{}{
		t.Name,
		t.AutoFix,
	}
	return json.Marshal(pivot)
}

// UnmarshalJSON unmashals a quoted json string to value
func (t *ModulesetModule) UnmarshalJSON(b []byte) error {
	pivot := [2]interface{}{}
	err := json.Unmarshal(b, &pivot)
	if err != nil {
		return err
	}
	if s, ok := pivot[0].(string); ok {
		t.Name = s
	} else {
		return fmt.Errorf("invalid moduleset name type: %+v", pivot[0])
	}
	if s, ok := pivot[1].(bool); ok {
		t.AutoFix = s
	} else {
		return fmt.Errorf("invalid moduleset autofix type: %+v", pivot[1])
	}
	return nil
}

func (t Modulesets) ModulesOf(modset string) []ModulesetModule {
	data, ok := t[modset]
	if !ok {
		return []ModulesetModule{}
	}
	return data
}

func (t Moduleset) ModuleNames() []string {
	l := make([]string, len(t))
	for i, m := range t {
		l[i] = m.Name
	}
	return l
}

func (t ModulesetModule) Render() string {
	buff := t.Name
	if t.AutoFix {
		buff += " (autofix)"
	}
	return buff
}

func (t Modulesets) Render() string {
	buff := "modulesets:\n"
	for modsetName, modset := range t {
		buff += fmt.Sprintf(" %s\n", modsetName)
		for _, mod := range modset {
			buff += fmt.Sprintf("  %s\n", mod.Render())
		}
	}
	return buff
}

func (t ModulesetRelations) Render() string {
	buff := "moduleset relations:\n"
	for name, l := range t {
		buff += fmt.Sprintf(" %s\n", name)
		for _, s := range l {
			buff += fmt.Sprintf("  %s\n", s)
		}
	}
	return buff
}

func (t ModulesetRulesetRelations) Render() string {
	buff := "moduleset-ruleset relations:\n"
	for name, l := range t {
		buff += fmt.Sprintf(" %s\n", name)
		for _, s := range l {
			buff += fmt.Sprintf("  %s\n", s)
		}
	}
	return buff
}

func (t T) ListModulesets(filter string) ([]string, error) {
	var err error
	data := make([]string, 0)
	if filter == "" {
		filter = "%"
	}
	err = t.collectorClient.CallFor(&data, "comp_list_modulesets", filter)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (t T) AttachModulesets(l []string) error {
	for _, s := range l {
		if err := t.AttachModuleset(s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

func (t T) AttachModuleset(s string) error {
	var (
		response *jsonrpc.RPCResponse
		err      error
	)
	if t.objectPath.IsZero() {
		response, err = t.collectorClient.Call("comp_attach_moduleset", hostname.Hostname(), s)
	} else {
		response, err = t.collectorClient.Call("comp_attach_svc_moduleset", t.objectPath.String(), s)
	}
	if err != nil {
		return err
	}
	collector.LogSimpleResponse(response, t.log)
	return nil
}

func (t T) DetachModuleset(s string) error {
	var (
		response *jsonrpc.RPCResponse
		err      error
	)
	if t.objectPath.IsZero() {
		response, err = t.collectorClient.Call("comp_detach_moduleset", hostname.Hostname(), s)
	} else {
		response, err = t.collectorClient.Call("comp_detach_svc_moduleset", t.objectPath.String(), s)
	}
	if err != nil {
		return err
	}
	collector.LogSimpleResponse(response, t.log)
	return nil
}
