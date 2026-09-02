package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/andreazorzetto/yh/highlight"
	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/yaml"

	"github.com/opensvc/om3/v3/util/flatten"
	"github.com/opensvc/om3/v3/util/render"
	"github.com/opensvc/om3/v3/util/render/palette"
)

type (
	// RenderFunc is the protype of human format renderer functions.
	RenderFunc func() string

	// Renderer hosts the renderer options and data, and exposes the rendering
	// method.
	Renderer struct {
		DefaultOutput string
		Output        string
		Color         string
		Data          any
		HumanRenderer RenderFunc
		Colorize      *palette.ColorPaletteFunc
		Stream        bool
	}

	renderer interface {
		Render() string
	}

	getItemser interface {
		GetItems() any
	}
)

var (
	indent              = "    "
	regexpJSONKey       = regexp.MustCompile(`(".+":)`)
	regexpJSONReference = regexp.MustCompile(`({[\w.#-_:]+})`)
	regexpJSONScope     = regexp.MustCompile(`(@.+)(":)`)
	regexpJSONErrors    = regexp.MustCompile(`(")(down|stdby down|err|error)(")`)
	regexpJSONOptimal   = regexp.MustCompile(`(")(up|stdby up|ok)(")`)
	regexpJSONWarning   = regexp.MustCompile(`(")(warn)(")`)
	regexpJSONSecondary = regexp.MustCompile(`(")(n/a)(")`)

	regexpANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// Sprint returns the string representation of the data in one of the
// supported format (json, flat, human, ...).
//
// The human format needs a RenderFunc to be passed.
func (t Renderer) Sprint() (string, error) {
	var (
		options, format string
	)
	if t.DefaultOutput != "" {
		if t.Output == "auto" {
			t.Output = t.DefaultOutput
		}
		if strings.HasPrefix(t.Output, "+") {
			t.Output = t.DefaultOutput + "," + t.Output[1:]
		}
	}
	if i := strings.Index(t.Output, "="); i > 0 {
		options = t.Output[i+1:]
		format = t.Output[:i]
	} else {
		format = t.Output
	}
	formatID := toID[format]

	render.SetColor(t.Color)
	if t.Colorize == nil {
		t.Colorize = palette.DefaultFuncPalette()
	}
	switch data := t.Data.(type) {
	case []string:
		if data == nil {
			// JSON Marshal renders "null" for unallocated empty slices
			t.Data = make([]string, 0)
		}
	}
	switch formatID {
	case Flat:
		var sep string
		if t.Stream {
			sep = "---\n"
		}
		b, err := json.Marshal(t.Data)
		if err != nil {
			panic(err)
		}
		if color.NoColor {
			return sep + flatten.SprintFlat(b), nil
		} else {
			return sep + flatten.SprintFlatColor(b, t.Colorize), nil
		}
	case JSON:
		b, err := json.MarshalIndent(t.Data, "", indent)
		if err != nil {
			return "", err
		}
		s := string(b) + "\n"
		s = regexpJSONKey.ReplaceAllString(s, t.Colorize.Primary("$1"))
		s = regexpJSONReference.ReplaceAllString(s, t.Colorize.Optimal("$1"))
		s = regexpJSONScope.ReplaceAllString(s, t.Colorize.Error("$1")+"$2")
		s = regexpJSONErrors.ReplaceAllString(s, "$1"+t.Colorize.Error("$2")+"$3")
		s = regexpJSONOptimal.ReplaceAllString(s, "$1"+t.Colorize.Optimal("$2")+"$3")
		s = regexpJSONWarning.ReplaceAllString(s, "$1"+t.Colorize.Warning("$2")+"$3")
		s = regexpJSONSecondary.ReplaceAllString(s, "$1"+t.Colorize.Secondary("$2")+"$3")
		return s, nil
	case JSONLine:
		b, err := json.Marshal(t.Data)
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	case YAML:
		var sep string
		if t.Stream {
			sep = "---\n"
		}
		b, err := yaml.Marshal(t.Data)
		if err != nil {
			return "", err
		}
		if color.NoColor {
			return string(b) + sep, nil
		} else {
			buf := bytes.NewBuffer(b)
			s, err := highlight.Highlight(buf)
			if err != nil {
				return "", err
			}
			return s + sep, nil
		}
	case Tab:
		s, err := t.renderTab(options)
		if err != nil {
			return "", err
		}
		return s, nil
	case Template:
		s, err := t.renderTemplate(options)
		if err != nil {
			return "", err
		}
		return s, nil
	default:
		if t.HumanRenderer != nil {
			return t.HumanRenderer(), nil
		}
		if r, ok := t.Data.(renderer); ok {
			return r.Render(), nil
		}
		b, err := json.MarshalIndent(t.Data, "", indent)
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	}
}

// unstructureder is the view a type composes for a tab render, when
// its columns are not its fields.
type unstructureder interface {
	Unstructured() map[string]any
}

var jsonRegexp = regexp.MustCompile(`^\{\.?([^{}]+)\}$|^\.?([^{}]+)$`)

// RelaxedJSONPathExpression attempts to be flexible with JSONPath expressions, it accepts:
//   - metadata.name (no leading '.' or curly braces '{...}'
//   - {metadata.name} (no leading '.')
//   - .metadata.name (no curly braces '{...}')
//   - {.metadata.name} (complete expression)
//
// And transforms them all into a valid jsonpath expression:
//
//	{.metadata.name}
func RelaxedJSONPathExpression(pathExpression string) (string, error) {
	if len(pathExpression) == 0 {
		return pathExpression, nil
	}
	submatches := jsonRegexp.FindStringSubmatch(pathExpression)
	if submatches == nil {
		return "", fmt.Errorf("unexpected path string, expected a 'name1.name2' or '.name1.name2' or '{name1.name2}' or '{.name1.name2}'")
	}
	if len(submatches) != 3 {
		return "", fmt.Errorf("unexpected submatch list: %v", submatches)
	}
	var fieldSpec string
	if len(submatches[1]) != 0 {
		fieldSpec = submatches[1]
	} else {
		fieldSpec = submatches[2]
	}
	return fmt.Sprintf("{.%s}", fieldSpec), nil
}

// indirect returns the value a cell is about, which for the optional
// fields of the api types is what their pointer points at. It reports
// false when there is nothing behind it.
func indirect(v reflect.Value) (reflect.Value, bool) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v, false
		}
		v = v.Elem()
	}
	return v, v.IsValid()
}

