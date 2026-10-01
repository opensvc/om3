package xconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resourceReferrer is a referrer whose sections are resources, as an object
// with resources is.
type resourceReferrer struct {
	shareReferrer
}

func (t *resourceReferrer) HasResourceSections() bool { return true }

// A section shaped as a resource id and naming no driver group is a resource
// the instance status warns it ignores, so the validation reports it, once,
// in the same words.
func TestValidateUnsupportedSection(t *testing.T) {
	cfg, err := NewObject("", []byte("[foo#1]\nbaz = 1\nbar = 2\n"))
	require.NoError(t, err)
	cfg.Referrer = &resourceReferrer{shareReferrer{config: cfg}}
	alerts, err := cfg.Validate()
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	assert.Equal(t, alertKindUnknownDriver, alerts[0].Kind)
	assert.Equal(t, alertLevelWarn, alerts[0].Level)
	assert.Equal(t, "the foo driver is not supported by this agent: the resource is ignored", alerts[0].Comment)
}

// A section that is neither a resource nor a section an object reads, as the
// empty node section a past bug left, is shown by no resource in the status:
// the validation reports it, once, with or without keys.
func TestValidateUnknownSection(t *testing.T) {
	for _, conf := range []string{"[node]\n", "[node]\nfoo = bar\nbar = foo\n", "[subset]\n"} {
		cfg, err := NewObject("", []byte(conf))
		require.NoError(t, err)
		cfg.Referrer = &resourceReferrer{shareReferrer{config: cfg}}
		alerts, err := cfg.Validate()
		require.NoError(t, err)
		require.Len(t, alerts, 1, conf)
		assert.Equal(t, alertKindUnknownSection, alerts[0].Kind, conf)
		assert.Equal(t, alertLevelWarn, alerts[0].Level, conf)
	}
}

// The sections that are no resource are not reported, and neither is a
// section of a configuration whose sections are not resources, as the node's.
func TestValidateSectionsThatAreNoResource(t *testing.T) {
	for _, conf := range []string{"[DEFAULT]\n", "[env]\n", "[labels]\n", "[data]\n", "[subset#a]\n"} {
		cfg, err := NewObject("", []byte(conf))
		require.NoError(t, err)
		cfg.Referrer = &resourceReferrer{shareReferrer{config: cfg}}
		alerts, err := cfg.Validate()
		require.NoError(t, err)
		assert.Empty(t, alerts, conf)
	}
	cfg, err := NewObject("", []byte("[node]\n"))
	require.NoError(t, err)
	cfg.Referrer = &shareReferrer{config: cfg}
	alerts, err := cfg.Validate()
	require.NoError(t, err)
	assert.Empty(t, alerts)
}
