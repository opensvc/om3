package compliance

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	Run struct {
		Modsets []string
		Mods    []string
		Attach  bool
		Force   bool

		InitAt        time.Time
		BeginAt       time.Time
		EndAt         time.Time
		ModuleActions ModuleActions

		main    *T
		data    Data
		modules []*Module
	}
	ModuleActions []ModuleAction
	ModuleAction  struct {
		Action   Action
		Module   string
		BeginAt  time.Time
		EndAt    time.Time
		ExitCode int
		Log      LogEntries
	}
	Action   string
	ExitCode int
	RunStat  struct {
		Ok    int
		Nok   int
		NA    int
		Total int
	}
)

const (
	ActionCheck   Action = "check"
	ActionFix     Action = "fix"
	ActionFixable Action = "fixable"

	ExitCodeOk  int = 0
	ExitCodeNok int = 1
	ExitCodeNA  int = 2
)

func (t *T) NewRun() *Run {
	run := Run{
		Modsets:       []string{},
		Mods:          []string{},
		InitAt:        time.Now(),
		BeginAt:       time.Now(),
		ModuleActions: make(ModuleActions, 0),
		main:          t,
	}
	return &run
}

func (t *Run) Close() {
	t.EndAt = time.Now()
}

func (t *Run) SetModulesetsExpr(s string) {
	t.Modsets = xstrings.Split(s, ",")
}

func (t *Run) SetModulesets(l []string) {
	t.Modsets = l
}

func (t *Run) SetModulesExpr(s string) {
	t.Mods = xstrings.Split(s, ",")
}

func (t *Run) SetModules(l []string) {
	t.Mods = l
}

func (t *Run) SetAttach(v bool) {
	t.Attach = v
}

func (t *Run) SetForce(v bool) {
	t.Attach = v
}

func (t *Run) endInit() {
	t.BeginAt = time.Now()
}

func (t *Run) init() error {
	defer t.endInit()
	if len(t.Mods) > 0 && len(t.Modsets) > 0 {
		return errors.Errorf("modules and modulesets can't be selected both")
	}
	if t.Attach && len(t.Modsets) > 0 {
		if err := t.main.AttachModulesets(t.Modsets); err != nil {
			return err
		}
	}
	if data, err := t.main.GetData(t.Modsets); err != nil {
		return errors.Wrap(err, "init data")
	} else {
		t.data = data
	}
	t.modules = make(Modules, 0)
	for _, modName := range t.data.ExpandModules(t.Modsets, t.Mods) {
		if mod, err := t.main.NewModule(modName); err != nil {
			return errors.Wrap(err, "init module")
		} else {
			t.modules = append(t.modules, mod)
		}
	}
	return nil
}

func (t *Run) Check() error {
	return t.do(ActionCheck)
}

func (t *Run) do(action Action) error {
	defer t.Close()
	if err := t.init(); err != nil {
		return err
	}
	for _, mod := range t.modules {
		if err := t.moduleAction(mod, action); err != nil {
			return err
		}
	}
	return nil
}

func (t *Run) moduleExec(mod *Module, action Action) (ModuleAction, error) {
	ma := ModuleAction{
		Action:  action,
		Module:  mod.Name(),
		BeginAt: time.Now(),
	}
	cmd := command.New(
		command.WithName(mod.Path()),
		command.WithVarArgs(string(action)),
		command.WithIgnoredExitCodes(),
		// TODO: command.WithEnv(),
		command.WithOnStdoutLine(func(s string) {
			ma.Log.Out(s)
		}),
		command.WithOnStderrLine(func(s string) {
			ma.Log.Err(s)
		}),
	)
	err := cmd.Run()
	if err != nil {
		ma.Log.Err(fmt.Sprint(err))
	}
	ma.ExitCode = cmd.ExitCode()
	ma.EndAt = time.Now()
	t.ModuleActions = append(t.ModuleActions, ma)
	return ma, err
}

