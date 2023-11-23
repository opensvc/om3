package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFileincAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRule     string
		expectError  bool
		expectedRule CompFileinc
	}{
		"with a true rule (with check)": {
			jsonRule:    `{"path":"/tmp/foo","check":"regex","fmt":"lala","strict_fmt":false}`,
			expectError: false,
			expectedRule: CompFileinc{
				Path:      "/tmp/foo",
				Check:     "regex",
				Replace:   "",
				Fmt:       "lala",
				StrictFmt: false,
				Ref:       "",
			},
		},

		"with a true rule (with replace)": {
			jsonRule:    `{"path":"/tmp/foo","replace":"regex","fmt":"lala","strict_fmt":false}`,
			expectError: false,
			expectedRule: CompFileinc{
				Path:      "/tmp/foo",
				Check:     "",
				Replace:   "regex",
				Fmt:       "lala",
				StrictFmt: false,
				Ref:       "",
			},
		},

		"with a no check and no replace": {
			jsonRule:     `{"path":"/tmp/foo","fmt":"lala","strict_fmt":false,"ref":"thisisaref"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},

		"with no path": {
			jsonRule:     `{"check":"regex","fmt":"lala","strict_fmt":false,"ref":"thisisaref"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},

		"with check and replace": {
			jsonRule:     `{"path":"/tmp/foo","replace":"regex","fmt":"lala","strict_fmt":false,"ref":"thisisaref","check":"lala"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},

		"with fmt and ref": {
			jsonRule:     `{"path":"/tmp/foo","replace":"regex","fmt":"lala","strict_fmt":false,"ref":"thisisaref","fmt":"thisisfmt"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompFileincs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRule))
			} else {
				require.NoError(t, obj.Add(c.jsonRule))
				require.Equal(t, c.expectedRule, obj.rules[0].(CompFileinc))
			}
		})
	}
}
