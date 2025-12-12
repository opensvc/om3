package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/xsession"
	"github.com/opensvc/om3/v3/util/xstrings"
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
		modules Modules
	}
	ModuleActions []*ModuleAction
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
	ActionAuto    Action = "auto"

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
	t.Push()
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
	t.Force = v
}

func (t *Run) endInit() {
	t.BeginAt = time.Now()
}

func (t *Run) init() error {
	defer t.endInit()
	if len(t.Mods) > 0 && len(t.Modsets) > 0 {
		return fmt.Errorf("modules and modulesets can't be selected both")
	}
	if t.Attach && len(t.Modsets) > 0 {
		if err := t.main.AttachModulesets(t.Modsets); err != nil {
			return err
		}
	}
	if data, err := t.main.GetData(t.Modsets); err != nil {
		return fmt.Errorf("init data: %w", err)
	} else {
		t.data = data
	}
	t.modules = make(Modules, 0)
	for _, mod := range t.data.ExpandModules(t.Modsets, t.Mods) {
		err := t.main.Validate(mod)
		if err == nil {
			t.modules = append(t.modules, mod)
			continue
		}
		rset := t.data.Ruleset(mod.ModulesetName())
		if strings.Contains(rset.Filter, "via moduleset") {
			mod.order = parseModuleOrder(mod.Name())
			t.modules = append(t.modules, mod)
			continue
		}
		return fmt.Errorf("init module: %w", err)
	}
	sort.Sort(t.modules)
	return nil
}

func (t *Run) Auto() error {
	return t.do(ActionAuto)
}

func (t *Run) Check() error {
	return t.do(ActionCheck)
}

func (t *Run) Fix() error {
	return t.do(ActionFix)
}

func (t *Run) Fixable() error {
	return t.do(ActionFixable)
}

func (t *Run) Env() (Envs, error) {
	envs := make(Envs)
	if err := t.init(); err != nil {
		return envs, err
	}
	for _, mod := range t.modules {
		env, err := t.moduleEnv(mod)
		if err != nil {
			return envs, err
		}
		envs[mod.Name()] = env
	}
	return envs, nil
}