func (t *Run) moduleAction(mod *Module, action Action) error {
	var (
		ma  ModuleAction
		err error
	)
	if t.Force {
		_, err = t.moduleExec(mod, action)
		return err
	}
	switch action {
	case ActionCheck:
		if ma, err = t.moduleExec(mod, ActionCheck); err != nil {
			return err
		}
		if ma.ExitCode != ExitCodeNok {
			return nil
		}
		if _, err = t.moduleExec(mod, ActionFixable); err != nil {
			return err
		}
	case ActionFix:
		if ma, err = t.moduleExec(mod, ActionCheck); err != nil {
			return err
		}
		if ma.ExitCode == ExitCodeOk {
			return nil
		}
		if _, err = t.moduleExec(mod, ActionFixable); err != nil {
			return err
		}
		if ma.ExitCode == ExitCodeNok {
			return nil
		}
		if ma, err = t.moduleExec(mod, ActionFix); err != nil {
			return err
		}
		if ma, err = t.moduleExec(mod, ActionCheck); err != nil {
			return err
		}
	case ActionFixable:
		if ma, err = t.moduleExec(mod, ActionFixable); err != nil {
			return err
		}
	default:
		return errors.Errorf("%s: invalid action", action)
	}
	return nil
}

func (t Run) runDuration() time.Duration {
	return t.EndAt.Sub(t.BeginAt)
}

func (t Run) initDuration() time.Duration {
	return t.BeginAt.Sub(t.InitAt)
}

func (t Run) Stat() RunStat {
	stat := RunStat{}
	m := make(map[string]int)
	for _, ma := range t.ModuleActions {
		m[ma.Module] = ma.ExitCode
	}
	for _, x := range m {
		switch x {
		case 0:
			stat.Ok += 1
		case 2:
			stat.NA += 1
		default:
			stat.Nok += 1
		}
	}
	stat.Total = len(m)
	return stat
}

func (t Run) Render() string {
	buff := t.ModuleActions.Render()
	buff += "\n"
	stat := t.Stat()
	buff += "Run:\n"
	buff += fmt.Sprintf(" Data Fetch:        %s\n", t.initDuration())
	buff += fmt.Sprintf(" Modules Execution: %s\n", t.runDuration())
	buff += fmt.Sprintf(" Modules Count:     %d\n", stat.Total)
	buff += fmt.Sprintf(" Modules States:    %d ok, %d nok, %d n/a\n", stat.Ok, stat.Nok, stat.NA)
	return buff
}

func (t ModuleAction) Status() string {
	switch t.ExitCode {
	case 0:
		return rawconfig.Node.Colorize.Optimal("ok")
	case 1:
		return rawconfig.Node.Colorize.Error("nok")
	case 2:
		return rawconfig.Node.Colorize.Secondary("n/a")
	default:
		return fmt.Sprintf("%s (%d)", rawconfig.Node.Colorize.Error("nok"), t.ExitCode)
	}
}

func (t ModuleAction) Duration() time.Duration {
	return t.EndAt.Sub(t.BeginAt)
}

func (t ModuleActions) Render() string {
	buff := ""
	last := ""
	for _, ma := range t {
		if ma.Module != last {
			buff += fmt.Sprintf("- Module: %s\n", rawconfig.Node.Colorize.Bold(ma.Module))
			last = ma.Module
		}
		buff += ma.Render()
	}
	return buff
}

func (t ModuleAction) Render() string {
	buff := fmt.Sprintf("  - Action:   %s\n", rawconfig.Node.Colorize.Bold(t.Action))
	buff += fmt.Sprintf("    Status:   %s\n", t.Status())
	buff += fmt.Sprintf("    Duration: %s\n", t.Duration())
	buff += fmt.Sprintf("    Log:\n")
	for _, e := range t.Log.Entries() {
		switch e.Level {
		case LogLevelOut:
			buff += fmt.Sprintf("      %s\n", e.Msg)
		case LogLevelErr:
			buff += fmt.Sprintf("      %s\n", rawconfig.Node.Colorize.Error("Err: ")+e.Msg)
		}
	}
	return buff

}
