package oxcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/tmplrender"
)

type (
	// CmdTemplate renders Go templates from the cluster data and the
	// datastore keys, to files, and runs a reload command when a file
	// changes. With Watch, it renders a template again each time something
	// it read changes.
	//
	// A template is named by In, Out, ReloadCommand, Mode, User and Group,
	// or several by the Config file.
	CmdTemplate struct {
		In            string
		Out           string
		Config        string
		Watch         bool
		ReloadCommand string
		Delay         time.Duration
		MaxDelay      time.Duration
		ReloadTimeout time.Duration
		Mode          string
		User          string
		Group         string
		Namespace     string
		Node          string
		Vars          []string

		// Debug logs the events subscribed, the events leading to a
		// render, and the api calls.
		Debug bool
	}

	// templateJob is a template, the file it renders to, and the command
	// reloading what reads the file.
	templateJob struct {
		in            string
		out           string
		r             *tmplrender.Renderer
		output        *templateOutput
		reloadCommand string
		reloadTimeout time.Duration

		// reads are the reads of the last render, in a watch.
		reads []tmplrender.Read
	}

	// templateConfig is the config file of --config.
	templateConfig struct {
		// Vars are the template data, as .name reads it, of all the
		// templates. A --var of the same name replaces one.
		Vars map[string]string `yaml:"vars"`

		Templates []templateConfigEntry `yaml:"templates"`
	}

	templateConfigEntry struct {
		In            string `yaml:"in"`
		Out           string `yaml:"out"`
		Mode          string `yaml:"mode"`
		User          string `yaml:"user"`
		Group         string `yaml:"group"`
		ReloadCommand string `yaml:"reload_command"`
		ReloadTimeout string `yaml:"reload_timeout"`
	}

	// templateReload is a reload command, run once for the files it
	// reloads that were written.
	templateReload struct {
		command string
		timeout time.Duration
		outs    []string
	}
)

// prepare reads and parses the templates, and checks the options.
func (t *CmdTemplate) prepare() ([]*templateJob, error) {
	if t.Config != "" {
		return t.prepareConfig()
	}
	if t.In == "" || t.Out == "" {
		return nil, fmt.Errorf("--in and --out, or --config, are required")
	}
	if t.Out == "-" && (t.Watch || t.ReloadCommand != "") {
		return nil, fmt.Errorf("--out - writes to the standard output, which can't be watched nor reloaded")
	}
	if t.In == "-" && t.Watch {
		return nil, fmt.Errorf("--in - reads the template from the standard input, which can't be read again on SIGHUP: name a file to watch")
	}
	data, err := t.data(nil)
	if err != nil {
		return nil, err
	}
	job, err := t.newJob(t.In, t.Out, t.Mode, t.User, t.Group, t.ReloadCommand, t.ReloadTimeout, data)
	if err != nil {
		return nil, err
	}
	return []*templateJob{job}, nil
}

// prepareConfig returns the templates of the config file.
func (t *CmdTemplate) prepareConfig() ([]*templateJob, error) {
	if t.In != "" || t.Out != "" || t.ReloadCommand != "" || t.Mode != "" || t.User != "" || t.Group != "" {
		return nil, fmt.Errorf("--config names the templates: --in, --out, --reload-command, --mode, --user and --group are set in it")
	}
	b, err := os.ReadFile(t.Config)
	if err != nil {
		return nil, err
	}
	var cfg templateConfig
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", t.Config, err)
	}
	if len(cfg.Templates) == 0 {
		return nil, fmt.Errorf("%s: no templates", t.Config)
	}
	data, err := t.data(cfg.Vars)
	if err != nil {
		return nil, err
	}
	outs := make(map[string]int)
	jobs := make([]*templateJob, 0, len(cfg.Templates))
	for i, e := range cfg.Templates {
		where := fmt.Sprintf("%s: templates[%d]", t.Config, i)
		if e.In == "" || e.Out == "" {
			return nil, fmt.Errorf("%s: in and out are required", where)
		}
		if e.In == "-" || e.Out == "-" {
			return nil, fmt.Errorf("%s: the standard input and output are for --in and --out", where)
		}
		out := filepath.Clean(e.Out)
		if j, ok := outs[out]; ok {
			return nil, fmt.Errorf("%s: out %s is the out of templates[%d] too", where, e.Out, j)
		}
		outs[out] = i
		timeout := t.ReloadTimeout
		if e.ReloadTimeout != "" {
			if timeout, err = time.ParseDuration(e.ReloadTimeout); err != nil {
				return nil, fmt.Errorf("%s: reload_timeout: %w", where, err)
			}
		}
		job, err := t.newJob(e.In, e.Out, e.Mode, e.User, e.Group, e.ReloadCommand, timeout, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// data returns the template data: vars, replaced by the --var of the same
// name.
func (t *CmdTemplate) data(vars map[string]string) (map[string]any, error) {
	data := make(map[string]any)
	for k, v := range vars {
		data[k] = v
	}
	for _, s := range t.Vars {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --var %s: key=value is expected", s)
		}
		data[k] = v
	}
	return data, nil
}

func (t *CmdTemplate) newJob(in, out, mode, usr, grp, reloadCommand string, reloadTimeout time.Duration, data map[string]any) (*templateJob, error) {
	if err := checkReloadCommand(reloadCommand); err != nil {
		return nil, err
	}
	job := &templateJob{
		in:            in,
		out:           out,
		reloadCommand: reloadCommand,
		reloadTimeout: reloadTimeout,
		r: &tmplrender.Renderer{
			Name:      in,
			Namespace: t.Namespace,
			Data:      data,
		},
	}
	if err := job.readTemplate(); err != nil {
		return nil, err
	}
	if out != "-" {
		var err error
		if job.output, err = newTemplateOutput(out, mode, usr, grp); err != nil {
			return nil, err
		}
		if _, err := job.output.check(); err != nil {
			return nil, err
		}
	}
	return job, nil
}

// readTemplate reads the template of the job, and checks it parses: the
// former template is kept when it does not.
func (job *templateJob) readTemplate() error {
	var (
		b   []byte
		err error
	)
	if job.in == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(job.in)
	}
	if err != nil {
		return err
	}
	text := job.r.Text
	job.r.Text = string(b)
	if err := job.r.Parse(); err != nil {
		job.r.Text = text
		return err
	}
	return nil
}

