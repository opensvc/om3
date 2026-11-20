package xconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/key"
)

// measureReferrer resolves "disk#2.free" as a resource measuring itself
// would, and counts the measures: a deferred reference is one not measured.
type measureReferrer struct {
	config   *T
	measures int
	inherit  keywords.Inherit
}

func (t *measureReferrer) KeywordLookup(k key.T, _ string) *keywords.Keyword {
	switch {
	case k.Option == "size":
		return &keywords.Keyword{Option: "size", Arithmetic: true, Inherit: t.inherit}
	case k.Section == "env":
		return &keywords.Keyword{Option: k.Option, Section: "env", Inherit: keywords.InheritLeaf}
	}
	return nil
}

func (t *measureReferrer) IsVolatile() bool            { return true }
func (t *measureReferrer) Config() *T                  { return t.config }
func (t *measureReferrer) ConfigData() any             { return nil }
func (t *measureReferrer) Nodes() ([]string, error)    { return []string{"n1"}, nil }
func (t *measureReferrer) DRPNodes() ([]string, error) { return nil, nil }
func (t *measureReferrer) Dereference(s string) (string, error) {
	if s == "disk#2.free" {
		t.measures++
		return "1000", nil
	}
	return s, ErrUnknownReference
}

func isFree(ref string) bool { return strings.HasSuffix(ref, ".free") }

func newMeasureConfig(t *testing.T, conf string, inherit keywords.Inherit) (*T, *measureReferrer) {
	t.Helper()
	cfg, err := NewObject("", []byte(conf))
	require.NoError(t, err)
	r := &measureReferrer{config: cfg, inherit: inherit}
	cfg.Referrer = r
	return cfg, r
}

func evalDeferring(cfg *T, k key.T) (any, error) {
	kw := cfg.Referrer.KeywordLookup(k, "")
	return cfg.EvalKeywordDeferringAs(k, kw, "", isFree)
}

// A value naming a deferred reference is not evaluated, and the reference
// is not resolved: what it measures is not measured.
func TestEvalKeywordDeferringAsDefersBeforeResolving(t *testing.T) {
	cfg, r := newMeasureConfig(t, "[disk#3]\nsize = $(50% * {disk#2.free})\n", keywords.InheritLeaf)
	k := key.New("disk#3", "size")

	_, err := evalDeferring(cfg, k)
	require.ErrorIs(t, err, ErrRefDeferred)
	assert.ErrorIs(t, err, ErrRefNotAvailable)
	assert.Equal(t, 0, r.measures, "the deferred reference is not measured")

	v, err := cfg.EvalKeywordAs(k, cfg.Referrer.KeywordLookup(k, ""), "")
	require.NoError(t, err)
	assert.Equal(t, "500", v, "evaluated now, the value is measured")
	assert.Equal(t, 1, r.measures)
}

// A keyword referencing one that names a deferred reference is deferred
// too, rather than left holding the reference.
func TestEvalKeywordDeferringAsDefersThroughAReferencedKeyword(t *testing.T) {
	cfg, r := newMeasureConfig(t, "[env]\nsz = $(50% * {disk#2.free})\n\n[disk#3]\nsize = {env.sz}\n", keywords.InheritLeaf)

	_, err := evalDeferring(cfg, key.New("disk#3", "size"))
	require.ErrorIs(t, err, ErrRefDeferred)
	assert.Equal(t, 0, r.measures)
}

// A value naming no deferred reference evaluates as EvalKeywordAs has it.
func TestEvalKeywordDeferringAsOfAValueDeferringNothing(t *testing.T) {
	cfg, _ := newMeasureConfig(t, "[env]\nsz = 10m\n\n[disk#3]\nsize = $(2 * {env.sz})\n\n[disk#4]\nsize = 100%FREE\n", keywords.InheritLeaf)
	for _, rid := range []string{"disk#3", "disk#4"} {
		k := key.New(rid, "size")
		kw := cfg.Referrer.KeywordLookup(k, "")
		want, err := cfg.EvalKeywordAs(k, kw, "")
		require.NoError(t, err)
		got, err := evalDeferring(cfg, k)
		require.NoError(t, err)
		assert.Equal(t, want, got, rid)
	}
}

// A keyword inheriting from DEFAULT whose own value is deferred does not
// fall back to the value it inherits: the value it names is the one to
// evaluate, later.
func TestEvalKeywordDeferringAsDoesNotFallBackToTheInheritedValue(t *testing.T) {
	cfg, _ := newMeasureConfig(t, "[DEFAULT]\nsize = 1g\n\n[disk#3]\nsize = {disk#2.free}\n", keywords.InheritLeaf2Head)

	_, err := evalDeferring(cfg, key.New("disk#3", "size"))
	require.ErrorIs(t, err, ErrRefDeferred)
}
