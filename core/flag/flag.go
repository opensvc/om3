package flag

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

func Install(cmd *cobra.Command, data interface{}) {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Struct:
			switch e := fv.Addr().Interface().(type) {
			default:
				Install(cmd, e)
			}
		default:
			installFlag(cmd, ft, fv)
		}
	}
}

func installFlag(cmd *cobra.Command, ft reflect.StructField, fv reflect.Value) {
	var (
		ok   bool
		flag string
		opt  Opt
	)
	if flag, ok = ft.Tag.Lookup("flag"); !ok {
		//log.Info().Msgf("%s %s has no flag tag", cmd.Use, ft.Name)
		return
	}
	if opt, ok = Tags[flag]; !ok {
		//log.Error().Msgf("%s has flag tag %s but no opt", ft.Name, flag)
		return
	}
	//log.Info().Msgf("%s %s has flag tag %s and opt %s", cmd.Use, ft.Name, flag, opt)
	opt.installFlag(cmd, fv)
}

func (t *Opt) installFlag(cmd *cobra.Command, v reflect.Value) {
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
