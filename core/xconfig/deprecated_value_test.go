package xconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/key"
)

// shareReferrer declares one keyword that takes a size, and says which of the
// forms it takes is on its way out, the way the size of a logical volume does.
type shareReferrer struct {
	config *T
}

func (t *shareReferrer) KeywordLookup(k key.T, _ string) *keywords.Keyword {
	if k.Section == "disk#1" && k.Option == "size" {
		return &keywords.Keyword{
			Option:              "size",
			Section:             "disk#1",
			DeprecatedValue:     `(?i)[0-9]+%(free|pvs|vg)\b`,
			DeprecatedValueText: "write it as a size instead",
		}
	}
	return nil
}

func (t *shareReferrer) IsVolatile() bool                     { return true }
func (t *shareReferrer) Config() *T                           { return t.config }
func (t *shareReferrer) ConfigData() any                      { return nil }
func (t *shareReferrer) Dereference(s string) (string, error) { return s, nil }
func (t *shareReferrer) Nodes() ([]string, error)             { return []string{"n1"}, nil }
func (t *shareReferrer) DRPNodes() ([]string, error)          { return nil, nil }

func validateSize(t *testing.T, value string) Alerts {
	t.Helper()
	cfg, err := NewObject("", []byte("[disk#1]\nsize = "+value+"\n"))
	require.NoError(t, err)
	cfg.Referrer = &shareReferrer{config: cfg}
	alerts, err := cfg.Validate()
	require.NoError(t, err)
	return alerts
}

// A keyword is the one to use, and one of the forms it takes is not. That is
// said where the configuration is written, because writing it is the moment
// the form can still be changed for nothing.
func TestValidateDeprecatedValue(t *testing.T) {
	for _, value := range []string{"100%FREE", "50%VG", "20%pvs"} {
		t.Run(value, func(t *testing.T) {
			alerts := validateSize(t, value)
			require.Len(t, alerts, 1)
			assert.Equal(t, alertKindDeprecatedValue, alerts[0].Kind)
			assert.Equal(t, alertLevelWarn, alerts[0].Level, "the value still works")
			assert.Equal(t, "write it as a size instead", alerts[0].Comment)
		})
	}

	// A size, and the number an expression over the free space came out as,
	// are both sizes.
	for _, value := range []string{"10m", "1g", "268435456", "$(100% * 268435456)"} {
		t.Run("ok "+value, func(t *testing.T) {
			assert.Empty(t, validateSize(t, value))
		})
	}
}

// A commit refuses the errors and keeps the warnings to itself, so what is
// kept has to be readable on its own: the two are told apart the same way.
func TestAlertsWarns(t *testing.T) {
	alerts := Alerts{
		{Key: key.New("fs#1", "vg"), Kind: alertKindUnknown, Level: alertLevelWarn},
		{Key: key.New("fs#1", "type"), Kind: alertKindCandidates, Level: alertLevelError},
		{Key: key.New("disk#1", "size"), Kind: alertKindDeprecatedValue, Level: alertLevelWarn},
	}

	warns := alerts.Warns()
	require.Len(t, warns, 2)
	assert.Equal(t, key.New("fs#1", "vg"), warns[0].Key)
	assert.Equal(t, key.New("disk#1", "size"), warns[1].Key)

	assert.Len(t, alerts.Errors(), 1, "and the errors are the rest")
	assert.Equal(t, len(alerts), len(alerts.Warns())+len(alerts.Errors()),
		"every alert is one or the other")

	assert.Empty(t, Alerts{}.Warns())
}
