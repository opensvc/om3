package ini

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	upstream "github.com/cvaroqui/ini"
)

// The package this one replaces, kept as a test only dependency to check that
// the two read the same values out of the same bytes. It is dropped once this
// package has been in production long enough.
var upstreamOptions = upstream.LoadOptions{
	Loose:                      true,
	AllowPythonMultilineValues: true,
	SpaceBeforeInlineComment:   true,
}

// corpus holds documents covering the syntax the OpenSVC configuration files
// use, and the corners the hand written redaction scanner used to get wrong.
var corpus = []string{
	sample,
	"",
	"\n",
	"[a]\n",
	"[a]\nx = 1\n",
	"x = 1\n",
	"# only a comment\n",
	"\n\n\n",
	"[a]\nx=1\n",
	"[a]\nx:1\n",
	"[a]\nx = \n",
	"[a]\nx =\n",
	"[a]\nx = 1 # comment\n",
	"[a]\nx = 1 ; comment\n",
	"[a]\nx = a#b\n",
	"[a]\nx = a;b\n",
	"[a]\nx = a#b;c\n",
	"[a]\nx = one\n\ttwo\n\tthree\n",
	"[a]\nx = one\n  two\n",
	"[a]\nx =\n\tcontinued\n",
	"[a]\nx = \"\"\"multi\nline\"\"\"\n",
	"[a]\nx = `back ticked`\n",
	"[a]\nx = \"quoted\"\n",
	"[a]\nx = 'quoted'\n",
	"[a]\nx = trailing\\\ncontinued\n",
	"[DEFAULT]\nnodes = n1\n[a]\nx = 1\n",
	"[a]\nx = 1\n[a.b]\ny = 2\n",
	"[a]\n\n# comment\n\nx = 1\n\n",
	"[a]\nx = 1\n[a]\ny = 2\n",
	"[a]\nx = 1\nx = 2\n",
	"[a]\n  indented = 1\n",
	"[a]\nx = {fqdn}.disk\n",
	"[a]\nx = 1\r\ny = 2\r\n",
	"[a]\nx = a b  c\n",
	"[a]\nempty =\ny = 2\n",
	"[a]\nx = 1",
	"[a]\nx = one\n\ttwo",
}

// tree is the reading of a document: its sections, their keys and the values,
// all in order.
func tree(sections []string, keys [][2]string) string {
	var b strings.Builder
	for _, s := range sections {
		fmt.Fprintf(&b, "[%s]\n", s)
	}
	for _, kv := range keys {
		fmt.Fprintf(&b, "%s=%q\n", kv[0], kv[1])
	}
	return b.String()
}

func ourTree(f *File) string {
	var sections []string
	var keys [][2]string
	for _, s := range f.Sections() {
		sections = append(sections, s.Name())
		for _, k := range s.Keys() {
			keys = append(keys, [2]string{s.Name() + "." + k.Name(), k.Value()})
		}
	}
	return tree(sections, keys)
}

func upstreamTree(f *upstream.File) string {
	var sections []string
	var keys [][2]string
	for _, s := range f.Sections() {
		sections = append(sections, s.Name())
		for _, k := range s.Keys() {
			keys = append(keys, [2]string{s.Name() + "." + k.Name(), k.Value()})
		}
	}
	return tree(sections, keys)
}

// TestReadsTheSameValuesAsUpstream is the check that this package did not
// change the meaning of the configuration files already in production.
func TestReadsTheSameValuesAsUpstream(t *testing.T) {
	for i, src := range corpus {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			want, wantErr := upstream.LoadSources(upstreamOptions, []byte(src))
			got, gotErr := Load(testOptions, []byte(src))
			if (wantErr != nil) != (gotErr != nil) {
				t.Fatalf("source %q: upstream err=%v, ours err=%v", src, wantErr, gotErr)
			}
			if wantErr != nil {
				return
			}
			if a, b := upstreamTree(want), ourTree(got); a != b {
				t.Errorf("source %q:\n--- upstream ---\n%s\n--- ours ---\n%s", src, a, b)
			}
		})
	}
}

func TestCorpusWriteBackIsByteIdentical(t *testing.T) {
	for i, src := range corpus {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			f, err := Load(testOptions, []byte(src))
			if err != nil {
				t.Skipf("not parseable: %v", err)
			}
			got, err := f.Bytes()
			if err != nil {
				t.Fatalf("Bytes: %v", err)
			}
			if string(got) == src {
				return
			}
			// A document defining a key twice is normalised to the last
			// definition. Everything else must be given back untouched.
			if hasDuplicateKey(src) {
				t.Skip("document redefines a key")
			}
			t.Errorf("source %q written back as %q", src, got)
		})
	}
}

