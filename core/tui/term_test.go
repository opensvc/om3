package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestExplainTermError(t *testing.T) {
	runErr := errors.New("exit status 1")
	unknown := func(string) error { return errors.New("infocmp failed") }
	known := func(string) error { return nil }

	cases := []struct {
		name     string
		term     string
		lookup   func(string) error
		err      error
		wantNil  bool
		wantSame bool
		contains []string
	}{
		{name: "no error", term: "bogus", lookup: unknown, err: nil, wantNil: true},
		{name: "known terminal", term: "xterm", lookup: known, err: runErr, wantSame: true},
		{
			name: "unknown terminal", term: "bogus", lookup: unknown, err: runErr,
			contains: []string{`terminal type "bogus" is not supported`, "exit status 1", termHint},
		},
		{
			name: "unset terminal", term: "", lookup: unknown, err: runErr,
			contains: []string{"the TERM environment variable is not set", "exit status 1", termHint},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TERM", tc.term)
			saved := lookupTerminfo
			lookupTerminfo = tc.lookup
			defer func() { lookupTerminfo = saved }()

			got := explainTermError(tc.err)
			switch {
			case tc.wantNil:
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
			case tc.wantSame:
				if got != tc.err {
					t.Fatalf("got %v, want the unchanged error", got)
				}
			default:
				if !errors.Is(got, tc.err) {
					t.Fatalf("got %v, which does not wrap %v", got, tc.err)
				}
				for _, s := range tc.contains {
					if !strings.Contains(got.Error(), s) {
						t.Errorf("got %q, missing %q", got, s)
					}
				}
			}
		})
	}
}
