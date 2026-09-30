package tui

import (
	"strings"

	"github.com/rivo/tview"
	"golang.org/x/exp/maps"
)

type node map[string]node

var (
	nodeRoot = node{
		"do":      nil,
		"connect": nil,
		"go": node{
			"sec":   nil,
			"cfg":   nil,
			"usr":   nil,
			"svc":   nil,
			"vol":   nil,
			"pool":  nil,
			"net":   nil,
			"relay": nil,
		},
	}
)

func (t node) Candidates(prefix, arg string, flags map[string]bool) []string {
	//prefix, _, _ = strings.Cut(prefix, " --")
	if t == nil {
		return []string{}
	}
	var candidates []string
	for _, candidate := range maps.Keys(t) {
		if _, ok := flags[candidate]; ok {
			continue
		}
		if arg == "" || strings.HasPrefix(candidate, arg) {
			candidates = append(candidates, prefix+candidate)
		}
	}
	return candidates
}

func (t *App) getDo() node {
	row, col := t.objects.GetSelection()

	if t.focus() == viewInstance && row > 1 {
		if _, ok := t.flex.GetItem(2).(*tview.Table); ok {
			return doResource.node()
		}
		return nil
	}

	if row == 0 {
		if col == 1 {
			return doCluster.node()
		}
		if col >= t.firstInstanceCol {
			return doNode.node()
		}
		return nil
	}

	if row >= t.firstObjectRow {
		if col == 0 {
			return doObject.node()
		}
		if col >= t.firstInstanceCol {
			return doInstance.node()
		}
	}

	return nil
}

func (t *App) getCompletions(text string) []string {
	args := strings.Fields(text)

	current := nodeRoot
	current["do"] = t.getDo()

	var prefix strings.Builder

	n := len(args)
	flags := make(map[string]bool)

	if n == 0 {
		return current.Candidates(prefix.String(), "", flags)
	}

	for i, arg := range args {
		next, ok := current[arg]
		isFlag := strings.HasPrefix(arg, "--")
		if isFlag {
			next = current
		}
		if !ok {
			return current.Candidates(prefix.String(), arg, flags)
		}
		if isFlag {
			if _, ok := flags[arg]; !ok {
				prefix.WriteString(arg)
				prefix.WriteString(" ")
				flags[arg] = true
			}
		} else {
			prefix.WriteString(arg)
			prefix.WriteString(" ")
		}
		if i == n-1 {
			if !strings.HasSuffix(text, " ") {
				return []string{}
			}
			return next.Candidates(prefix.String(), "", flags)
		}
		current = next
	}
	return []string{}
}

func (t *App) buildCompletions(options, args []string, currentIndex int, prefix string) []string {
	results := make([]string, len(options))
	for i, option := range options {
		results[i] = prefix + " " + option
	}
	return results
}