// rows returns the values a tab render makes a row of.
//
// They are handed to the jsonpath as they are, typed. The jsonpath
// resolves a struct field by its json tag, which is the name the tab
// expressions already use, so nothing has to restate the types to be
// selectable: what the api sends and what a tab render selects cannot
// drift apart.
//
// A type whose columns are not its fields, the heartbeat table entry
// and its beating and stale words for example, implements Unstructured
// and is rendered from the map it returns. That is what the interface
// is for now: a view the type composes, not a copy of the type.
func rows(data any) []any {
	if data == nil {
		return nil
	}
	switch reflect.TypeOf(data).Kind() {
	case reflect.Slice, reflect.Array:
		v := reflect.ValueOf(data)
		l := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			l = append(l, row(v.Index(i).Interface()))
		}
		return l
	default:
		return []any{row(data)}
	}
}

func row(v any) any {
	if i, ok := v.(unstructureder); ok {
		return i.Unstructured()
	}
	return v
}

func (t Renderer) renderTab(options string) (string, error) {
	var (
		hasHeader bool
	)
	jsonPaths := make([]*jsonpath.JSONPath, 0)
	headers := make([]string, 0)
	for _, option := range strings.Split(options, ",") {
		l := strings.Split(option, ":")
		var header, jp string
		switch len(l) {
		case 2:
			header = l[0]
			jp = l[1]
		case 1:
			jp = option
		default:
			continue
		}
		if rjp, err := RelaxedJSONPathExpression(jp); err != nil {
			return "", err
		} else {
			jp = rjp
		}
		jsonPath := jsonpath.New(option)
		if err := jsonPath.Parse(jp); err != nil {
			return "", err
		}
		headers = append(headers, header)
		jsonPaths = append(jsonPaths, jsonPath)
		if header != "" {
			hasHeader = true
		}
	}
	var data any
	if i, ok := t.Data.(getItemser); ok {
		data = i.GetItems()
	} else {
		data = t.Data
	}
	lines := rows(data)
	w := bytes.NewBuffer([]byte{})
	needAlign := len(jsonPaths) > 1

	calculateColumnWidths := func(rows [][]string) []int {
		widths := make([]int, len(rows[0]))
		for _, row := range rows {
			for i, cell := range row {
				plain := regexpANSI.ReplaceAllString(cell, "")
				w := runewidth.StringWidth(plain)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
		return widths
	}

	sprintAligned := func(rows [][]string) {
		if len(rows) == 0 {
			return
		}
		columnWidths := calculateColumnWidths(rows)

		for _, row := range rows {
			for i, cell := range row {
				if needAlign {
					plain := regexpANSI.ReplaceAllString(cell, "")
					padding := columnWidths[i] - runewidth.StringWidth(plain)
					fmt.Fprint(w, cell)
					if padding > 0 {
						fmt.Fprint(w, strings.Repeat(" ", padding))
					}
					fmt.Fprint(w, "  ")
				} else {
					fmt.Fprint(w, cell)
				}
			}
			fmt.Fprintln(w)
		}
	}

	rows := make([][]string, 0)
	if hasHeader {
		rows = append(rows, headers)
	}
	for _, line := range lines {
		row := make([]string, len(jsonPaths))
		for i, jsonPath := range jsonPaths {
			values, err := jsonPath.FindResults(line)
			if err != nil {
				row[i] = "-"
				continue
			}
			valueStrings := []string{}
			if len(values) == 0 || len(values[0]) == 0 {
				row[i] = "-"
				continue
			}
			for arrIx := range values {
				for valIx := range values[arrIx] {
					v, ok := indirect(values[arrIx][valIx])
					if !ok {
						// An optional field the sender left out has
						// nothing to print, as an absent key had.
						continue
					}
					switch i := v.Interface().(type) {
					case time.Time:
						valueStrings = append(valueStrings, i.Format(time.RFC3339))
					default:
						valueStrings = append(valueStrings, fmt.Sprintf("%v", i))
					}
				}
			}
			if len(valueStrings) == 0 {
				row[i] = "-"
				continue
			}
			row[i] = strings.Join(valueStrings, ",")
		}
		rows = append(rows, row)
	}
	sprintAligned(rows)
	return w.String(), nil
}

// Print prints the representation of the data in one of the
// supported format (json, flat, human, ...).
//
// The human format needs a RenderFunc to be passed.
func (t Renderer) Print() {
	if s, err := t.Sprint(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		fmt.Print(s)
	}
}