func hasDuplicateKey(src string) bool {
	f, err := Load(testOptions, []byte(src))
	if err != nil {
		return false
	}
	n := 0
	for _, s := range f.Sections() {
		n += len(s.Keys())
	}
	lines := 0
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		if strings.ContainsAny(line, "=:") {
			lines++
		}
	}
	return lines > n
}

func FuzzAgainstUpstream(f *testing.F) {
	for _, src := range corpus {
		f.Add([]byte(src))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Beyond its read buffer, upstream silently truncates a value
		// continued on the following lines. Stay well under it.
		if len(data) > 2048 {
			t.Skip()
		}
		// Upstream decodes UTF-16, this package refuses it. A UTF-16
		// configuration file could not be written back byte for byte, and
		// OpenSVC never writes one.
		if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) || bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
			t.Skip()
		}
		want, wantErr := upstream.LoadSources(upstreamOptions, data)
		got, gotErr := Load(testOptions, data)
		if wantErr != nil || gotErr != nil {
			if (wantErr != nil) != (gotErr != nil) {
				t.Fatalf("data %q: upstream err=%v, ours err=%v", data, wantErr, gotErr)
			}
			return
		}
		if a, b := upstreamTree(want), ourTree(got); a != b {
			t.Fatalf("data %q:\n--- upstream ---\n%s\n--- ours ---\n%s", data, a, b)
		}
	})
}

// FuzzWriteBack checks the two properties the whole package exists for: what
// is written parses back to what was read, and writing is stable.
func FuzzWriteBack(f *testing.F) {
	for _, src := range corpus {
		f.Add([]byte(src))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		first, err := Load(testOptions, data)
		if err != nil {
			t.Skip()
		}
		out1, err := first.Bytes()
		if err != nil {
			// A document holding a value with no writable form is reported
			// rather than silently written wrong. That is the contract.
			t.Skip()
		}
		second, err := Load(testOptions, out1)
		if err != nil {
			t.Fatalf("data %q written as %q does not parse back: %v", data, out1, err)
		}
		if a, b := ourTree(first), ourTree(second); a != b {
			t.Fatalf("data %q written as %q reads back differently:\n--- before ---\n%s\n--- after ---\n%s", data, out1, a, b)
		}
		out2, err := second.Bytes()
		if err != nil {
			t.Fatalf("data %q: %v", data, err)
		}
		if string(out1) != string(out2) {
			t.Fatalf("data %q: writing is not stable: %q then %q", data, out1, out2)
		}
	})
}

// FuzzRedact checks that replacing a value in place never leaves a piece of it
// behind, which is what the redaction of a secret needs.
func FuzzRedact(f *testing.F) {
	for _, src := range corpus {
		f.Add([]byte(src), "s3cr3t")
	}
	f.Fuzz(func(t *testing.T, data []byte, secret string) {
		const marker = "********"
		if secret == "" || len(data) > 2048 {
			t.Skip()
		}
		// A secret the marker itself holds could not be told from it.
		if strings.Contains(marker, secret) {
			t.Skip()
		}
		doc, err := Load(testOptions, data)
		if err != nil {
			t.Skip()
		}
		// Plant the secret in every key, then redact every key.
		for _, s := range doc.Sections() {
			for _, k := range s.Keys() {
				k.SetValue(secret)
			}
		}
		planted, err := doc.Bytes()
		if err != nil {
			t.Skip()
		}
		doc, err = Load(testOptions, planted)
		if err != nil {
			t.Fatalf("planted %q: %v", planted, err)
		}
		for _, s := range doc.Sections() {
			for _, k := range s.Keys() {
				if k.Value() != secret {
					t.Fatalf("planted %q: [%s]%s = %q, want %q", planted, s.Name(), k.Name(), k.Value(), secret)
				}
				k.SetValueKeepFormat(marker)
			}
		}
		out, err := doc.Bytes()
		if err != nil {
			t.Skip()
		}
		// The secret may legitimately survive in a comment or a section name
		// of the source document, which are not values. Check the values.
		back, err := Load(testOptions, out)
		if err != nil {
			t.Fatalf("redacted %q does not parse: %v", out, err)
		}
		for _, s := range back.Sections() {
			for _, k := range s.Keys() {
				if strings.Contains(k.Value(), secret) {
					t.Fatalf("planted %q redacted as %q still holds the secret in [%s]%s = %q", planted, out, s.Name(), k.Name(), k.Value())
				}
			}
		}
	})
}
