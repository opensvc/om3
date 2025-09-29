package tui

import (
	"strings"

	"github.com/rivo/tview"
	"golang.org/x/exp/maps"
)

var completionTree = map[string]any{
	"do": map[string]any{
		"cluster": []string{"freeze", "unfreeze"},
		"object": map[string]any{
			"abort":       nil,
			"freeze":      nil,
			"giveback":    nil,
			"provision":   nil,
			"purge":       nil,
			"restart":     nil,
			"start":       nil,
			"stop":        nil,
			"switch":      []string{"--live"},
			"unfreeze":    nil,
			"unprovision": nil,
		},
		"instance": map[string]any{
			"clear":       nil,
			"delete":      nil,
			"freeze":      nil,
			"provision":   nil,
			"refresh":     nil,
			"restart":     nil,
			"start":       nil,
			"stop":        nil,
			"switch":      []string{"--live"},
			"takeover":    []string{"--live"},
			"unfreeze":    nil,
			"unprovision": nil,
		},
		"resource": map[string]any{
			"disable":     nil,
			"enable":      nil,
			"provision":   nil,
			"run":         nil,
			"start":       []string{"--force"},
			"stop":        []string{"--force"},
			"unprovision": nil,
		},
		"service": []string{"restart", "start", "stop"},
		"task":    []string{"abort", "restart", "start", "stop"},
		"node":    []string{"drain", "freeze", "unfreeze"},
	},
	"filter":  nil,
	"connect": nil,
	"go":      []string{"sec", "cfg", "vol", "pool", "net", "relay"},
}

func (t *App) getActualSelected() string {
	row, col := t.objects.GetSelection()

	if t.focus() == viewInstance && row > 1 {
		if _, ok := t.flex.GetItem(2).(*tview.Table); ok {
			return "resource"
		}
		return ""
	}

	if row == 0 {
		if col == 1 {
			return "cluster"
		}
		if col >= t.firstInstanceCol {
			return "node"
		}
		return ""
	}

	if row >= t.firstObjectRow {
		if col == 0 {
			return "object"
		}
		if col >= t.firstInstanceCol {
			return "instance"
		}
	}

	return ""
}

func (t *App) getCompletions(text string) []string {
	args := strings.Fields(text)
	if len(args) == 0 {
		return maps.Keys(completionTree)
	}

	current := completionTree
	var prefix strings.Builder

	isMap := func(v any) bool {
		_, ok := v.(map[string]any)
		return ok
	}

	for i, arg := range args {
		if i > 0 {
			prefix.WriteString(" ")
		}

		next, exist := current[arg]
		if !exist {
			if isMap(current) {
				var results []string
				options := maps.Keys(current)
				for _, option := range options {
					if strings.HasPrefix(option, arg) {
						results = append(results, prefix.String()+option)
					}
				}
				return results
			}
			return []string{}
		}
		prefix.WriteString(arg)
		switch v := next.(type) {
		case map[string]any:
			if arg == "do" {
				selected := t.getActualSelected()
				if selected == "" {
					return []string{}
				}
				if _, ok := v[selected]; !ok {
					return []string{}
				}
				switch v[selected].(type) {
				case []string:
					return t.buildCompletions(v[selected].([]string), args, i, prefix.String())
				case map[string]any:
					current = v[selected].(map[string]any)
				default:
					return []string{}
				}
			}
		case []string:
			return t.buildCompletions(v, args, i, prefix.String())
		case nil:
			return []string{}
		}
	}
	if m, ok := any(current).(map[string]any); ok {
		keys := maps.Keys(m)
		res := make([]string, 0, len(keys))
		for _, key := range keys {
			res = append(res, prefix.String()+" "+key)
		}
		return res
	}
	return []string{}
}

func (t *App) buildCompletions(options, args []string, currentIndex int, prefix string) []string {
	if len(args) == currentIndex+2 {
		filter := args[currentIndex+1]
		var results []string
		for _, option := range options {
			if strings.HasPrefix(option, filter) {
				completion := prefix + " " + option
				results = append(results, completion)
			}
		}
		return results
	}

	results := make([]string, 0, len(options))
	for _, option := range options {
		results = append(results, prefix+" "+option)
	}
	return results
}
