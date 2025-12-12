package main

import (
	"os"

	"github.com/opensvc/om3/v3/util/capexec"
	"github.com/spf13/pflag"
)

func main() {
	t := capexec.T{}
	flags := pflag.FlagSet{}
	t.FlagSet(&flags)
	flags.Parse(os.Args[2:])
	args := flags.Args()
	t.Exec(args)
}
