package tui

import (
	"strings"

	"github.com/rivo/tview"
	"golang.org/x/exp/maps"
)

var completionTree = map[string]any{
	"do": map[string]any{
		"cluster":  []string{"freeze", "unfreeze"},
		"object":   []string{"abort", "delete", "freeze", "giveback", "provision", "purge", "restart", "start", "stop", "switch", "unfreeze", "unprovision"},
		"instance": []string{"clear", "delete", "freeze", "provision", "refresh", "start", "stop", "switch", "takeover", "unfreeze", "unprovision"},
		"resource": []string{"disable", "enable", "provision", "run", "start", "stop", "unprovision"},
		"node":     []string{"drain", "freeze", "unfreeze"},
	},
	"filter":  nil,
	"connect": nil,
	"go":      []string{"sec", "cfg", "vol", "pool", "net"},
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

	for i, arg := range args {
		if i > 0 {
			prefix.WriteString(" ")
		}
		prefix.WriteString(arg)

		next, exist := current[arg]
		if !exist {
			if i == 0 {
				var results []string
				options := maps.Keys(completionTree)
				for _, option := range options {
					if strings.HasPrefix(option, arg) {
						results = append(results, option)
					}
				}
				return results
			}
			return []string{}
		}
		switch v := next.(type) {
		case map[string]any:
			if arg == "do" {
				selected := t.getActualSelected()
				if selected == "" {
					return []string{}
				}
				actions, ok := v[selected].([]string)
				if !ok {
					return []string{}
				}
				return t.buildCompletions(actions, args, i, prefix.String())
			}
		case []string:
			if arg == "go" {
				return t.buildCompletions(v, args, i, prefix.String())
			}
		case nil:
			return []string{}
		}
	}
	if m, ok := any(current).(map[string]any); ok {
		return maps.Keys(m)
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
