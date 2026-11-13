package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunExitCode(t *testing.T) {
	ma := func(module string, action Action, code int) *ModuleAction {
		return &ModuleAction{Module: module, Action: action, ExitCode: code}
	}
	for _, tc := range []struct {
		name string
		mas  ModuleActions
		want int
	}{
		{"no module", ModuleActions{}, ExitCodeOk},
		{"compliant", ModuleActions{ma("m1", ActionCheck, 0), ma("m2", ActionCheck, 2)}, ExitCodeOk},
		{"not compliant, fixable", ModuleActions{ma("m1", ActionCheck, 1), ma("m1", ActionFixable, 0)}, ExitCodeNok},
		{"fixed", ModuleActions{
			ma("m1", ActionCheck, 1),
			ma("m1", ActionFixable, 0),
			ma("m1", ActionFix, 0),
			ma("m1", ActionCheck, 0),
		}, ExitCodeOk},
		{"fix failed", ModuleActions{
			ma("m1", ActionCheck, 1),
			ma("m1", ActionFixable, 0),
			ma("m1", ActionFix, 1),
			ma("m1", ActionCheck, 1),
		}, ExitCodeNok},
		{"all not applicable", ModuleActions{ma("m1", ActionCheck, 2), ma("m2", ActionCheck, 2)}, ExitCodeNA},
		{"module error", ModuleActions{ma("m1", ActionCheck, 127)}, ExitCodeNok},
		{"fixable run", ModuleActions{ma("m1", ActionFixable, 1)}, ExitCodeNok},
		{"forced fix", ModuleActions{ma("m1", ActionFix, 0)}, ExitCodeOk},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Run{ModuleActions: tc.mas}.ExitCode())
		})
	}
}
