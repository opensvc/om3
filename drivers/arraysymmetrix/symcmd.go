package arraysymmetrix

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

type (
	// runFunc runs the symcli program bin, "symdev" for example, with args,
	// and returns what it wrote and its exit code. err is for a program that
	// could not be run at all: a program exiting non-zero ran, and its exit
	// code says how it ended.
	//
	// A test sets one on the array to answer as an array would, with no
	// array and no symcli installed.
	runFunc func(ctx context.Context, bin string, args []string) (stdout, stderr []byte, exitCode int, err error)

	// xmlNode is an element of a symcli xml output, read without knowing
	// its schema, for the outputs v2 read the same way.
	xmlNode struct {
		XMLName xml.Name
		Content string    `xml:",chardata"`
		Nodes   []xmlNode `xml:",any"`
	}
)

// run runs a symcli program, through the runner a test set when it set one.
//
// The command is never confirmed on a prompt, unless a developer set
// PromptReader: an action reaches here from the collector, with no stdin to
// answer and a stdout read as the json result of the action.
func (t *Array) run(ctx context.Context, level zerolog.Level, bin string, args []string) ([]byte, []byte, int, error) {
	if t.runner != nil {
		return t.runner(ctx, bin, args)
	}
	dir := t.kwSymcliPath()
	if !filepath.IsAbs(dir) {
		return nil, nil, -1, fmt.Errorf("symcli_path %s is not an absolute path", dir)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithPrompt(PromptReader),
		command.WithName(filepath.Join(dir, bin)),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(level),
		command.WithLogLevel(level),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	var exitErr *command.ErrExitCode
	switch {
	case errors.As(err, &exitErr):
		return cmd.Stdout(), cmd.Stderr(), cmd.ExitCode(), nil
	case err != nil:
		return cmd.Stdout(), cmd.Stderr(), -1, err
	default:
		return cmd.Stdout(), cmd.Stderr(), 0, nil
	}
}

// symResult runs a symcli program and returns its outcome as a step of a
// plan, whatever its exit code. The error is for a program that could not
// be run at all.
func (t *Array) symResult(ctx context.Context, level zerolog.Level, bin string, args ...string) (Result, error) {
	result := Result{Cmd: append([]string{bin}, args...)}
	stdout, stderr, exitCode, err := t.run(ctx, level, bin, args)
	result.Ret = exitCode
	result.Out = string(stdout)
	result.Err = string(stderr)
	if err != nil {
		if result.Ret == 0 {
			result.Ret = -1
		}
		if result.Err == "" {
			result.Err = err.Error()
		}
		return result, fmt.Errorf("%s: %w", strings.Join(result.Cmd, " "), err)
	}
	return result, nil
}

// sym runs a symcli program changing the array, and returns its outcome.
//
// A program exiting non-zero is an error, naming the command and carrying
// what it said, so a refusal of the array is never read as a success.
func (t *Array) sym(ctx context.Context, bin string, args ...string) (Result, error) {
	return t.symAt(ctx, zerolog.InfoLevel, bin, args...)
}

func (t *Array) symAt(ctx context.Context, level zerolog.Level, bin string, args ...string) (Result, error) {
	result, err := t.symResult(ctx, level, bin, args...)
	if err != nil {
		return result, err
	}
	if result.Ret != 0 {
		return result, result.failure()
	}
	return result, nil
}

// symOn runs a symcli program changing the array sid.
func (t *Array) symOn(ctx context.Context, bin, sid string, args ...string) (Result, error) {
	if sid == "" {
		return Result{}, errNoSID
	}
	return t.sym(ctx, bin, append([]string{"-sid", sid}, args...)...)
}

// symXML runs a symcli program reading the array sid, in the xml output
// format, and returns that output.
func (t *Array) symXML(ctx context.Context, bin, sid string, args ...string) ([]byte, error) {
	if sid == "" {
		return nil, errNoSID
	}
	args = append([]string{"-sid", sid, "-output", "xml_e"}, args...)
	result, err := t.symAt(ctx, zerolog.TraceLevel, bin, args...)
	if err != nil {
		return nil, err
	}
	return []byte(result.Out), nil
}

var errNoSID = errors.New("no array serial: set the name keyword of the array section to the array sid")

// failure is the error of a step that exited non-zero.
func (t Result) failure() error {
	msg := strings.TrimSpace(t.Err)
	if msg == "" {
		msg = strings.TrimSpace(t.Out)
	}
	return fmt.Errorf("%s: exit code %d: %s", strings.Join(t.Cmd, " "), t.Ret, msg)
}

// SymEnv returns the environment of the symcli commands.
//
// v2 made symcli wait for the locks of the database and of the gatekeepers
// rather than fail on them, and let commands run in parallel on the array,
// unless the environment already said otherwise. The same is done here: an
// action failing on a lock another one held for a second is a disk the
// collector form fails to allocate for nothing.
func (t *Array) SymEnv() []string {
	var l []string
	if s := t.kwSymcliConnect(); s != "" {
		l = append(l, "SYMCLI_CONNECT="+s)
	}
	for _, kv := range [][2]string{
		{"SYMCLI_WAIT_ON_DB", "1"},
		{"SYMCLI_WAIT_ON_GK", "1"},
		{"SYMCLI_CTL_ACCESS", "PARALLEL"},
	} {
		if _, ok := os.LookupEnv(kv[0]); ok {
			continue
		}
		l = append(l, kv[0]+"="+kv[1])
	}
	return l
}

// parseXMLNode returns the root element of a symcli xml output.
func parseXMLNode(b []byte) (xmlNode, error) {
	var n xmlNode
	if err := xml.Unmarshal(b, &n); err != nil {
		return n, err
	}
	return n, nil
}

// findAll returns the elements named name under n, at any depth, in
// document order, as v2 found them with ElementTree.iter.
func (t xmlNode) findAll(name string) []xmlNode {
	var l []xmlNode
	for _, child := range t.Nodes {
		if child.XMLName.Local == name {
			l = append(l, child)
		}
		l = append(l, child.findAll(name)...)
	}
	return l
}

// childText returns the text of the first child element named name, or "".
func (t xmlNode) childText(name string) string {
	for _, child := range t.Nodes {
		if child.XMLName.Local == name {
			return strings.TrimSpace(child.Content)
		}
	}
	return ""
}

// v2Map returns the children of the element the way v2 read them: an
// element holding elements is a map of them, any other one is its text, and
// an element repeated keeps its last value.
//
// Every leaf is a string, which is what the collector expects of the data it
// reads back and joins into the commands it chains.
func (t xmlNode) v2Map() map[string]any {
	m := make(map[string]any)
	for _, child := range t.Nodes {
		if len(child.Nodes) > 0 || strings.HasPrefix(child.Content, "\n") {
			m[child.XMLName.Local] = child.v2Map()
		} else {
			m[child.XMLName.Local] = child.Content
		}
	}
	return m
}

// countElements returns how many elements named name the xml document b
// holds, at any depth.
func countElements(b []byte, name string) (int, error) {
	d := xml.NewDecoder(bytes.NewReader(b))
	n := 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		if e, ok := tok.(xml.StartElement); ok && e.Name.Local == name {
			n++
		}
	}
}

