package arrayhp3par

import (
	"github.com/spf13/cobra"
)

func useFlagListVolumes() {
	cmd := cmdListVolumes
	cmd.Use = "list-volumes"
	cmd.Short = "List virtual volumes"
	cmd.Long = `List all virtual volumes on the HPE 3PAR array.`
}

func useFlagListSystems() {
	cmd := cmdListSystems
	cmd.Use = "list-systems"
	cmd.Short = "List storage systems"
	cmd.Long = `List storage system information for the HPE 3PAR array.`
}

func useFlagListNodes() {
	cmd := cmdListNodes
	cmd.Use = "list-nodes"
	cmd.Short = "List controller nodes"
	cmd.Long = `List all controller nodes on the HPE 3PAR array.`
}

func useFlagListCPGs() {
	cmd := cmdListCPGs
	cmd.Use = "list-cpgs"
	cmd.Short = "List Common Provisioning Groups"
	cmd.Long = `List all Common Provisioning Groups (CPGs) on the HPE 3PAR array.`
}

func useFlagListPorts() {
	cmd := cmdListPorts
	cmd.Use = "list-ports"
	cmd.Short = "List array ports"
	cmd.Long = `List all ports on the HPE 3PAR array.`
}

func useFlagShowVersion() {
	cmd := cmdShowVersion
	cmd.Use = "show-version"
	cmd.Short = "Show array version"
	cmd.Long = `Show the version of the HPE 3PAR array.`
}

func useFlagListRCGs() {
	cmd := cmdListRCGs
	cmd.Use = "list-rcgs"
	cmd.Short = "List Remote Copy Groups"
	cmd.Long = `List all Remote Copy Groups (RCGs) on the HPE 3PAR array.`
}

func useFlagShowRCG() {
	cmd := cmdShowRCG
	cmd.Use = "show-rcg"
	cmd.Short = "Show Remote Copy Group details"
	cmd.Long = `Show details of a specific Remote Copy Group (RCG) on the HPE 3PAR array.`
	cmd.Args = cobra.MinimumNArgs(1)
}
