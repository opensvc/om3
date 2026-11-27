package tui

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
)

// termHint is the advice appended to the errors about the terminal type.
const termHint = "set the TERM environment variable to a supported terminal type, like xterm-256color"

// lookupTerminfo is tcell.LookupTerminfo, replaced by the tests.
var lookupTerminfo = func(name string) error {
	_, err := tcell.LookupTerminfo(name)
	return err
}

// explainTermError returns err explained by the TERM environment variable
// when the terminal type it names is unknown.
//
// tcell falls back to running infocmp for a terminal type missing from
// its builtin database, and returns the infocmp error as is, for example
// "exit status 1", which does not tell the user the TERM value is the
// culprit.
func explainTermError(err error) error {
	if err == nil {
		return nil
	}
	term := os.Getenv("TERM")
	if lookupTerminfo(term) == nil {
		// The terminal type is known: the error has another cause.
		return err
	}
	if term == "" {
		return fmt.Errorf("the TERM environment variable is not set (%w): %s", err, termHint)
	}
	return fmt.Errorf("terminal type %q is not supported (%w): %s", term, err, termHint)
}
