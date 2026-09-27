package commoncmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/daemon/api"
)

// CmdObjectCap sets the process group caps of objects and applies them to
// their running instances.
type CmdObjectCap struct {
	OptsGlobal
	OptsAsync
	Set []string
}

// NewCmdObjectCap returns the cap command of a kind.
//
// Its help lists the pg_* keywords of the store, with their example and the
// first sentence of their text, so it documents every cap the command sets,
// the ones added later included.
func NewCmdObjectCap(kind string) *cobra.Command {
	var t CmdObjectCap
	cmd := &cobra.Command{
		Use:   "cap",
		Short: "set the process group caps and apply them to the running instances",
		Long:  capLong(kind),
		RunE: func(cmd *cobra.Command, args []string) error {
			return t.Run(kind)
		},
	}
	CmdOrchestrated(cmd)
	flags := cmd.Flags()
	AddFlagsGlobal(flags, &t.OptsGlobal)
	FlagsAsync(flags, &t.OptsAsync)
	flags.StringArrayVar(&t.Set, "set", nil, "a cap to set, as [<section>.]pg_<name>[@<node>]=<value>")
	_ = cmd.RegisterFlagCompletionFunc("set", capSetCompletion(kind))
	return cmd
}

// Run writes the caps and queues the orchestration applying them.
func (t *CmdObjectCap) Run(kind string) error {
	mergedSelector := MergeSelector("", t.ObjectSelector, kind, "")
	params := api.PostObjectActionCapParams{}
	if len(t.Set) > 0 {
		set := append([]string{}, t.Set...)
		params.Set = &set
	}
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("capped"),
		objectaction.WithAsyncTargetOptions(params),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithSort(t.Sort),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
	).Do()
}

// pgKeywordsOf returns the pg_* keywords of a command kind, the service ones
// for a command of any kind.
func pgKeywordsOf(kind string) []*keywords.Keyword {
	k := naming.ParseKind(kind)
	if k != naming.KindSvc && k != naming.KindVol {
		k = naming.KindSvc
	}
	return object.PGKeywords(k)
}

func capLong(kind string) string {
	var b strings.Builder
	b.WriteString(`Set the process group caps of the object, of its subsets or of its resources,
and apply them to every running instance without a restart.

The caps are written as the pg_* keywords of the configuration, weighed by the
same rbac policy and claim checks as a configuration update. The orchestration
then applies the configuration written on every node running an instance. An
instance not running applies the caps when it starts.

A cap is set as [<section>.]pg_<name>[@<node>]=<value>. The section is the
object when not named, a subset#<name>, or a resource id. A value of "default"
lifts the cap: removing the keyword would leave the cap in place.

Without --set, the caps the configuration holds are applied again, which puts
back caps changed or lifted by hand.

  om test/svc/web cap --set pg_mem_limit=4g --set container#1.pg_cpu_quota=80%
  om test/svc/web cap --set subset#db.pg_mem_high=3g --wait
  om test/svc/web cap

The caps:

`)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, kw := range pgKeywordsOf(kind) {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", kw.Option, kw.Example, capSummary(kw.Text))
	}
	_ = w.Flush()
	fmt.Fprintf(&b, "\nThe full text of a cap: om <path> config doc --kw DEFAULT.pg_<name>\n")
	return b.String()
}

// capSummaryWidth is how long a cap summary may be, to hold on one line of a
// terminal after the name and the example.
const capSummaryWidth = 70

// capSummary is the first sentence of a keyword text, cut to one line, and
// said with the restriction on the hierarchy the text names, which a reader
// choosing a cap needs first.
func capSummary(text string) string {
	s := strings.Join(strings.Fields(text), " ")
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".")
	if len(s) > capSummaryWidth {
		// Cut at the last clause that fits, or at the last word.
		cut := s[:capSummaryWidth]
		if i := strings.LastIndexAny(cut, ",:;"); i > capSummaryWidth/2 {
			s = cut[:i]
		} else if i := strings.LastIndex(cut, " "); i > 0 {
			s = cut[:i] + "..."
		}
	}
	switch {
	case strings.Contains(text, "Only the unified cgroup hierarchy has it"):
		s += " (unified only)"
	case strings.Contains(text, "caps nothing on a node holding the unified"):
		s += " (v1 only)"
	}
	return s
}

// capSetCompletion completes a --set value: the caps, and the caps of the
// resources of the object named.
func capSetCompletion(kind string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names := make([]string, 0)
		for _, kw := range pgKeywordsOf(kind) {
			names = append(names, kw.Option+"=")
		}
		candidates := append([]string{}, names...)
		selector := getSelectorFromCommand(cmd)
		if selector == "" && len(args) > 0 {
			selector = args[0]
		}
		if isSimplePath(selector) {
			for _, rid := range getRIDsForSelector(selector) {
				for _, name := range names {
					candidates = append(candidates, rid+"."+name)
				}
			}
		}
		l := make([]string, 0)
		for _, c := range candidates {
			if strings.HasPrefix(c, toComplete) {
				l = append(l, c)
			}
		}
		return l, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
}