func (t Run) autoAction(action Action, mod *Module) Action {
	if action != ActionAuto {
		return action
	}
	if mod.Autofix() {
		return ActionFix
	}
	return ActionCheck
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

func (t *Run) moduleEnv(mod *Module) ([]string, error) {
	m := make(map[string]interface{})
	m["LANG"] = "C.UTF-8"
	m["LC_NUMERIC"] = "C"
	m["LC_TIME"] = "C"
	m["PYTHONIOENCODING"] = "utf-8"
	m["OSVC_PYTHON"] = rawconfig.Paths.Python
	m["OSVC_PATH_ETC"] = rawconfig.Paths.Etc
	m["OSVC_PATH_VAR"] = rawconfig.Paths.Var
	m["OSVC_PATH_COMP"] = t.main.varDir
	m["OSVC_PATH_TMP"] = rawconfig.Paths.Tmp
	m["OSVC_PATH_LOG"] = rawconfig.Paths.Log
	m["OSVC_NODEMGR"] = filepath.Join(rawconfig.Paths.Bin, "nodemgr")
	m["OSVC_SVCMGR"] = filepath.Join(rawconfig.Paths.Bin, "svcmgr")
	m["OSVC_SESSION_UUID"] = xsession.ID

	if runtime.GOOS != "windows" {
		m["PATH"] = rawconfig.Paths.Bin + ":" + os.Getenv("PATH")
	}

	/*
			# add services env section keys, with values eval'ed on this node
		        if self.context.svc:
		            os.environ[self.context.format_rule_var("SVC_NAME")] = self.context.format_rule_val(self.context.svc.name)
		            os.environ[self.context.format_rule_var("SVC_PATH")] = self.context.format_rule_val(self.context.svc.path)
		            if self.context.svc.namespace:
		                os.environ[self.context.format_rule_var("SVC_NAMESPACE")] = self.context.format_rule_val(self.context.svc.namespace)
		            for key, val in self.context.svc.env_section_keys_evaluated().items():
		                os.environ[self.context.format_rule_var("SVC_CONF_ENV_"+key.upper())] = self.context.format_rule_val(val)

		        for rset in self.ruleset.values():
		            if (rset["filter"] != "explicit attachment via moduleset" and "matching non-public contextual ruleset shown via moduleset" not in rset["filter"]) or ( \
		               self.moduleset in self.context.data["modset_rset_relations"]  and \
		               rset['name'] in self.context.data["modset_rset_relations"][self.moduleset]
		               ):
		                for rule in rset['vars']:
		                    var, val, var_class = self.context.parse_rule(rule)
		                    os.environ[self.context.format_rule_var(var)] = self.context.format_rule_val(val)
	*/

	for _, rset := range t.data.Rsets {
		if t.rsetIsNotPrivateContextualAndNotExplicitViaModuleset(rset) || t.rsetIsRelatedToModset(rset, mod.modset) {
			for k, v := range rset.Vars.EnvMap() {
				m[k] = v
			}
		}
	}

	env := make([]string, 0)
	for k, v := range m {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env, nil
}

func (t Run) rsetIsRelatedToModset(rset Ruleset, modsetName string) bool {
	relations, ok := t.data.ModsetRsetRelations[modsetName]
	if !ok {
		return false
	}
	for _, s := range relations {
		if s == rset.Name {
			return true
		}
	}
	return false
}

func (t Run) rsetIsNotPrivateContextualAndNotExplicitViaModuleset(rset Ruleset) bool {
	return !t.rsetIsPrivateContextual(rset) && !t.rsetIsExplicitViaModuleset(rset)
}

func (t Run) rsetIsPrivateContextual(rset Ruleset) bool {
	return strings.Contains(rset.Filter, "matching non-public contextual ruleset shown via moduleset")
}

func (t Run) rsetIsExplicitViaModuleset(rset Ruleset) bool {
	return rset.Filter == "explicit attachment via moduleset"
}

func (t *Run) autoModuleExec(mod *Module, action Action) (*ModuleAction, error) {
	ma := &ModuleAction{
		Action:  action,
		Module:  mod.Name(),
		BeginAt: time.Now(),
	}
	env, err := t.moduleEnv(mod)
	if err != nil {
		ma.ExitCode = -1
		ma.EndAt = time.Now()
		return ma, err
	}
	rset := t.data.Ruleset(mod.ModulesetName())
	vars := rset.Vars
	sort.Sort(vars)
	for i, v := range vars {
		ret := t.autoModuleVarExec(mod, action, v, env, ma)
		switch ret {
		case ExitCodeOk:
		case ExitCodeNA:
			if i == 0 {
				ma.ExitCode = 2
			}
		default:
			ma.ExitCode = ret
			if action == ActionFix {
				break // at first error in a fix run
			}
		}
	}
	ma.EndAt = time.Now()
	t.ModuleActions = append(t.ModuleActions, ma)
	return ma, err
}

func (t *Run) autoModuleVarExec(mod *Module, action Action, v Var, env []string, ma *ModuleAction) int {
	if v.Class == "raw" {
		return 0
	}
	fn := t.getObjectFunc(v.Class)
	if fn == nil {
		ma.Log.Err(fmt.Sprint("invalid class", v.Class, "for rule", v.Name))
		return 1
	}
	ret := fn(action, v, env, ma)
	return ret
}

type objectExecFunc func(action Action, v Var, env []string, ma *ModuleAction) int

func (t *Run) getObjectFunc(class string) objectExecFunc {
	return t.objectExec
}

func (t *Run) objectExec(action Action, v Var, env []string, ma *ModuleAction) int {
	//path := filepath.Join(t.main.varDir, "com.opensvc", v.Class+".py")
	cmd := command.New(
		command.WithName(filepath.Join(rawconfig.Paths.Compliance, v.Class)),
		command.WithVarArgs(v.EnvName(), string(action)),
		command.WithIgnoredExitCodes(),
		command.WithEnv(env),
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
		return 1
	}
	return cmd.ExitCode()
}

func (t *Run) moduleExec(mod *Module, action Action) (*ModuleAction, error) {
	if mod.path == "" {
		return t.autoModuleExec(mod, action)
	}
	ma := &ModuleAction{
		Action:  action,
		Module:  mod.Name(),
		BeginAt: time.Now(),
	}
	env, err := t.moduleEnv(mod)
	if err != nil {
		ma.ExitCode = -1
		ma.EndAt = time.Now()
		return ma, err
	}
	cmd := command.New(
		command.WithName(mod.Path()),
		command.WithVarArgs(string(action)),
		command.WithIgnoredExitCodes(),
		command.WithEnv(env),
		command.WithOnStdoutLine(func(s string) {
			ma.Log.Out(s)
		}),
		command.WithOnStderrLine(func(s string) {
			ma.Log.Err(s)
		}),
	)
	err = cmd.Run()
	if err != nil {
		ma.Log.Err(fmt.Sprint(err))
	}
	ma.ExitCode = cmd.ExitCode()
	ma.EndAt = time.Now()
	t.ModuleActions = append(t.ModuleActions, ma)
	return ma, err
}

func (t Run) Push() error {
	if len(t.ModuleActions) == 0 {
		return nil
	}
	vars := []string{
		"run_nodename",
		"run_module",
		"run_status",
		"run_log",
		"run_action",
		"rset_md5",
		"run_svcname",
	}
	hn := hostname.Hostname()
	vals := make([][]interface{}, 0)
	md5sum := t.data.RulesetsMD5()
	for _, ma := range t.ModuleActions {
		v := []interface{}{
			hn,
			ma.Module,
			ma.ExitCode,
			ma.Log.RenderForCollector(),
			ma.Action,
			md5sum,
			t.main.objectPath,
		}
		vals = append(vals, v)
	}
	_, err := t.main.collectorClient.Call("comp_log_actions", vars, vals)
	if err != nil {
		return err
	}
	return nil
}

func (t *Run) moduleAction(mod *Module, action Action) error {
	var (
		ma  *ModuleAction
		err error
	)
	if t.Force {
		_, err = t.moduleExec(mod, action)
		return err
	}
	action = t.autoAction(action, mod)
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
		if ma, err = t.moduleExec(mod, ActionFixable); err != nil {
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
		return fmt.Errorf("%s: invalid action", action)
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
		if ma.Action == ActionCheck {
			m[ma.Module] = ma.ExitCode
		}
	}
	for _, x := range m {
		switch x {
		case 0:
			stat.Ok++
		case 2:
			stat.NA++
		default:
			stat.Nok++
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
	if stat.Total > 0 {
		buff += fmt.Sprintf(" Checks by State:   %d ok, %d nok, %d n/a\n", stat.Ok, stat.Nok, stat.NA)
	}
	return buff
}

func (t *ModuleAction) Status() string {
	switch t.ExitCode {
	case 0:
		return rawconfig.Colorize.Optimal("ok")
	case 2:
		return rawconfig.Colorize.Secondary("n/a")
	default:
		return rawconfig.Colorize.Error("nok")
	}
}

func (t *ModuleAction) StatusAndExitCode() string {
	switch t.ExitCode {
	case 0:
		return rawconfig.Colorize.Optimal("ok")
	case 1:
		return rawconfig.Colorize.Error("nok")
	case 2:
		return rawconfig.Colorize.Secondary("n/a")
	default:
		return fmt.Sprintf("%s (%d)", rawconfig.Colorize.Error("nok"), t.ExitCode)
	}
}

func (t *ModuleAction) Duration() time.Duration {
	return t.EndAt.Sub(t.BeginAt)
}

func (t *ModuleAction) Render() string {
	buff := fmt.Sprintf("  - Action:   %s\n", rawconfig.Colorize.Bold(t.Action))
	buff += fmt.Sprintf("    Status:   %s\n", t.StatusAndExitCode())
	buff += fmt.Sprintf("    Duration: %s\n", t.Duration())
	buff += fmt.Sprintf("    Log:\n")
	buff += t.Log.Render()
	return buff

}

func (t ModuleActions) Render() string {
	buff := ""
	last := ""
	for _, ma := range t {
		if ma.Module != last {
			buff += fmt.Sprintf("- Module: %s\n", rawconfig.Colorize.Bold(ma.Module))
			last = ma.Module
		}
		buff += ma.Render()
	}
	return buff
}
