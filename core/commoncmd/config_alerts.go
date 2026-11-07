package commoncmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

// ConfigAlertsOutput is how the alerts of the validation of an object
// configuration print by default.
const ConfigAlertsOutput = "tab=LEVEL:level,PATH:path,NODE:node,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment"

type (
	// DryRun has a configuration write run its checks and write nothing,
	// and says how to print the alerts the daemon answers it with.
	DryRun struct {
		Output string
		Color  string
	}
)

// NewDryRun returns the dry run a command asks with dryRun, printing as its
// output and color options say, and nil for a write.
func NewDryRun(dryRun bool, output, color string) *DryRun {
	if !dryRun {
		return nil
	}
	return &DryRun{Output: output, Color: color}
}

// Report prints the alerts the daemon answered a dry run of a configuration
// write of p with, and returns an error when one is an error, so the dry run
// exits non-zero for a write that would be refused.
//
// For a reader, the alerts print only when there are some, and the dry run
// says what the write would have done when none is an error. A machine
// output is the list, even empty.
func (t DryRun) Report(p naming.Path, alerts *api.ConfigAlertList, done string) error {
	items := api.ConfigAlertItems{}
	if alerts != nil && alerts.Items != nil {
		items = alerts.Items
	}
	human := output.New(t.Output) == output.Human
	if !human || len(items) > 0 {
		if err := PrintConfigAlerts(items, t.Output, "", t.Color); err != nil {
			return err
		}
	}
	if err := ConfigAlertErrors(items); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	if human {
		fmt.Printf("%s: would be %s\n", p, done)
	}
	return nil
}

// PrintConfigAlerts prints the alerts of the validation of a configuration,
// as om validate config does.
func PrintConfigAlerts(items api.ConfigAlertItems, out, sort, color string) error {
	return output.Renderer{
		DefaultOutput: ConfigAlertsOutput,
		Output:        out,
		Sort:          sort,
		Color:         color,
		Data:          api.ConfigAlertList{Kind: "ConfigAlertList", Items: items},
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// ConfigAlertErrors returns an error when an alert is an error, so the
// validation exits non-zero for a configuration that is not good.
func ConfigAlertErrors(items api.ConfigAlertItems) error {
	n := 0
	for _, e := range items {
		if e.Level == api.ConfigAlertLevelError {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("the configuration has %d error(s)", n)
	}
	return nil
}

// HasConfigAlertErrorsOf says whether an error alert is of kind.
func HasConfigAlertErrorsOf(items api.ConfigAlertItems, kind string) bool {
	for _, e := range items {
		if e.Level == api.ConfigAlertLevelError && e.Kind == kind {
			return true
		}
	}
	return false
}
