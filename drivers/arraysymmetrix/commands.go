package arraysymmetrix

import (
	"github.com/opensvc/om3/core/array"
	"github.com/spf13/cobra"
)

func newParent() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "array",
		Short:         "Manage a symmetrix storage array",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return cmd
}

func newMapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "map",
		Short: "map commands",
	}
	return cmd
}
func newUnmapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unmap",
		Short: "unmap commands",
	}
	return cmd
}
func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add commands",
	}
	return cmd
}
func newDelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "del",
		Short: "del commands",
	}
	return cmd
}
func newRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename",
		Short: "rename commands",
	}
	return cmd
}
func newResizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resize",
		Short: "resize commands",
	}
	return cmd
}

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set commands",
	}
	return cmd
}

type OptResizeDisk struct {
	Dev   string
	SID   string
	Size  string
	Force bool
}

func newResizeDiskCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "resize a volume",
		RunE: func(_ *cobra.Command, _ []string) error {
			opt := OptResizeDisk{
				Dev:   dev,
				Size:  size,
				Force: force,
			}
			if data, err := t.ResizeDisk(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	useFlagSize(cmd)
	useFlagForce(cmd)
	return cmd
}

type OptUnmapDisk struct {
	Dev string
	SID string
}

func newUnmapDiskCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "unmap a volume",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptUnmapDisk{
				Dev: dev,
			}
			if data, err := t.UnmapDisk(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	return cmd
}

type OptRenameDisk struct {
	Dev  string
	Name string
	SID  string
}

func newRenameDiskCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "map a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptRenameDisk{
				Dev:  dev,
				Name: name,
			}
			if data, err := t.RenameDisk(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	useFlagName(cmd)
	return cmd
}

type OptMapDisk struct {
	Dev      string
	SID      string
	SLO      string
	SRP      string
	SG       string
	Mappings array.Mappings
}

func newMapDiskCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "map a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptMapDisk{
				Dev:      dev,
				Mappings: mappings,
				SLO:      slo,
				SRP:      srp,
				SG:       sg,
			}
			if data, err := t.MapDisk(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	useFlagMapping(cmd)
	useFlagSLO(cmd)
	useFlagSRP(cmd)
	useFlagSG(cmd)
	return cmd
}

type OptDelDisk struct {
	Dev string
	SID string
}

func newDelDiskCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "unmap a volume and delete",
		RunE: func(_ *cobra.Command, _ []string) error {
			opt := OptDelDisk{
				Dev: dev,
			}
			if data, err := t.DelDisk(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	return cmd
}

type OptAddThinDev struct {
	Name     string
	RDFG     string
	Size     string
	SG       string
	SLO      string
	SRDF     bool
	SRDFMode string
	SRDFType string
	SID      string
}

func newAddThinDevCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tdev",
		Short: "add a thin dev, no masking",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptAddThinDev{
				Name:     name,
				Size:     size,
				SRDF:     srdf,
				RDFG:     rdfg,
				SG:       sg,
				SRDFMode: srdfMode,
				SRDFType: srdfType,
			}
			if data, err := t.AddThinDev(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagName(cmd)
	useFlagSize(cmd)
	useFlagSRDF(cmd)
	useFlagSRDFMode(cmd)
	useFlagSRDFType(cmd)
	useFlagRDFG(cmd)
	useFlagSG(cmd)
	return cmd
}

type OptDelThinDev struct {
	Dev string
	SID string
}

func newDelThinDevCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tdev",
		Short: "delete a thin dev, no unmasking",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptDelThinDev{
				Dev: dev,
			}
			if data, err := t.DelThinDev(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	return cmd
}

type OptDeletePair struct {
	Dev string
	SID string
}

func newDeletePairCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deletepair",
		Short: "delete a SRDF pairing for the device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptDeletePair{
				Dev: dev,
			}
			if data, err := t.DeletePair(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagDev(cmd)
	return cmd
}

type OptCreatePair struct {
	Pair       string
	RDFG       string
	Invalidate string
	SID        string
	SRDFMode   string
	SRDFType   string
}

func newCreatePairCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "createpair",
		Short: "add a SRDF pairing for the device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptCreatePair{
				Pair:       pair,
				RDFG:       rdfg,
				Invalidate: invalidate,
				SRDFMode:   srdfMode,
				SRDFType:   srdfType,
			}
			return t.CreatePair(opt)
		},
	}
	useFlagPair(cmd)
	useFlagRDFG(cmd)
	useFlagInvalidate(cmd)
	useFlagSRDFMode(cmd)
	useFlagSRDFType(cmd)
	return cmd
}

type OptAddDisk struct {
	Name     string
	Size     string
	SID      string
	SG       string
	SLO      string
	SRP      string
	SRDF     bool
	SRDFMode string
	SRDFType string
	RDFG     string
	Mappings array.Mappings
}

func newAddDiskCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "add a volume and map",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt := OptAddDisk{
				Name:     name,
				Size:     size,
				SLO:      slo,
				SRP:      srp,
				SRDF:     srdf,
				SRDFMode: srdfMode,
				SRDFType: srdfType,
				RDFG:     rdfg,
				Mappings: mappings,
			}
			if data, err := t.AddDisk(opt); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagName(cmd)
	useFlagSize(cmd)
	useFlagMapping(cmd)
	useFlagSLO(cmd)
	useFlagSG(cmd)
	useFlagSRP(cmd)
	useFlagSRDF(cmd)
	useFlagSRDFMode(cmd)
	useFlagSRDFType(cmd)
	useFlagRDFG(cmd)
	return cmd
}

type OptFreeThinDev struct {
	SID string
	Dev string
}

func newFreeThinDev(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "free",
		Short: "free thin device",
		RunE: func(_ *cobra.Command, _ []string) error {
			opt := OptFreeThinDev{
				Dev: dev,
			}
			return t.FreeThinDev(opt)
		},
	}
	useFlagDev(cmd)
	return cmd
}

type OptSetSRDFMode struct {
	SRDFMode string
	Dev      string
	SID      string
}

func newSetSRDFModeCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode",
		Short: "set SRDF mode",
		RunE: func(_ *cobra.Command, _ []string) error {
			opt := OptSetSRDFMode{
				Dev:      dev,
				SRDFMode: srdfMode,
			}
			return t.SetSRDFMode(opt)
		},
	}
	useFlagDev(cmd)
	useFlagSRDFMode(cmd)
	return cmd
}

func newAddMaskingCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "masking",
		Short: "present disks to hosts in batch mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if data, err := t.AddMasking([]byte(data)); err != nil {
				return err
			} else {
				return dump(data)
			}
		},
	}
	useFlagData(cmd)
	return cmd
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get commands",
	}
	return cmd
}
func newGetPoolsCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pools",
		Short: "get thin pools",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := t.SymCfgPoolList()
			if err != nil {
				return err
			}
			return dump(data)
		},
	}
	return cmd
}
func newGetStorageGroupsCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sgs",
		Short: "get storage groups",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := t.SymSGList("")
			if err != nil {
				return err
			}
			return dump(data)
		},
	}
	return cmd
}
func newGetSRPsCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "srps",
		Short: "get SRP names",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := t.SymCfgSRPList()
			if err != nil {
				return err
			}
			return dump(data)
		},
	}
	return cmd
}
func newGetDirectorsCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "directors",
		Short: "get directors",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := t.SymCfgDirectorList("all")
			if err != nil {
				return err
			}
			return dump(data)
		},
	}
	return cmd
}
func newGetThinDevsCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tdevs",
		Short: "get thin devs",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := t.SymDevList("")
			if err != nil {
				return err
			}
			return dump(data)
		},
	}
	return cmd
}
func newGetViewsCmd(t *Array) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "views",
		Short: "get masking views",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := t.SymAccessListViewDetail()
			if err != nil {
				return err
			}
			return dump(data)
		},
	}
	return cmd
}
