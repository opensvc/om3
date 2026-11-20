package commoncmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/unstructured"
)

type (
	// CmdDaemonEventsDoc documents the event kinds of the daemon.
	CmdDaemonEventsDoc struct {
		Color  string
		Output string
		Sort   string
		Kinds  []string
	}
)

// NewCmdDaemonEventsDoc returns the "daemon events doc" command.
func NewCmdDaemonEventsDoc() *cobra.Command {
	var options CmdDaemonEventsDoc
	cmd := &cobra.Command{
		Use:   "doc [NAME ...]",
		Short: "document the event kinds",
		Long: "Without a name, list the event kinds of the daemon, each with its summary. " +
			"With names, say for each kind when it is published and what it means, " +
			"and the keys of its data, as an event of the stream carries them.",
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Kinds = args
			return options.Run()
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return msgbus.Kinds(), cobra.ShellCompDirectiveNoFileComp
		},
	}
	CmdWithArg(cmd, `NAME  An event kind, as daemon events --filter takes it, as NodeNetworkStatusUpdated.`)
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	return cmd
}

func (t *CmdDaemonEventsDoc) Run() error {
	if len(t.Kinds) == 0 {
		return t.list()
	}
	docs := make([]msgbus.Doc, 0, len(t.Kinds))
	for _, kind := range t.Kinds {
		doc, err := msgbus.GetDoc(kind)
		if err != nil {
			return fmt.Errorf("%w: daemon events doc lists them", err)
		}
		docs = append(docs, doc)
	}
	return output.Renderer{
		HumanRenderer: func() string { return renderEventDocs(docs) },
		Output:        t.Output,
		Color:         t.Color,
		Data:          docs,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// list lists the event kinds, each with its summary.
func (t *CmdDaemonEventsDoc) list() error {
	l := make(unstructured.List, 0)
	for _, kind := range msgbus.Kinds() {
		doc, err := msgbus.GetDoc(kind)
		if err != nil {
			continue
		}
		l = append(l, map[string]any{
			"kind":    doc.Kind,
			"summary": doc.Summary(),
		})
	}
	return output.Renderer{
		DefaultOutput: "tab=KIND:kind,SUMMARY:summary",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          l,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// renderEventDocs renders the documentation of event kinds for a terminal:
// for each, its name, its text, and the keys of its data aligned.
func renderEventDocs(docs []msgbus.Doc) string {
	var sb strings.Builder
	for i, doc := range docs {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "%s\n\n", rawconfig.Colorize.Bold(doc.Kind))
		for _, line := range strings.Split(doc.Text, "\n") {
			if line == "" {
				sb.WriteString("\n")
				continue
			}
			for _, l := range wrapWords(line, eventDocWidth) {
				fmt.Fprintf(&sb, "  %s\n", l)
			}
		}
		if len(doc.Fields) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n  %s\n", rawconfig.Colorize.Secondary("data:"))
		width := 0
		for _, f := range doc.Fields {
			width = max(width, len(f.Name))
		}
		for _, f := range doc.Fields {
			fmt.Fprintf(&sb, "    %-*s  %s\n", width, f.Name, f.Type)
		}
	}
	return sb.String()
}

// eventDocWidth is the width the text of an event kind is wrapped at.
const eventDocWidth = 76

// wrapWords returns the lines of s, cut between words at width columns, a
// word longer than width on a line of its own.
func wrapWords(s string, width int) []string {
	var (
		lines []string
		line  string
	)
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) > width:
			lines = append(lines, line)
			line = word
		default:
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
