package arrayhp3par

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/drivers/shared/hp3par"
)

// csvArgs asks a command for comma separated values, with no header and no
// totals line, so what the array prints is what the parser reads.
//
// v2 sets these once for a whole ssh session instead. They are asked for per
// command here because the session the tested command shapes open runs one
// command and closes.
const csvArgs = " -csvtable -nohdtot"

// The columns each cli command is asked for, and the order they come back in.
// They are v2's, because the collector reads what it is handed by name and
// stores it in columns of its own.
var (
	colsShowVV = []string{
		"Name", "VV_WWN", "Prov", "CopyOf", "Tot_Rsvd_MB", "VSize_MB", "UsrCPG", "CreationTime",
	}
	colsShowVVRemoteCopy = append(append([]string{}, colsShowVV...), "RcopyGroup", "RcopyStatus")
	colsShowSys          = []string{
		"ID", "Name", "Model", "Serial", "Nodes", "Master", "TotalCap", "AllocCap", "FreeCap", "FailedCap",
	}
	colsShowNode = []string{
		"Available_Cache", "Control_Mem", "Data_Mem", "InCluster", "LED", "Master", "Name", "Node", "State",
	}
	colsShowCPG = []string{
		"Id", "Name", "Warn%", "VVs", "TPVVs", "Usr", "Snp", "Total", "Used",
	}
	colsShowPort = []string{
		"N:S:P", "Mode", "State", "Node_WWN", "Port_WWN", "Type", "Protocol", "Label", "Partner", "FailoverState",
	}
)

// ShowVV returns the virtual volumes of the array.
//
// The remote copy columns are asked for only when the array is licensed for
// it: a column the array does not know makes the whole command fail.
func (t *Array) ShowVV(ctx context.Context) (any, error) {
	cols := colsShowVV
	if ok, err := t.hasRemoteCopy(ctx); err != nil {
		return nil, err
	} else if ok {
		cols = colsShowVVRemoteCopy
	}
	out, err := t.run(ctx, "showvv -showcols "+strings.Join(cols, ",")+csvArgs)
	if err != nil {
		return nil, err
	}
	return hp3par.ParseCSV(out, cols), nil
}

// ShowSys returns what the array says about itself.
func (t *Array) ShowSys(ctx context.Context) (any, error) {
	out, err := t.run(ctx, "showsys"+csvArgs)
	if err != nil {
		return nil, err
	}
	return hp3par.ParseCSV(out, colsShowSys), nil
}

// ShowNode returns the controller nodes of the array.
func (t *Array) ShowNode(ctx context.Context) (any, error) {
	out, err := t.run(ctx, "shownode -showcols "+strings.Join(colsShowNode, ",")+csvArgs)
	if err != nil {
		return nil, err
	}
	return hp3par.ParseCSV(out, colsShowNode), nil
}

// ShowCPG returns the common provisioning groups of the array.
func (t *Array) ShowCPG(ctx context.Context) (any, error) {
	out, err := t.run(ctx, "showcpg"+csvArgs)
	if err != nil {
		return nil, err
	}
	return hp3par.ParseCSV(out, colsShowCPG), nil
}

// ShowPort returns the ports of the array.
func (t *Array) ShowPort(ctx context.Context) (any, error) {
	out, err := t.run(ctx, "showport"+csvArgs)
	if err != nil {
		return nil, err
	}
	return hp3par.ParseCSV(out, colsShowPort), nil
}

// ShowVersion returns the version the array runs.
func (t *Array) ShowVersion(ctx context.Context) (any, error) {
	out, err := t.run(ctx, "showversion -s")
	if err != nil {
		return nil, err
	}
	return map[string]string{"Version": strings.TrimSpace(out)}, nil
}

// hasRemoteCopy reports whether the array is licensed for remote copy.
func (t *Array) hasRemoteCopy(ctx context.Context) (bool, error) {
	out, err := t.run(ctx, "showlicense")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "Remote Copy"), nil
}

// Actions returns what this array answers to.
//
// v2 declares no action for a 3PAR: it reads the array to report it to the
// collector and no further. The same readings are commands here, so what is
// pushed can be looked at without pushing it.
func (t *Array) Actions() []array.Action {
	show := func(word, short string, fn func(context.Context) (any, error)) array.Action {
		return array.Action{
			Path:  []string{"get", word},
			Short: short,
			Run: func(ctx context.Context, _ array.Input) (any, error) {
				return fn(ctx)
			},
		}
	}
	return []array.Action{
		show("volumes", "get virtual volumes", t.ShowVV),
		show("system", "get the array itself", t.ShowSys),
		show("nodes", "get controller nodes", t.ShowNode),
		show("cpgs", "get common provisioning groups", t.ShowCPG),
		show("ports", "get ports", t.ShowPort),
		show("version", "get the version the array runs", t.ShowVersion),
	}
}

// Reports returns the sections of its configuration this array pushes to the
// collector.
//
// The section names are the ones v2 pushes for a 3par array, because the
// collector reads them to know what it was handed.
func (t *Array) Reports() []array.Report {
	return []array.Report{
		{Key: "showvv", Get: t.ShowVV},
		{Key: "showsys", Get: t.ShowSys},
		{Key: "shownode", Get: t.ShowNode},
		{Key: "showcpg", Get: t.ShowCPG},
		{Key: "showport", Get: t.ShowPort},
		{Key: "showversion", Get: t.ShowVersion},
	}
}