// UnmarshalXML keeps the pairing as v2 read it.
func (t *RDF) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var n xmlNode
	if err := d.DecodeElement(&n, &start); err != nil {
		return err
	}
	t.raw = n.v2Map()
	return nil
}

// MarshalJSON renders the pairing as v2 rendered it.
func (t RDF) MarshalJSON() ([]byte, error) {
	if t.raw == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(t.raw)
}

// get returns the text of the element at path, or "" when there is none.
func (t *RDF) get(path ...string) string {
	if t == nil {
		return ""
	}
	m := t.raw
	for i, name := range path {
		v, ok := m[name]
		if !ok {
			return ""
		}
		if i == len(path)-1 {
			s, _ := v.(string)
			return s
		}
		if m, ok = v.(map[string]any); !ok {
			return ""
		}
	}
	return ""
}

func (t *RDF) has(name string) bool {
	if t == nil {
		return false
	}
	_, ok := t.raw[name].(map[string]any)
	return ok
}

// PairState is the state of the pair, "Synchronized" or "Suspended" for
// example.
func (t *RDF) PairState() string { return t.get("RDF_Info", "pair_state") }

// RemoteWWN is the wwn of the device paired with this one, on the remote
// array.
func (t *RDF) RemoteWWN() string { return t.get("Remote", "wwn") }

// RAGroupNum is the number of the RDF group of the pair on this array.
func (t *RDF) RAGroupNum() string { return t.get("Local", "ra_group_num") }

// LocalType is the role of the device in the pair, "R1" or "R2".
func (t *RDF) LocalType() string { return t.get("Local", "type") }

// RemoteDev is the device paired with this one.
func (t *RDF) RemoteDev() string { return t.get("Remote", "dev_name") }

// RemoteSID is the array of the device paired with this one.
func (t *RDF) RemoteSID() string { return t.get("Remote", "remote_symid") }

// Mode is the mirroring mode of the pair, "Synchronous" for example.
func (t *RDF) Mode() string { return t.get("Mode", "mode") }

// paired returns the SRDF pairing of the device, or nil when it is not
// paired.
//
// A device is paired when its RDF element names both a local and a remote
// member, which is the test v2 made before acting on a pair.
func (t Device) paired() *RDF {
	if t.RDF == nil || !t.RDF.has("Local") || !t.RDF.has("Remote") {
		return nil
	}
	return t.RDF
}
