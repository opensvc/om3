package sysreport

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/opensvc/om3/v3/util/hostname"
)

// recorder is a sender listing what each report carries. The archive is read
// during the call, as it is removed once sent.
type recorder struct {
	t       *testing.T
	fail    bool
	calls   int
	members []string
	deleted []string
	full    bool
}

func (r *recorder) SendSysreport(archive string, deleted []string, full bool) error {
	r.calls++
	r.members = nil
	r.deleted = deleted
	r.full = full
	if r.fail {
		return errors.New("refused")
	}
	if archive == "" {
		return nil
	}
	f, err := os.Open(archive)
	if err != nil {
		r.t.Fatalf("open archive: %s", err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.t.Fatalf("read archive: %s", err)
		}
		r.members = append(r.members, hdr.Name)
	}
	sort.Strings(r.members)
	return nil
}

func (r *recorder) has(member string) bool {
	for _, m := range r.members {
		if m == member {
			return true
		}
	}
	return false
}

func run(t *testing.T, root, tracked string, rec *recorder, force bool) {
	t.Helper()
	if err := runErr(root, tracked, rec, force); err != nil {
		t.Fatalf("sysreport: %s", err)
	}
}

func runErr(root, tracked string, rec *recorder, force bool) error {
	sr := New(filepath.Join(root, "etc"), filepath.Join(root, "var"))
	sr.SetConfigReader(strings.NewReader("FILE " + tracked + "\n"))
	sr.SetSender(rec)
	sr.SetForce(force)
	return sr.Do()
}

// A first report is a full one, a change sends the file again, a deletion is
// sent alone, with no archive when nothing changed, and a forced report is a
// full one.
func TestSysreportSends(t *testing.T) {
	root := t.TempDir()
	tracked := filepath.Join(root, "data", "tracked.conf")
	if err := os.MkdirAll(filepath.Dir(tracked), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	member := hostname.Hostname() + "/file" + tracked
	rec := &recorder{t: t}

	run(t, root, tracked, rec, false)
	if rec.calls != 1 || !rec.has(member) || !rec.full || len(rec.deleted) != 0 {
		t.Fatalf("first report: calls %d, members %v, full %v, deleted %v", rec.calls, rec.members, rec.full, rec.deleted)
	}

	if err := os.WriteFile(tracked, []byte("v2, longer"), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, root, tracked, rec, false)
	if rec.calls != 2 || !rec.has(member) || rec.full {
		t.Fatalf("changed report: calls %d, members %v", rec.calls, rec.members)
	}

	if err := os.Remove(tracked); err != nil {
		t.Fatal(err)
	}
	run(t, root, tracked, rec, false)
	if rec.calls != 3 || rec.has(member) || len(rec.deleted) != 1 || rec.deleted[0] != tracked {
		t.Fatalf("deletion report: calls %d, members %v, deleted %v", rec.calls, rec.members, rec.deleted)
	}

	if err := os.WriteFile(tracked, []byte("v3"), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, root, tracked, rec, true)
	if rec.calls != 4 || !rec.full || !rec.has(member) || len(rec.deleted) != 0 {
		t.Fatalf("forced report: calls %d, full %v, members %v, deleted %v", rec.calls, rec.full, rec.members, rec.deleted)
	}
}

// A report without a sender fails rather than collecting for nothing.
func TestSysreportNeedsASender(t *testing.T) {
	root := t.TempDir()
	sr := New(filepath.Join(root, "etc"), filepath.Join(root, "var"))
	sr.SetConfigReader(strings.NewReader(""))
	if err := sr.Do(); err == nil {
		t.Fatal("a sysreport without a sender succeeded")
	}
}

// A report the collector refuses leaves the files it carried unreported, and
// the next report is a full one, which carries them.
func TestSysreportAfterAFailedReport(t *testing.T) {
	root := t.TempDir()
	tracked := filepath.Join(root, "data", "tracked.conf")
	if err := os.MkdirAll(filepath.Dir(tracked), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	member := hostname.Hostname() + "/file" + tracked
	rec := &recorder{t: t}
	run(t, root, tracked, rec, false)

	if err := os.WriteFile(tracked, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	rec.fail = true
	if err := runErr(root, tracked, rec, false); err == nil {
		t.Fatal("a refused report succeeded")
	}

	rec.fail = false
	run(t, root, tracked, rec, false)
	if !rec.full || !rec.has(member) {
		t.Fatalf("report after a failure: full %v, members %v", rec.full, rec.members)
	}

	// Nothing changed since the accepted report: nothing is sent, not a
	// full report again.
	calls := rec.calls
	run(t, root, tracked, rec, false)
	if rec.calls != calls {
		t.Fatalf("the report after an accepted one sent again, full %v", rec.full)
	}
}

// A report with nothing changed sends nothing, a second later too: the
// metadata recorded is the tracked file's, not the collected copy's, whose
// change time moves at each collection.
func TestSysreportUnchangedSendsNothing(t *testing.T) {
	root := t.TempDir()
	tracked := filepath.Join(root, "data", "tracked.conf")
	if err := os.MkdirAll(filepath.Dir(tracked), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{t: t}
	run(t, root, tracked, rec, false)
	time.Sleep(1100 * time.Millisecond)
	calls := rec.calls
	run(t, root, tracked, rec, false)
	if rec.calls != calls {
		t.Fatalf("an unchanged report sent %v", rec.members)
	}
}
