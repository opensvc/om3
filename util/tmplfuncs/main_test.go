package tmplfuncs

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuncMap(t *testing.T) {
	cases := map[string]string{
		`{{ "a,b" | split "," | join "-" }}`:      "a-b",
		`{{ "  x " | trim | upper }}`:             "X",
		`{{ "abc" | replace "b" "B" }}`:           "aBc",
		`{{ "" | default "none" }}`:               "none",
		`{{ "v" | default "none" }}`:              "v",
		`{{ "a\nb" | indent 2 }}`:                 "  a\n  b",
		`{{ "a" | nindent 1 }}`:                   "\n a",
		`{{ "x" | quote }}`:                       `"x"`,
		`{{ "hi" | b64enc }}`:                     "aGk=",
		`{{ "aGk=" | b64dec }}`:                   "hi",
		`{{ "" | sha256sum }}`:                    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		`{{ list | toJSON }}`:                     `["b","a"]`,
		`{{ (fromJSON "{\"a\":1}").a }}`:          "1",
		`{{ list | sortAlpha | join "," }}`:       "a,b",
		`{{ fnMatch "web*" "web1" }}`:             "true",
		`{{ reMatch "^w.b" "web" }}`:              "true",
		`{{ "prefix-x" | trimPrefix "prefix-" }}`: "x",
		`{{ hasPrefix "web1" "web" }}`:            "true",
		`{{ dict | toYAML }}`:                     "a: 1",
		`{{ "a b  c" | fields | join "," }}`:      "a,b,c",
	}
	for text, want := range cases {
		funcs := FuncMap()
		funcs["list"] = func() []string { return []string{"b", "a"} }
		funcs["dict"] = func() map[string]int { return map[string]int{"a": 1} }
		tmpl, err := template.New("t").Funcs(funcs).Parse(text)
		require.NoError(t, err, text)
		var buf bytes.Buffer
		require.NoError(t, tmpl.Execute(&buf, nil), text)
		assert.Equal(t, want, buf.String(), text)
	}
}

func TestNoEnvNorExec(t *testing.T) {
	for _, name := range []string{"env", "exec", "readFile"} {
		_, ok := FuncMap()[name]
		assert.False(t, ok, name)
	}
}
