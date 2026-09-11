package output

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"k8s.io/client-go/util/jsonpath"
)

// sortKey is one term of a sort expression: what to order on, and which way.
type sortKey struct {
	path *jsonpath.JSONPath
	expr string
	desc bool

	// self is the term ".", which names the item itself rather than a field
	// of it, for a listing whose items are bare values.
	self bool
}

// parseSort reads a comma separated sort expression, each term naming a field
// the way a tab column names one, optionally prefixed with "-" to reverse it.
//
//	--sort=-started_at,path
//
// The "-" has to be written with "--sort=" rather than "--sort ", or the
// shell's flag parser reads it as the next option.
func parseSort(s string) ([]sortKey, error) {
	keys := make([]sortKey, 0)
	for _, term := range strings.Split(s, ",") {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		key := sortKey{}
		if strings.HasPrefix(term, "-") {
			key.desc = true
			term = term[1:]
		} else if strings.HasPrefix(term, "+") {
			term = term[1:]
		}
		if term == "" {
			continue
		}
		key.expr = term
		if term == "." {
			key.self = true
			keys = append(keys, key)
			continue
		}
		expr, err := RelaxedJSONPathExpression(term)
		if err != nil {
			return nil, fmt.Errorf("sort %s: %w", term, err)
		}
		p := jsonpath.New(term)
		if err := p.Parse(expr); err != nil {
			return nil, fmt.Errorf("sort %s: %w", term, err)
		}
		key.path = p
		keys = append(keys, key)
	}
	return keys, nil
}

// sortValue is one field of one item, reduced to something comparable.
//
// A field the item does not carry is absent rather than empty, and absent
// sorts last whichever way the order runs: a listing puts what it knows first,
// and reversing the order is not a reason to lead with what it does not.
type sortValue struct {
	absent bool
	number float64
	text   string
	isNum  bool
}

func (a sortValue) compare(b sortValue) int {
	if a.isNum && b.isNum {
		switch {
		case a.number < b.number:
			return -1
		case a.number > b.number:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(a.text, b.text)
}

// valueOf reduces what a jsonpath found on an item to something comparable.
//
// An instant is compared as an instant and not as the string it is rendered
// as: two nodes of one cluster can report the same moment with different utc
// offsets, and the text of those does not order the way the moments do.
func valueOf(key sortKey, item any) (sortValue, bool) {
	var v reflect.Value
	if key.self {
		v = reflect.ValueOf(item)
	} else {
		results, err := key.path.FindResults(item)
		if err != nil {
			// The item has no such field. Told apart from a field it has and
			// has nothing in, which is absent rather than unknown.
			return sortValue{absent: true}, false
		}
		if len(results) == 0 || len(results[0]) == 0 {
			return sortValue{absent: true}, true
		}
		v = results[0][0]
	}
	iv, ok := indirect(v)
	if !ok {
		return sortValue{absent: true}, true
	}
	v = iv
	switch i := v.Interface().(type) {
	case time.Time:
		return sortValue{isNum: true, number: float64(i.UnixNano())}, true
	case time.Duration:
		return sortValue{isNum: true, number: float64(i)}, true
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return sortValue{isNum: true, number: float64(v.Int())}, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return sortValue{isNum: true, number: float64(v.Uint())}, true
	case reflect.Float32, reflect.Float64:
		return sortValue{isNum: true, number: v.Float()}, true
	case reflect.Bool:
		n := 0.0
		if v.Bool() {
			n = 1
		}
		return sortValue{isNum: true, number: n}, true
	}
	return sortValue{text: fmt.Sprintf("%v", v.Interface())}, true
}

// sortData orders the items of a listing in place.
//
// It is done here rather than by whoever assembled the data, and rather than
// by the daemon that served it, because a listing is often the union of what
// several nodes answered: each of them can only order its own share, and the
// order of the whole is the client's to decide once it holds the whole.
//
// It is done before the format is chosen, so that a machine reading the json
// and a person reading the table are given the same order.
func sortData(data any, s string) error {
	if s == "" || data == nil {
		return nil
	}
	keys, err := parseSort(s)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		// One item is already in order.
		return nil
	}
	n := v.Len()
	if n < 2 {
		return nil
	}

	// The values are read once per item rather than on every comparison: a
	// jsonpath lookup is reflection, and a sort asks O(n log n) times.
	values := make([][]sortValue, n)
	known := make([]bool, len(keys))
	for i := 0; i < n; i++ {
		item := row(v.Index(i).Interface())
		values[i] = make([]sortValue, len(keys))
		for j, key := range keys {
			value, ok := valueOf(key, item)
			values[i][j] = value
			known[j] = known[j] || ok
		}
	}
	for j, ok := range known {
		if !ok {
			// Not one item of the listing has this field. Ordering on it
			// would do nothing at all, and a misspelled field that quietly
			// does nothing is worse than one that says so.
			return fmt.Errorf("sort %s: the listing has no such field", keys[j].expr)
		}
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		for j, key := range keys {
			va, vb := values[order[a]][j], values[order[b]][j]
			if va.absent != vb.absent {
				// Decided before the direction is applied, so that reversing
				// the order does not bring what the listing does not know to
				// the top.
				return vb.absent
			}
			c := va.compare(vb)
			if c == 0 {
				continue
			}
			if key.desc {
				return c > 0
			}
			return c < 0
		}
		return false
	})

	sorted := reflect.MakeSlice(reflect.SliceOf(v.Type().Elem()), n, n)
	for i, j := range order {
		sorted.Index(i).Set(v.Index(j))
	}
	reflect.Copy(v, sorted)
	return nil
}
