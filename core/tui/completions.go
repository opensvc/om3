package tui

import (
	"strings"

	"github.com/rivo/tview"
	"golang.org/x/exp/maps"
)

type node map[string]node

var (
	nodeDoCluster = node{
		"freeze":   nil,
		"unfreeze": nil,
	}
	nodeDoObject = node{
		"abort":     nil,
		"freeze":    nil,
		"giveback":  nil,
		"provision": nil,
		"purge":     nil,
		"restart":   nil,
		"start":     nil,
		"stop":      nil,
		"switch": node{
			"--live": nil,
		},
		"unfreeze":    nil,
		"unprovision": nil,
	}
	nodeDoInstance = node{
		"clear":  nil,
		"delete": nil,
		"freeze": nil,
		"provision": node{
			"--disable-rollback": nil,
			"--leader":           nil,
		},
		"refresh": nil,
		"restart": nil,
		"start":   nil,
		"stop": node{
			"--force": nil,
		},
		"switch": node{
			"--live": nil,
		},
		"takeover": node{
			"--live": nil,
		},
		"unfreeze":    nil,
		"unprovision": nil,
	}
	nodeDoResource = node{
		"disable":   nil,
		"enable":    nil,
		"provision": nil,
		"run":       nil,
		"start": node{
			"--force": nil,
		},
		"stop": node{
			"--force": nil,
		},
		"unprovision": nil,
	}
	nodeDoTask = node{
		"abort": nil,
		"run":   nil,
	}
	nodeDoNode = node{
		"drain":    nil,
		"freeze":   nil,
		"unfreeze": nil,
	}
	nodeRoot = node{
		"do":      nil,
		"filter":  nil,
		"connect": nil,
		"go": node{
			"sec":   nil,
			"cfg":   nil,
			"vol":   nil,
			"pool":  nil,
			"net":   nil,
			"relay": nil,
		},
	}
)

func (t node) Candidates(prefix, arg string) []string {
	if t == nil {
		return []string{}
	}
	var candidates []string
	for _, candidate := range maps.Keys(t) {
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
			return nodeDoResource
		}
		return nil
	}

	if row == 0 {
		if col == 1 {
			return nodeDoCluster
		}
		if col >= t.firstInstanceCol {
			return nodeDoNode
		}
		return nil
	}

	if row >= t.firstObjectRow {
		if col == 0 {
			return nodeDoObject
		}
		if col >= t.firstInstanceCol {
			return nodeDoInstance
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

	for i, arg := range args {
		next, ok := current[arg]
		if !ok {
			return current.Candidates(prefix.String(), arg)
		}
		prefix.WriteString(arg)
		prefix.WriteString(" ")
		if i == n-1 {
			if !strings.HasSuffix(text, " ") {
				return []string{}
			}
			return next.Candidates(prefix.String(), "")
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
