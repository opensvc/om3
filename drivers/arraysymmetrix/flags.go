package arraysymmetrix

import (
	"github.com/opensvc/om3/v3/core/array"
	"github.com/spf13/cobra"
)

var (
	name       string
	dev        string
	data       string
	force      bool
	pair       string
	mappings   = make(array.Mappings)
	size       string
	slo        string
	srp        string
	srdf       bool
	rdfg       string
	sg         string
	invalidate string
	srdfType   string
	srdfMode   string
)

func useFlagName(cmd *cobra.Command) {
	cmd.Flags().StringVar(&name, "name", "", "item name")
}

func useFlagDev(cmd *cobra.Command) {
	cmd.Flags().StringVar(&dev, "dev", "", "the device id (ex: 00A04)")
}

func useFlagData(cmd *cobra.Command) {
	cmd.Flags().StringVar(&data, "data", "", "the workplan provided in json format")
}

func useFlagForce(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&force, "force", false, "bypass the downsize sanity check.")
}

func useFlagPair(cmd *cobra.Command) {
	cmd.Flags().StringVar(&pair, "pair", "", "the device id pair (ex: 00A04:00A04)")
}

func useFlagMapping(cmd *cobra.Command) {
	cmd.Flags().Var(&mappings, "mapping", "<hba_id>:<tgt_id>,<tgt_id>,... used in add map in replacement of --hostgroup. Can be specified multiple times")
}

func useFlagSize(cmd *cobra.Command) {
	cmd.Flags().StringVar(&size, "size", "", "disk size, expressed as a size expression like 1g, 100mib")
}

func useFlagSLO(cmd *cobra.Command) {
	cmd.Flags().StringVar(&slo, "slo", "", "the thin device Service Level Objective")
}

func useFlagSRP(cmd *cobra.Command) {
	cmd.Flags().StringVar(&srp, "srp", "", "the Storage Resource Pool hosting the device")
}

func useFlagSRDF(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&srdf, "srdf", false, "create a SRDF mirrored device pair with --array hosting the R1 member")
}

func useFlagSRDFType(cmd *cobra.Command) {
	cmd.Flags().StringVar(&srp, "srdf-type", "R1", "the device role in the SRDF mirror (ex: R1)")
}

func useFlagSRDFMode(cmd *cobra.Command) {
	cmd.Flags().StringVar(&srp, "srdf-mode", "sync", "device mirroring mode. either sync, acp_wp or acp_disk")
}

func useFlagRDFG(cmd *cobra.Command) {
	cmd.Flags().StringVar(&rdfg, "rdfg", "", "the RDF / RA Group number, required if --srdf is set")
}

func useFlagSG(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sg, "sg", "", "as an alternative to --mappings, specify the storage group to put the dev into.")
}

func useFlagInvalidate(cmd *cobra.Command) {
	cmd.Flags().StringVar(&invalidate, "invalidate", "", "the SRDF mirror member to invalidate upon createpair (ex: R2). don't set to just establish")
}
