package ini

import (
	"strings"
	"testing"
)

var testOptions = Options{
	Loose:                      true,
	AllowPythonMultilineValues: true,
	SpaceBeforeInlineComment:   true,
}

const sample = `# leading comment
; another one

[DEFAULT]
nodes = n1 n2
	n3
id     = 8c4d5b1e-0000-0000-0000-000000000001

# what app#1 does
[app#1]
type = simple
start = /bin/true # inline comment
env = A=1
	B=2
	C=3
secret = abc#def;ghi
empty =

[disk#0]
name = {fqdn}.disk
`

func load(t *testing.T, s string) *File {
	t.Helper()
	f, err := Load(testOptions, []byte(s))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return f
}

func TestWriteBackIsByteIdentical(t *testing.T) {
	f := load(t, sample)
	got, err := f.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if string(got) != sample {
		t.Errorf("write back differs from source:\n--- got ---\n%s\n--- want ---\n%s", got, sample)
	}
}

func TestValues(t *testing.T) {
	f := load(t, sample)
	for _, tc := range []struct {
		section string
		key     string
		want    string
	}{
		{"DEFAULT", "nodes", "n1 n2\n\tn3"},
		{"DEFAULT", "id", "8c4d5b1e-0000-0000-0000-000000000001"},
		{"app#1", "type", "simple"},
		{"app#1", "start", "/bin/true"},
		{"app#1", "env", "A=1\n\tB=2\n\tC=3"},
		{"app#1", "secret", "abc#def;ghi"},
		{"app#1", "empty", ""},
		{"disk#0", "name", "{fqdn}.disk"},
	} {
		if got := f.Section(tc.section).Key(tc.key).Value(); got != tc.want {
			t.Errorf("[%s]%s = %q, want %q", tc.section, tc.key, got, tc.want)
		}
	}
}

func TestSectionStrings(t *testing.T) {
	f := load(t, sample)
	want := []string{"DEFAULT", "app#1", "disk#0"}
	got := f.SectionStrings()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SectionStrings() = %v, want %v", got, want)
	}
}

func TestSetValueKeepsSurroundingBytes(t *testing.T) {
	f := load(t, sample)
	f.Section("app#1").Key("type").SetValue("forking")
	got := f.String()
	if !strings.Contains(got, "# what app#1 does\n[app#1]\ntype = forking\n") {
		t.Errorf("comment or section header lost:\n%s", got)
	}
	// Every other line must be untouched.
	want := strings.Replace(sample, "type = simple", "type = forking", 1)
	if got != want {
		t.Errorf("unrelated bytes changed:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSetValueKeepFormat(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     string
		want    string
		inplace bool
	}{
		{"aligned key keeps its padding", "id", "id     = ********\n", true},
		{"multiline value is replaced whole", "nodes", "nodes = ********\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := load(t, sample)
			k := f.Section("DEFAULT").Key(tc.key)
			if got := k.SetValueKeepFormat("********"); got != tc.inplace {
				t.Fatalf("SetValueKeepFormat() = %v, want %v", got, tc.inplace)
			}
			if k.Value() != "********" {
				t.Errorf("Value() = %q", k.Value())
			}
			if got := f.String(); !strings.Contains(got, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, got)
			}
		})
	}
}

func TestSetValueKeepFormatPreservesInlineComment(t *testing.T) {
	f := load(t, sample)
	f.Section("app#1").Key("start").SetValueKeepFormat("********")
	if got := f.String(); !strings.Contains(got, "start = ******** # inline comment\n") {
		t.Errorf("inline comment lost:\n%s", got)
	}
}

func TestRedactionDoesNotLeakValueTail(t *testing.T) {
	// "abc#def;ghi" is a single value, the inline comment needing a space
	// before its marker. Redacting it must leave nothing of it behind.
	f := load(t, sample)
	f.Section("app#1").Key("secret").SetValueKeepFormat("********")
	got := f.String()
	if strings.Contains(got, "def") || strings.Contains(got, "ghi") {
		t.Errorf("secret tail leaked:\n%s", got)
	}
	if !strings.Contains(got, "secret = ********\n") {
		t.Errorf("unexpected redaction:\n%s", got)
	}
}

