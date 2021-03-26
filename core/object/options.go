package object

import (
	"reflect"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type (
	Opt struct {
		// Long is the long cobra flag handle
		Long string

		// Short is the short cobra flag handle
		Short string

		// Desc is the cobra flag description
		Desc string

		// Default is the default value to initialize the cobra flag with
		Default string
	}
)

func InstallFlags(cmd *cobra.Command, data interface{}) {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i += 1 {
		ft := t.Field(i)
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Struct:
			switch e := fv.Addr().Interface().(type) {
			default:
				InstallFlags(cmd, e)
			}
		default:
			InstallFlag(cmd, ft, fv)
		}
	}
}

func InstallFlag(cmd *cobra.Command, ft reflect.StructField, fv reflect.Value) {
	var (
		ok   bool
		flag string
		opt  Opt
	)
	if flag, ok = ft.Tag.Lookup("flag"); !ok {
		return
	}
	if opt, ok = FlagTag[flag]; !ok {
		log.Error().Msgf("%s has flag tag %s but no opt", fv, flag)
		return
	}
	//log.Info().Msgf("%s has flag tag %s and opt %s", fv, flag, opt)
	opt.InstallFlag(cmd, fv)
}

func (t *Opt) InstallFlag(cmd *cobra.Command, v reflect.Value) {
	flagSet := cmd.Flags()
	switch dest := v.Addr().Interface().(type) {
	case *int:
		var dft int
		if t.Default != "" {
			dft, _ = strconv.Atoi(t.Default)
		}
		flagSet.IntVarP(dest, t.Long, t.Short, dft, t.Desc)
	case *int64:
		var dft int64
		if t.Default != "" {
			dft, _ = strconv.ParseInt(t.Default, 10, 64)
		}
		flagSet.Int64VarP(dest, t.Long, t.Short, dft, t.Desc)
	case *bool:
		var dft bool
		if t.Default != "" {
			dft, _ = strconv.ParseBool(t.Default)
		}
		flagSet.BoolVarP(dest, t.Long, t.Short, dft, t.Desc)
	case *string:
		flagSet.StringVarP(dest, t.Long, t.Short, t.Default, t.Desc)
	case *[]string:
		dft := make([]string, 0)
		flagSet.StringSliceVarP(dest, t.Long, t.Short, dft, t.Desc)
	case *time.Duration:
		var dft time.Duration
		if t.Default != "" {
			dft, _ = time.ParseDuration(t.Default)
		}
		flagSet.DurationVarP(dest, t.Long, t.Short, dft, t.Desc)
	default:
		log.Error().Msgf("unknown flag type: %s", dest)
	}
}

var FlagTag = map[string]Opt{
	"color": Opt{
		Long:    "color",
		Default: "auto",
		Desc:    "output colorization yes|no|auto",
	},
	"format": Opt{
		Long:    "format",
		Default: "auto",
		Desc:    "output format json|flat|auto"},
	"objselector": Opt{
		Long:    "selector",
		Short:   "s",
		Default: "",
		Desc:    "an object selector expression, '**/s[12]+!*/vol/*'"},
	"nolock": Opt{
		Long: "nolock",
		Desc: "don't acquire the action lock (danger)",
	},
	"time": Opt{
		Long:    "time",
		Default: "5m",
		Desc:    "Stop waiting for the object to reach the target state after a duration",
	},
	"waitlock": Opt{
		Long:    "waitlock",
		Default: "30s",
		Desc:    "Lock acquire timeout",
	},
	"dry-run": Opt{
		Long: "dry-run",
		Desc: "show the action execution plan",
	},
	"local": Opt{
		Long: "local",
		Desc: "Inline action on local instance",
	},
	"force": Opt{
		Long: "force",
		Desc: "allow dangerous operations",
	},
	"eval": Opt{
		Long: "eval",
		Desc: "dereference and evaluate arythmetic expressions in value",
	},
	"impersonate": Opt{
		Long: "impersonate",
		Desc: "impersonate a peer node when evaluating keywords",
	},
	"kws": Opt{
		Long: "kw",
		Desc: "keyword operations, <k><op><v> with op in = |= += -= ^=",
	},
	"kw": Opt{
		Long: "kw",
		Desc: "a configuration keyword, [<section>].<option>",
	},
	"object": Opt{
		Long:  "service",
		Short: "s",
		Desc:  "execute on a list of objects",
	},
	"node": Opt{
		Long: "node",
		Desc: "execute on a list of nodes",
	},
	"wait": Opt{
		Long: "wait",
		Desc: "wait for the object to reach the target state",
	},
	"watch": Opt{
		Long:  "watch",
		Short: "w",
		Desc:  "watch the monitor changes",
	},
	"refresh": Opt{
		Long:  "refresh",
		Short: "r",
		Desc:  "refresh the status data",
	},
	"rid": Opt{
		Long: "rid",
		Desc: "resource selector expression (ip#1,app,disk.type=zvol)",
	},
	"subsets": Opt{
		Long: "subsets",
		Desc: "subset selector expression (g1,g2)",
	},
	"tags": Opt{
		Long: "tags",
		Desc: "tag selector expression (t1,t2)",
	},
	"server": Opt{
		Long: "server",
		Desc: "uri of the opensvc api server. scheme raw|https",
	},
}

type (
	OptsGlobal struct {
		Color          string `flag:"color"`
		Format         string `flag:"format"`
		Server         string `flag:"server"`
		Local          bool   `flag:"local"`
		NodeSelector   string `flag:"node"`
		ObjectSelector string `flag:"object"`
		DryRun         bool   `flag:"dry-run"`
	}

	OptsLocking struct {
		Disable bool          `flag:"nolock"`
		Timeout time.Duration `flag:"waitlock"`
	}

	OptsAsync struct {
		Watch bool          `flag:"watch"`
		Wait  bool          `flag:"wait"`
		Time  time.Duration `flag:"time"`
	}

	OptsResourceSelector struct {
		Id     string `flag:"rid"`
		Subset string `flag:"subsets"`
		Tag    string `flag:"tags"`
	}
)
