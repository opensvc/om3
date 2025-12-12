package main

import (
	"os"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/util/capexec"
)

func main() {
	t := capexec.T{}
	flags := pflag.FlagSet{}
	t.FlagSet(&flags)
	flags.Parse(os.Args[2:])
	args := flags.Args()
	t.Exec(args)
}
