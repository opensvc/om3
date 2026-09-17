package xconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/key"
)

// sizeReferrer declares one keyword, converted to a size, in a section om
// knows nothing else about.
type sizeReferrer struct {
	config *T
}

func (t *sizeReferrer) KeywordLookup(k key.T, _ string) *keywords.Keyword {
	if k.Section == "quota" && k.Option == "usage" {
		return &keywords.Keyword{
			Option:    "usage",
			Section:   "quota",
			Converter: converters.Size,
		}
	}
	return nil
}

func (t *sizeReferrer) IsVolatile() bool                     { return true }
func (t *sizeReferrer) Config() *T                           { return t.config }
func (t *sizeReferrer) ConfigData() any                      { return nil }
func (t *sizeReferrer) Dereference(s string) (string, error) { return s, nil }
func (t *sizeReferrer) Nodes() ([]string, error)             { return []string{"n1"}, nil }
func (t *sizeReferrer) DRPNodes() ([]string, error)          { return nil, nil }

func newSizeConfig(t *testing.T) *T {
	t.Helper()
	cfg, err := NewObject("", []byte("[quota]\nusage = 200m\n"))
	require.NoError(t, err)
	cfg.Referrer = &sizeReferrer{config: cfg}
	return cfg
}

// A keyword declaring a converter evaluates to what it converts to, and
// asking for it as a string must answer rather than take the daemon down.
func TestGetStringOfAConvertedKeyword(t *testing.T) {
	cfg := newSizeConfig(t)
	k := key.New("quota", "usage")

	assert.NotPanics(t, func() { _ = cfg.GetString(k) })

	s, err := cfg.GetStringStrict(k)
	assert.NoError(t, err)
	assert.Equal(t, "209715200", s)
}

// A converter answers with what it makes of the value, and every shape it
// makes has to render as the configuration spells it.
func TestEvaluatedString(t *testing.T) {
	i := int64(209715200)
	b := true
	d := 90 * time.Second
	var nilPtr *int64

	for _, tc := range []struct {
		name     string
		value    any
		expected string
	}{
		{"nil", nil, ""},
		{"string", "1g", "1g"},
		{"size behind a pointer", &i, "209715200"},
		{"bool behind a pointer", &b, "true"},
		{"duration behind a pointer", &d, "1m30s"},
		{"a nil pointer is no value", nilPtr, ""},
		{"a list joins on spaces, not brackets", []string{"_/etc:/etc:ro", "b"}, "_/etc:/etc:ro b"},
		{"an empty list is no value", []string{}, ""},
		{"an int is itself", 42, "42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, EvaluatedString(tc.value))
		})
	}
}
