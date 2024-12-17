package actionrouter

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/monitor"
	"github.com/opensvc/om3/core/naming"
)

type (
	// T holds the action options common to all actioner implementations.
	T struct {
		Digest bool

		//
		// ObjectSelector expands into a selection of objects to execute
		// the action on.
		//
		ObjectSelector string

		//
		// NodeSelector expands into a selection of nodes to execute the
		// action on.
		//
		NodeSelector string

		RID    string
		Subset string
		Tag    string

		//
		// Local routes the action to the CRM instead of remoting it via
		// orchestration or remote execution.
		//
		Local bool

		//
		// DefaultIsLocal makes actions not explicitely Local nor remoted
		// via NodeSelector be treated as local (CRM level).
		//
		DefaultIsLocal bool

		//
		// Flags is the command flags as parsed by cobra. This is the struct
		// passed to the object method on local execution.
		//
		Flags any

		//
		// Target is the node or object state the daemons should orchestrate
		// to reach.
		//
		Target string

		//
		// TargetOptions is the options of the orchestration needed to reach
		// the Target.
		//
		TargetOptions any

		// Wait runs an event watcher to wait for target state or global expect reached
		Wait bool

		// WaitDuration is the maximum duration allowed for the Wait
		WaitDuration time.Duration

		//
		// Watch runs a event-driven monitor on the selected objects after
		// setting a new target. So the operator can see the orchestration
		// unfolding.
		//
		Watch bool

		//
		// Output controls the output data format.
		// <empty>   => human readable format
		// tab=...   => tubular customizable format
		// yaml      => yaml machine readable format
		// json      => json machine readable format
		// flat      => flattened json (<k>=<v>) machine readable format
		// flat_json => same as flat (backward compat)
		//
		Output string

		// DefaultOutput defines a default output to use when Output is
		// not specified.
		DefaultOutput string

		//
		// Color activates the colorization of outputs
		// auto => yes if os.Stdout is a tty
		// yes
		// no
		//
		Color string

		//
		// Server bypasses the agent api requester automatic selection. It
		// Accepts a uri where the scheme can be:
		// raw   => jsonrpc
		// http  => http/2 cleartext (over unix domain socket only)
		// https => http/2 with TLS
		// tls   => http/2 with TLS
		//
		Server string
	}

	// Actioner is the interface implemented by nodeaction.T and objectaction.T
	Actioner interface {
		DoRemote() error
		DoLocal() error
		DoAsync() error
		HasLocal() bool
		Options() T
	}

	// Result is a predictible type of actions return value, for reflect.
	Result struct {
		Nodename      string        `json:"nodename"`
		Path          naming.Path   `json:"path,omitempty"`
		Data          interface{}   `json:"data"`
		Error         error         `json:"-"`
		Panic         interface{}   `json:"-"`
		HumanRenderer func() string `json:"-"`
	}

	// renderer is implemented by data type stored in ActionResults.Data.
	renderer interface {
		Render() string
	}
)

func (t Result) Unstructured() map[string]any {
	return map[string]any{
		"nodename": t.Nodename,
		"path":     t.Path.String(),
		"data":     t.Data,
	}
}

// Do is the switch method between local, remote or async mode.
// If Watch is set, end up starting a monitor on the selected objects.
func Do(t Actioner) error {
	var errs error
	o := t.Options()
	switch {
	case o.NodeSelector != "":
		errs = t.DoRemote()
	case t.HasLocal() && (o.Local || o.DefaultIsLocal || o.RID != "" || o.Subset != "" || o.Tag != ""):
		errs = t.DoLocal()
	case o.Target != "":
		errs = t.DoAsync()
	case !clientcontext.IsSet() && t.HasLocal():
		errs = t.DoLocal()
	default:
		// post action on context endpoint
		errs = t.DoRemote()
	}
	if o.Watch {
		m := monitor.New()
		m.SetColor(o.Color)
		m.SetFormat(o.Output)
		m.SetSelector(o.ObjectSelector)
		cli, e := client.New(client.WithURL(o.Server), client.WithTimeout(0))
		if e != nil {
			_, _ = fmt.Fprintln(os.Stderr, e)
			return e
		}
		statusGetter := cli.NewGetDaemonStatus().SetSelector(o.ObjectSelector)
		evReader, err := cli.NewGetEvents().SetSelector(o.ObjectSelector).GetReader()
		errs = errors.Join(errs, err)
		err = m.DoWatch(statusGetter, evReader, os.Stdout)
		errs = errors.Join(errs, err)
	}
	return errs
}

func DefaultHumanRenderer(data interface{}) string {
	if data == nil {
		return ""
	}
	switch v := data.(type) {
	case renderer:
		return v.Render()
	case *time.Duration:
		if v == nil {
			// for example, ParseDuration() error on "eval --kw validity"
			return ""
		}
		return v.String() + "\n"
	case fmt.Stringer:
		return v.String()
	case string:
		return v + "\n"
	case []string:
		s := ""
		for _, e := range v {
			s += e + "\n"
		}
		return s
	case []byte:
		return string(v)
	default:
		return fmt.Sprintln(v)
	}
}
