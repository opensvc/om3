package keywords

import (
	"strings"

	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/key"
)

// Keyword represents a configuration option in an object or node configuration file
type (
	Keyword struct {
		Section string
		Option  string
		Attr    string

		// Generic means the keyword can be set in any section.
		Generic bool

		// Scopable means the keyword can have a different value on nodes, drpnodes, encapnodes or a specific node.
		Scopable bool

		// Required means the keyword mean be set, and thus disregards the default value.
		Required bool

		// Converter is the type caster to user.
		Converter converters.T

		// Text is a text explaining the role of the keyword.
		Text string

		// DefaultText is a text explaining the default value.
		DefaultText string

		// Example demonstrates the keyword usage.
		Example string

		// Default is the value returned when the non-required keyword is not set.
		Default string

		// Candidates is the list of accepted values. An empty list.
		Candidates []string

		// Depends is a list of key-value conditions to meet to accept this keyword in a config.
		//Depends []keyval.T

		// Kind limits the scope of this keyword to the object with kind matching this mask.
		Kind kind.Mask

		// Provisioning is set to true for keywords only used for resource provisioning
		Provisioning bool
	}

	Store []Keyword
)

func (t Store) Lookup(k key.T, kd kind.T) Keyword {
	driverGroup := strings.Split(k.Section, "#")[0]
	for _, kw := range t {
		if !kw.Kind.Has(kd) {
			continue
		}
		if k.Option != kw.Option {
			continue
		}
		if kw.Section == "" || k.Section == kw.Section || driverGroup == kw.Section {
			return kw
		}
	}
	return Keyword{}
}

func (t Keyword) IsZero() bool {
	return t.Option == ""
}
