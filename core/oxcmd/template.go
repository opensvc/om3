package oxcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/tmplrender"
)

type (
	// CmdTemplate renders a Go template from the cluster data and the
	// datastore keys, to a file, and runs a reload command when the file
	// changes. With Watch, it renders again each time something the
	// template read changes.
	CmdTemplate struct {
		In            string
		Out           string
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
	}
)

// prepare reads and parses the template, and checks the options.
func (t *CmdTemplate) prepare() (*tmplrender.Renderer, *templateOutput, error) {
	if t.In == "" || t.Out == "" {
		return nil, nil, fmt.Errorf("--in and --out are required")
	}
	if t.Out == "-" && (t.Watch || t.ReloadCommand != "") {
		return nil, nil, fmt.Errorf("--out - writes to the standard output, which can't be watched nor reloaded")
	}
	if t.In == "-" && t.Watch {
		return nil, nil, fmt.Errorf("--in - reads the template from the standard input, which can't be read again on SIGHUP: name a file to watch")
	}
	if err := checkReloadCommand(t.ReloadCommand); err != nil {
		return nil, nil, err
	}
	data := make(map[string]any)
	for _, s := range t.Vars {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return nil, nil, fmt.Errorf("invalid --var %s: key=value is expected", s)
		}
		data[k] = v
	}
	r := &tmplrender.Renderer{
		Name:      t.In,
		Namespace: t.Namespace,
		Data:      data,
	}
	if err := t.readTemplate(r); err != nil {
		return nil, nil, err
	}
	var out *templateOutput
	if t.Out != "-" {
		var err error
		if out, err = newTemplateOutput(t.Out, t.Mode, t.User, t.Group); err != nil {
			return nil, nil, err
		}
		if _, err := out.check(); err != nil {
			return nil, nil, err
		}
	}
	return r, out, nil
}

// readTemplate reads the template of r, and checks it parses.
func (t *CmdTemplate) readTemplate(r *tmplrender.Renderer) error {
	var (
		b   []byte
		err error
	)
	if t.In == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(t.In)
	}
	if err != nil {
		return err
	}
	text := r.Text
	r.Text = string(b)
	if err := r.Parse(); err != nil {
		r.Text = text
		return err
	}
	return nil
}

func (t *CmdTemplate) Run() error {
	r, out, err := t.prepare()
	if err != nil {
		return err
	}
	if t.Watch {
		return t.watch(r, out)
	}
	ctx := context.Background()
	c, err := client.New()
	if err != nil {
		return err
	}
	r.Fetcher = &templateFetcher{c: c}
	data, err := getClusterData(c)
	if err != nil {
		return err
	}
	res, err := r.Render(ctx, tmplrender.NewState(data))
	if err != nil {
		return err
	}
	if out == nil {
		_, err := os.Stdout.Write(res.Out)
		return err
	}
	t.warnMode(out, res)
	changed, err := out.write(res.Out)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	t.logf("%s written", t.Out)
	if t.ReloadCommand == "" {
		return nil
	}
	return runReload(ctx, t.ReloadCommand, t.Out, t.ReloadTimeout)
}

// getClusterData returns the cluster data, as the node serving the api sees
// it.
func getClusterData(c *client.T) (*clusterdump.Data, error) {
	b, err := c.NewGetClusterStatus().Get()
	if err != nil {
		return nil, err
	}
	data := clusterdump.NewData("")
	if err := json.Unmarshal(b, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (t *CmdTemplate) warnMode(out *templateOutput, res tmplrender.Result) {
	if res.ReadsSecret && out.isGroupOrWorldReadable() {
		t.logf("warning: %s holds sec keys, and its mode %04o lets others than its owner read it", t.Out, *out.mode)
	}
}

// logf logs a message on the standard error. A message never holds a value
// the template read.
func (t *CmdTemplate) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, time.Now().Format(time.RFC3339)+" template: "+format+"\n", args...)
}
