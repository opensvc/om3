package arraypure

import "github.com/spf13/cobra"

var (
	blocksize   string
	filter      string
	host        string
	hostGroup   string
	id          int
	initiators  []string
	lun         int
	mappings    []string
	naa         string
	name        string
	now         bool
	pod         string
	serial      string
	size        string
	targets     []string
	truncate    bool
	volumeGroup string
	wwn         string
)

func useFlagBlocksize(cmd *cobra.Command) {
	cmd.Flags().StringVar(&blocksize, "blocksize", "", "disk blocksize in B")
}

func useFlagFilter(cmd *cobra.Command) {
	cmd.Flags().StringVar(&filter, "filter", "", "items filtering expression. ex: id='1' and serial='abc' and pod.name='pod1' and destroyed='false'.")
}

func useFlagHost(cmd *cobra.Command) {
	cmd.Flags().StringVar(&host, "host", "", "initiator host name")
}

func useFlagHostGroup(cmd *cobra.Command) {
	cmd.Flags().StringVar(&hostGroup, "hostgroup", "", "host group name")
}

func useFlagID(cmd *cobra.Command) {
	cmd.Flags().IntVar(&id, "id", -1, "item id")
}

func useFlagInitiator(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&initiators, "initiator", []string{}, "initiator hba ids")
}

func useFlagLUN(cmd *cobra.Command) {
	cmd.Flags().IntVar(&lun, "lun", -1, "logical unit number")
}

func useFlagMapping(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&mappings, "mapping", []string{}, "<hba_id>:<tgt_id>,<tgt_id>,... used in add map in replacement of --host and --hostgroup. Can be specified multiple times")
}

func useFlagName(cmd *cobra.Command) {
	cmd.Flags().StringVar(&name, "name", "", "item name")
}

func useFlagNAA(cmd *cobra.Command) {
	cmd.Flags().StringVar(&naa, "naa", "", "volume naa identifier")
}

func useFlagNow(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&now, "now", false, "delete item after flagging it destroyed (DANGER)")
}

func useFlagPod(cmd *cobra.Command) {
	cmd.Flags().StringVar(&pod, "pod", "", "pod name")
}

func useFlagSerial(cmd *cobra.Command) {
	cmd.Flags().StringVar(&serial, "serial", "", "item serial")
}

func useFlagSize(cmd *cobra.Command) {
	cmd.Flags().StringVar(&size, "size", "", "disk size, expressed as a size expression like 1g, 100mib")
}

func useFlagTarget(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&targets, "target", []string{}, "targets to export the disk through")
}

func useFlagTruncate(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&truncate, "truncate", false, "allow truncating a resized volume (DANGER")
}

func useFlagVolumeGroup(cmd *cobra.Command) {
	cmd.Flags().StringVar(&volumeGroup, "volumegroup", "", "volume group name")
}

func useFlagWWN(cmd *cobra.Command) {
	cmd.Flags().StringVar(&wwn, "wwn", "", "world wide number identifier")
}
