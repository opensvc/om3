//go:build linux

package ressharenfs

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/rs/zerolog"
)

type (
	Mounts []Mount
	Mount  struct {
		Path    string
		Clients []string
	}
	Export struct {
		Client string
		Path   string
		Opts   []string
	}
	Exports   []Export
	OptsEntry struct {
		Client string
		Opts   []string
	}
	Opts []OptsEntry
)

func (t Mount) HasClient(s string) bool {
	for _, e := range t.Clients {
		if e == s || e == "*" {
			return true
		}
	}
	return false
}

func (t Mount) IsZero() bool {
	return t.Path == ""
}

func (t Mounts) ByPath(s string) Mounts {
	l := make(Mounts, 0)
	for _, e := range t {
		if e.Path != s {
			continue
		}
		l = append(l, e)
	}
	return l
}

func (t Export) HasOpts(l []string) bool {
	for _, e := range l {
		if !slices.Contains(t.Opts, e) {
			return false
		}
	}
	return true
}

func (t Export) IsZero() bool {
	return t.Path == ""
}

func (t Exports) Client(s string) Export {
	for _, e := range t {
		if e.Client == s {
			return e
		}
	}
	return Export{}
}

func (t Exports) ByPath(s string) Exports {
	l := make(Exports, 0)
	for _, e := range t {
		if e.Path != s {
			continue
		}
		l = append(l, e)
	}
	return l
}

func (t *T) stop() error {
	opts, err := t.parseOpts()
	if err != nil {
		return err
	}
	for _, e := range opts {
		if !slices.Contains(t.issuesNone, e.Client) {
			continue
		}
		if err := t.delExport(e); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) start(ctx context.Context) error {
	opts, err := t.parseOpts()
	if err != nil {
		return err
	}
	for _, e := range opts {
		if slices.Contains(t.issuesNone, e.Client) {
			continue
		}
		if slices.Contains(t.issuesWrongOpts, e.Client) {
			if err := t.delExport(e); err != nil {
				return err
			}
		}
		if err := t.addExport(e); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return t.delExport(e)
		})
	}
	return nil
}

func (t *T) isPathExported() (bool, error) {
	t.issues = make(map[string]string)
	t.issuesMissingClient = make([]string, 0)
	t.issuesWrongOpts = make([]string, 0)
	t.issuesNone = make([]string, 0)
	exports, err := t.getExports()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return false, err
	}
	if len(exports) == 0 {
		return false, nil
	}
	mount, err := t.getShowmount()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return false, err
	}
	if mount.IsZero() {
		t.StatusLog().Info("%s in userland etab but not in kernel etab", t.SharePath)
		return false, nil
	}
	opts, err := t.parseOpts()
	if err != nil {
		return false, err
	}
	for _, opt := range opts {
		client := exports.Client(opt.Client)
		if client.IsZero() {
			t.issues[opt.Client] = fmt.Sprintf("%s not exported to client %s", t.SharePath, opt.Client)
			t.issuesMissingClient = append(t.issuesMissingClient, opt.Client)
		} else if !mount.HasClient(opt.Client) {
			t.issues[opt.Client] = fmt.Sprintf("%s not exported to client %s in kernel etab", t.SharePath, opt.Client)
			t.issuesMissingClient = append(t.issuesMissingClient, opt.Client)
		} else if !client.HasOpts(opt.Opts) {
			t.issues[opt.Client] = fmt.Sprintf("%s is exported to client %s with missing options: current '%s', minimum required '%s'",
				t.SharePath,
				opt.Client,
				strings.Join(client.Opts, ","),
				strings.Join(opt.Opts, ","),
			)
			t.issuesWrongOpts = append(t.issuesWrongOpts, opt.Client)
		} else {
			t.issuesNone = append(t.issuesNone, opt.Client)
		}
	}
	return true, nil
}

func (t *T) parseOpts() (Opts, error) {
	return parseOpts(strings.Fields(t.ShareOpts))
}

func parseOpts(l []string) (o Opts, err error) {
	var e OptsEntry
	for _, one := range l {
		if e, err = parseOptsEntry(one); err != nil {
			return
		} else {
			o = append(o, e)
		}
	}
	return
}

func parseOptsEntry(s string) (e OptsEntry, err error) {
	l := strings.SplitN(s, "(", 2)
	switch len(l) {
	case 2:
		e.Client = l[0]
		s = l[1]
	default:
		err = fmt.Errorf("malformed share opts: '%s'. must be in client(opt,opt) client(opt,opt) format", s)
		return
	}
	s = strings.TrimRight(s, ")")
	e.Opts = strings.Split(s, ",")
	return
}

func (t *T) addExport(e OptsEntry) error {
	opts := strings.Join(e.Opts, ",")
	cmd := command.New(
		command.WithName("exportfs"),
		command.WithVarArgs("-i", "-o", opts, e.Client+":"+t.SharePath),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
		command.WithTimeout(10*time.Second),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) delExport(e OptsEntry) error {
	cmd := command.New(
		command.WithName("exportfs"),
		command.WithVarArgs("-u", e.Client+":"+t.SharePath),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
		command.WithTimeout(10*time.Second),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) getShowmount() (Mount, error) {
	if mounts, err := t.getShowmounts(); err != nil {
		return Mount{}, err
	} else if mounts = mounts.ByPath(t.SharePath); len(mounts) == 0 {
		return Mount{}, nil
	} else {
		return mounts[0], nil
	}
}

func (t *T) getShowmounts() (Mounts, error) {
	cmd := command.New(
		command.WithName("showmount"),
		command.WithVarArgs("-e", "--no-headers", "127.0.0.1"),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("showmount: %w", err)
	}
	mounts := make(Mounts, 0)
	for _, line := range strings.Split(string(out), "\n") {
		if mount, err := parseShowmountLine(line); err != nil {
			return mounts, err
		} else {
			mounts = append(mounts, mount)
		}
	}
	return mounts, nil
}

func (t *T) getExports() (Exports, error) {
	if exports, err := t.getAllExports(); err != nil {
		return nil, err
	} else {
		return exports.ByPath(t.SharePath), nil
	}
}

func (t *T) getAllExports() (Exports, error) {
	cmd := command.New(
		command.WithName("exportfs"),
		command.WithVarArgs("-v"),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	exports := make(Exports, 0)
	for _, line := range strings.Split(string(out), "\n") {
		if export, err := parseExport(line); err == nil {
			exports = append(exports, export)
		}
	}
	return exports, nil
}

func parseShowmountLine(s string) (mount Mount, err error) {
	l := strings.Fields(s)
	if len(l) != 2 {
		err = fmt.Errorf("invalid showmount -e output line format (expected 2 fields): %s", s)
		return
	}
	mount.Path = l[0]
	if l[1] == "(everyone)" {
		mount.Clients = []string{"*"}
	} else {
		mount.Clients = strings.Split(l[1], ",")
	}
	return
}

func parseExport(s string) (Export, error) {
	l := strings.Fields(s)
	if len(l) == 0 {
		return Export{}, fmt.Errorf("invalid exportfs -v output line format (expected 1 client(opt,opt)): %s", s)
	}
	opts, err := parseOpts(l[1:])
	if err != nil {
		return Export{}, err
	}
	if len(opts) != 1 {
		return Export{}, fmt.Errorf("invalid exportfs -v output line format (expected 1 client(opt,opt)): %s", s)
	}
	client := opts[0].Client
	if client == "<world>" {
		client = "*"
	}
	return Export{
		Path:   l[0],
		Client: client,
		Opts:   opts[0].Opts,
	}, nil
}
