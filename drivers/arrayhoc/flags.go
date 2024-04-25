package arrayhoc

import "github.com/spf13/cobra"

var (
	filter            string
	volumeId          int
	size              string
	poolId            string
	resourceGroup     string
	name              string
	serial            string
	volumeIdRangeFrom int
	volumeIdRangeTo   int
	mappings          []string
	hostGroups        []string
	lun               int
	compression       bool
	deduplication     bool
)

func useFlagVolumeID(cmd *cobra.Command) {
	cmd.Flags().IntVar(&volumeId, "id", -1, "item id")
}

func useFlagFilter(cmd *cobra.Command) {
	cmd.Flags().StringVar(&filter, "filter", "", "filter the resources returned by a get query")
}

func useFlagSerial(cmd *cobra.Command) {
	cmd.Flags().StringVar(&serial, "serial", "", "volume serial")
}

func useFlagSize(cmd *cobra.Command) {
	cmd.Flags().StringVar(&size, "size", "", "disk size, expressed as a size expression like 1g, 100mib")
}

func useFlagPoolID(cmd *cobra.Command) {
	cmd.Flags().StringVar(&poolId, "pool-id", "", "pool id")
}

func useFlagResourceGroup(cmd *cobra.Command) {
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "resource group name")
}

func useFlagName(cmd *cobra.Command) {
	cmd.Flags().StringVar(&name, "name", "", "item name")
}

func useFlagVolumeIdRangeFrom(cmd *cobra.Command) {
	cmd.Flags().IntVar(&volumeIdRangeFrom, "from", -1, "volume id range begin")
}

func useFlagVolumeIdRangeTo(cmd *cobra.Command) {
	cmd.Flags().IntVar(&volumeIdRangeTo, "to", -1, "volume id range end")
}

func useFlagLUN(cmd *cobra.Command) {
	cmd.Flags().IntVar(&lun, "lun", -1, "logical unit number")
}

func useFlagMapping(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&mappings, "mapping", []string{}, "<hba_id>:<tgt_id>,<tgt_id>,... used in add map in replacement of --hostgroup. Can be specified multiple times")
}

func useFlagHostGroup(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&hostGroups, "hostGroup", []string{}, "can be specified multiple times")
}

func useFlagCompression(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&compression, "compression", false, "activate volume compression")
}

func useFlagDeduplication(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&deduplication, "deduplication", false, "activate volume data deduplication")
}