func TestNewKeyGoesInItsSection(t *testing.T) {
	f := load(t, sample)
	if _, err := f.Section("app#1").NewKey("stop", "/bin/false"); err != nil {
		t.Fatal(err)
	}
	got := f.String()
	if !strings.Contains(got, "empty =\nstop = /bin/false\n\n[disk#0]") {
		t.Errorf("key not appended at the end of its section:\n%s", got)
	}
}

func TestDeleteKeyDropsItsComment(t *testing.T) {
	f := load(t, "[a]\n# about x\nx = 1\ny = 2\n")
	f.Section("a").DeleteKey("x")
	if got, want := f.String(), "[a]\ny = 2\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDeleteSection(t *testing.T) {
	f := load(t, sample)
	f.DeleteSection("app#1")
	got := f.String()
	if strings.Contains(got, "app#1") {
		t.Errorf("section still present:\n%s", got)
	}
	if !strings.Contains(got, "[disk#0]") {
		t.Errorf("wrong section deleted:\n%s", got)
	}
}

func TestEmptyHasDefaultSection(t *testing.T) {
	f := Empty(testOptions)
	if got, want := strings.Join(f.SectionStrings(), ","), "DEFAULT"; got != want {
		t.Errorf("SectionStrings() = %q, want %q", got, want)
	}
	f.Section("DEFAULT").Key("id").SetValue("x")
	if got, want := f.String(), "[DEFAULT]\nid = x\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBareLeadingKeysKeepNoDefaultHeader(t *testing.T) {
	const src = "nodes = n1\n\n[a]\nx = 1\n"
	f := load(t, src)
	if got := f.String(); got != src {
		t.Errorf("got %q, want %q", got, src)
	}
	if got := f.Section("DEFAULT").Key("nodes").Value(); got != "n1" {
		t.Errorf("nodes = %q", got)
	}
}

func TestChildSectionFallback(t *testing.T) {
	f := load(t, "[a]\nx = 1\n[a.b]\ny = 2\n")
	if got := f.Section("a.b").Key("x").Value(); got != "1" {
		t.Errorf("[a.b]x = %q, want 1", got)
	}
	if !f.Section("a.b").HasKey("x") {
		t.Error("HasKey(x) = false")
	}
}

func TestNoDefaultInheritance(t *testing.T) {
	f := load(t, "[DEFAULT]\nd = 9\n[a]\nx = 1\n")
	if f.Section("a").HasKey("d") {
		t.Error("a key of the default section must not be inherited")
	}
}

func TestSectionAndKeyAreCreatedOnRead(t *testing.T) {
	f := load(t, "[a]\nx = 1\n")
	f.Section("nope")
	if got := strings.Join(f.SectionStrings(), ","); got != "DEFAULT,a,nope" {
		t.Errorf("SectionStrings() = %q", got)
	}
	f.Section("a").Key("absent")
	if got := strings.Join(f.Section("a").KeyStrings(), ","); got != "x,absent" {
		t.Errorf("KeyStrings() = %q", got)
	}
}

func TestEncodeRoundTrips(t *testing.T) {
	for _, value := range []string{
		"", "simple", " padded ", "a#b", "a #b", "a ;b", "a`b", `a"b`,
		"multi\n\tline", "multi\n line\n  deeper", "no\nindent",
		"trailing\\", `"quoted"`, "'quoted'", "a\n\nb", "\n\tleading newline",
	} {
		f := Empty(testOptions)
		f.Section("s").Key("k").SetValue(value)
		b, err := f.Bytes()
		if err != nil {
			t.Errorf("value %q: %v", value, err)
			continue
		}
		g, err := Load(testOptions, b)
		if err != nil {
			t.Errorf("value %q: reload %q: %v", value, b, err)
			continue
		}
		if got := g.Section("s").Key("k").Value(); got != value {
			t.Errorf("value %q written as %q read back as %q", value, b, got)
		}
	}
}

func TestReload(t *testing.T) {
	f := load(t, sample)
	f.Section("app#1").Key("type").SetValue("forking")
	if err := f.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := f.Section("app#1").Key("type").Value(); got != "simple" {
		t.Errorf("type = %q after reload, want simple", got)
	}
	if got := f.String(); got != sample {
		t.Errorf("reload did not restore the source bytes")
	}
}
