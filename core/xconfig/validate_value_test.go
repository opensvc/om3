package xconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/key"
)

// quotaReferrer declares one keyword whose values a Validate function tells
// apart, the way the pg_* keywords are.
type quotaReferrer struct {
	shareReferrer
}

func (t *quotaReferrer) KeywordLookup(k key.T, _ string) *keywords.Keyword {
	if k.Section == "disk#1" && k.BaseOption() == "quota" {
		return &keywords.Keyword{
			Option:   "quota",
			Section:  "disk#1",
			Scopable: true,
			Validate: func(s string) error {
				if s != "50%" && s != "default" {
					return fmt.Errorf("%q is not a quota", s)
				}
				return nil
			},
		}
	}
	return nil
}

func validateQuota(t *testing.T, lines string) Alerts {
	t.Helper()
	cfg, err := NewObject("", []byte("[disk#1]\n"+lines+"\n"))
	require.NoError(t, err)
	cfg.Referrer = &quotaReferrer{shareReferrer{config: cfg}}
	alerts, err := cfg.Validate()
	require.NoError(t, err)
	return alerts
}

// A value the keyword does not take is refused when written, rather than
// failing where it is used, which is later and on every node.
func TestValidateValue(t *testing.T) {
	for _, lines := range []string{"quota = 50%", "quota = default", "quota =", "quota@n1 = 50%"} {
		t.Run("ok "+lines, func(t *testing.T) {
			assert.Empty(t, validateQuota(t, lines))
		})
	}
	for _, lines := range []string{"quota = 50%@x", "quota@n1 = x"} {
		t.Run(lines, func(t *testing.T) {
			alerts := validateQuota(t, lines)
			require.Len(t, alerts, 1)
			assert.Equal(t, alertKindCandidates, alerts[0].Kind)
			assert.Equal(t, alertLevelError, alerts[0].Level)
			assert.Contains(t, alerts[0].Comment, "is not a quota")
		})
	}
}