// write writes the render of the job, and returns true when the file
// changed.
func (t *CmdTemplate) write(job *templateJob, res tmplrender.Result) (bool, error) {
	if job.output == nil {
		_, err := os.Stdout.Write(res.Out)
		return false, err
	}
	t.warnMode(job, res)
	changed, err := job.output.write(res.Out)
	if err != nil {
		return false, fmt.Errorf("write %s: %w", job.out, err)
	}
	if changed {
		t.logf("%s written", job.out)
	}
	return changed, nil
}

// reloadsOf returns the reload commands of the jobs written, each once, in
// the order of the jobs, with the longest timeout of its jobs.
func reloadsOf(written []*templateJob) []*templateReload {
	var l []*templateReload
	m := make(map[string]*templateReload)
	for _, job := range written {
		if job.reloadCommand == "" {
			continue
		}
		rl, ok := m[job.reloadCommand]
		if !ok {
			rl = &templateReload{command: job.reloadCommand}
			m[job.reloadCommand] = rl
			l = append(l, rl)
		}
		rl.timeout = max(rl.timeout, job.reloadTimeout)
		rl.outs = append(rl.outs, job.out)
	}
	return l
}

// reload runs the reload commands of the jobs written, each once after all
// the files are written, and returns their errors.
func (t *CmdTemplate) reload(written []*templateJob) error {
	var errs []error
	for _, rl := range reloadsOf(written) {
		// Not cancelled by a stop signal: a reload started finishes,
		// bounded by its timeout.
		if err := runReload(context.Background(), rl.command, strings.Join(rl.outs, " "), rl.timeout); err != nil {
			errs = append(errs, err)
			continue
		}
		t.logf("reloaded by %s", rl.command)
	}
	return errors.Join(errs...)
}

func (t *CmdTemplate) Run() error {
	jobs, err := t.prepare()
	if err != nil {
		return err
	}
	if t.Watch {
		return t.watch(jobs)
	}
	ctx := context.Background()
	c, err := client.New()
	if err != nil {
		return err
	}
	data, err := getClusterData(c, t.debugFunc())
	if err != nil {
		return err
	}
	st := tmplrender.NewState(data)

	// All the templates are rendered before a file is written: a file
	// of a set is not written with another failing to render.
	results := make([]tmplrender.Result, len(jobs))
	var errs []error
	for i, job := range jobs {
		job.r.Fetcher = &templateFetcher{c: c, debugf: t.debugFunc()}
		if results[i], err = job.r.Render(ctx, st); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	var written []*templateJob
	for i, job := range jobs {
		changed, err := t.write(job, results[i])
		if err != nil {
			errs = append(errs, err)
		} else if changed {
			written = append(written, job)
		}
	}
	if err := t.reload(written); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// getClusterData returns the cluster data, as the node serving the api sees
// it.
func getClusterData(c *client.T, debugf func(string, ...any)) (*clusterdump.Data, error) {
	begin := time.Now()
	b, err := c.NewGetClusterStatus().Get()
	if debugf != nil {
		debugf("api: GET /api/cluster/status: %d bytes in %s, error: %v", len(b), time.Since(begin).Round(time.Millisecond), err)
	}
	if err != nil {
		return nil, err
	}
	data := clusterdump.NewData("")
	if err := json.Unmarshal(b, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (t *CmdTemplate) warnMode(job *templateJob, res tmplrender.Result) {
	if job.output != nil && res.ReadsSecret && job.output.isGroupOrWorldReadable() {
		t.logf("warning: %s holds sec keys, and its mode %04o lets others than its owner read it", job.out, *job.output.mode)
	}
}

// debugf logs a message on the standard error with --debug.
func (t *CmdTemplate) debugf(format string, args ...any) {
	if t.Debug {
		t.logf("debug: "+format, args...)
	}
}

// debugFunc returns debugf with --debug, nil without.
func (t *CmdTemplate) debugFunc() func(string, ...any) {
	if !t.Debug {
		return nil
	}
	return t.debugf
}

// logf logs a message on the standard error. A message never holds a value
// a template read.
func (t *CmdTemplate) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, time.Now().Format(time.RFC3339)+" template: "+format+"\n", args...)
}
