package arrayfreenas

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/sizeconv"
)

var (
	Head                  = "/api/v2.0"
	DatasetTypeVolume     = "VOLUME"
	DatasetTypeFilesystem = "FILESYSTEM"
	RequestTimeout        = 10 * time.Second
)

type (
	Array struct {
		*array.Array
	}

	UnmapDiskOptions struct {
		Name    string
		Mapping string
	}

	MapDiskOptions struct {
		Name    string
		Mapping string
		LunId   *int
	}

	DelISCSIExtentOptions struct {
		Id   int
		Name string
	}

	AddDiskOptions struct {
		AddZvolOptions
		InsecureTPC bool
		Mapping     string
		LunId       *int
	}

	// AddZvolOptions receives "add zvol" and "add disk" command line flags values
	AddZvolOptions struct {
		Name          string
		Size          string
		Blocksize     string
		Sparse        bool
		Deduplication string
		Compression   string
	}

	AddISCSIExtentOptions struct {
		Name        string
		Disk        string
		Blocksize   string
		InsecureTPC bool
	}

	AddISCSITargetGroupOptions struct {
		PortalId      int
		Target        string
		InitiatorName string
		InitiatorId   int
		AuthMethod    string
		Auth          string
	}

	Disk struct {
		Dataset *Dataset   `json:"dataset"`
		ISCSI   *DiskISCSI `json:"iscsi,omitempty"`
	}
	DiskISCSI struct {
		Extent        *ISCSIExtent        `json:"extent,omitempty"`
		TargetExtents []ISCSITargetExtent `json:"targetextents,omitempty"`
	}

	// CompositeValue defines model for CompositeValue.
	CompositeValue struct {
		Rawvalue string  `json:"rawvalue"`
		Source   *string `json:"source,omitempty"`
		Value    *string `json:"value,omitempty"`
	}
)

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "truenas"), NewDriver)
	driver.Register(driver.NewID(driver.GroupArray, "freenas"), NewDriver) // backward compat
}

func NewDriver() array.Driver {
	t := New()
	var i any = t
	return i.(array.Driver)
}

func New() *Array {
	t := &Array{
		Array: array.New(),
	}
	return t
}

func (t *Array) Run(args []string) error {
	var (
		auth          string
		authGroupId   int
		authMethod    string
		authNetworks  []string
		id            int
		blocksize     string
		compression   string
		comment       string
		dedup         string
		disk          string
		initiatorName string
		initiatorId   int
		initiators    []string
		insecureTPC   bool
		listen        []string
		lunId         int
		mapping       string
		name          string
		portalId      int
		size          string
		sparse        bool
		target        string
		volume        string
	)
	newParent := func() *cobra.Command {
		cmd := &cobra.Command{
			SilenceErrors: true,
			SilenceUsage:  true,
			Use:           "array",
			Short:         "Manage a truenas storage array",
		}
		return cmd
	}

	newMapCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "map",
			Short: "map commands",
		}
		return cmd
	}
	newMapISCSICmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "iscsi",
			Short: "map iscsi commands",
		}
		return cmd
	}
	newUnmapCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "unmap",
			Short: "unmap commands",
		}
		return cmd
	}
	newUnmapISCSICmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "iscsi",
			Short: "unmap iscsi commands",
		}
		return cmd
	}
	newAddCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "add",
			Short: "add commands",
		}
		return cmd
	}
	newDelCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "del",
			Short: "del commands",
		}
		return cmd
	}

	newUnmapISCSIZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "zvol",
			Short: "unmap a zvol-type dataset",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := UnmapDiskOptions{
					Name:    name,
					Mapping: mapping,
				}
				if data, err := t.UnmapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&mapping, "mappings", "", "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.PersistentFlags().MarkHidden("mappings")
		return cmd
	}
	newMapISCSIZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:    "zvol",
			Hidden: true,
			Short:  "map a zvol-type dataset",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := MapDiskOptions{
					Name:    name,
					Mapping: mapping,
				}
				if lunId >= 0 {
					opt.LunId = &lunId
				}
				if data, err := t.MapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&mapping, "mappings", "", "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.Flags().IntVar(&lunId, "lun", -1, "")
		cmd.PersistentFlags().MarkHidden("mappings")
		return cmd
	}
	newMapDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "map a zvol-type dataset",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := MapDiskOptions{
					Name:    name,
					Mapping: mapping,
				}
				if lunId >= 0 {
					opt.LunId = &lunId
				}
				if data, err := t.MapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.Flags().StringVar(&mapping, "mappings", "", "")
		cmd.Flags().IntVar(&lunId, "lun", -1, "")
		cmd.PersistentFlags().MarkHidden("mappings")
		return cmd
	}
	newDelDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "unmap a zvol-type dataset and delete",
			RunE: func(_ *cobra.Command, _ []string) error {
				if data, err := t.DelDisk(name); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newAddDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "add a zvol-type dataset and map",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := AddDiskOptions{
					AddZvolOptions: AddZvolOptions{
						Name:          name,
						Size:          size,
						Blocksize:     blocksize,
						Sparse:        sparse,
						Deduplication: dedup,
						Compression:   compression,
					},
					InsecureTPC: insecureTPC,
					Mapping:     mapping,
				}
				if lunId >= 0 {
					opt.LunId = &lunId
				}
				if data, err := t.AddDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		cmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
		cmd.Flags().BoolVar(&sparse, "sparse", true, "")
		cmd.Flags().BoolVar(&insecureTPC, "insecure-tpc", false, "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.Flags().StringVar(&dedup, "dedup", "off", "")
		cmd.Flags().StringVar(&compression, "compression", "inherit", "")
		cmd.Flags().IntVar(&lunId, "lun", -1, "")
		return cmd
	}
	newAddISCSIZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:    "zvol",
			Short:  "add a zvol-type dataset",
			Hidden: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := AddDiskOptions{
					AddZvolOptions: AddZvolOptions{
						Name:          volume + "/" + name,
						Size:          size,
						Blocksize:     blocksize,
						Sparse:        sparse,
						Deduplication: dedup,
						Compression:   compression,
					},
					InsecureTPC: insecureTPC,
					Mapping:     mapping,
				}
				if lunId >= 0 {
					opt.LunId = &lunId
				}
				if data, err := t.AddDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&volume, "volume", "", "")
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		cmd.Flags().BoolVar(&sparse, "sparse", true, "")
		cmd.Flags().StringVar(&dedup, "dedup", "off", "")
		cmd.Flags().StringVar(&compression, "compression", "inherit", "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.Flags().IntVar(&lunId, "lun", -1, "")
		return cmd
	}
	newAddZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "zvol",
			Short: "add a zvol-type dataset",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := AddZvolOptions{
					Name:          name,
					Size:          size,
					Blocksize:     blocksize,
					Sparse:        sparse,
					Deduplication: dedup,
					Compression:   compression,
				}
				if data, err := t.AddZvol(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		cmd.Flags().BoolVar(&sparse, "sparse", true, "")
		cmd.Flags().StringVar(&dedup, "dedup", "off", "")
		cmd.Flags().StringVar(&compression, "compression", "inherit", "")
		return cmd
	}
	newDelZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "zvol",
			Short: "del a zvol-type dataset",
			RunE: func(_ *cobra.Command, _ []string) error {
				if data, err := t.DeleteDataset(name); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "get",
			Short: "get commands",
		}
		return cmd
	}
	newGetPoolsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "pools",
			Short: "get pools",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpPools()
			},
		}
		return cmd
	}
	newGetDatasetsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "datasets",
			Short: "get datasets",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpDatasets()
			},
		}
		return cmd
	}
	newGetDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "get dataset, extent and targetextents",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpDisk(name)
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetDatasetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "dataset",
			Short: "get dataset",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpDataset(name)
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetSystemInfoCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "system",
			Short: "get system information",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpSystemInfo()
			},
		}
		return cmd
	}
	newAddISCSICmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "iscsi",
			Short: "iscsi subsystem",
		}
		return cmd
	}
	newDelISCSICmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "iscsi",
			Short: "iscsi subsystem",
		}
		return cmd
	}
	newAddISCSIPortalCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "portal",
			Short: "create a iscsi portal",
			RunE: func(_ *cobra.Command, _ []string) error {
				listenParam := make([]ISCSIPortalListenIp, 0)
				for _, server := range listen {
					l := strings.SplitN(server, ":", 2)
					if len(l) != 2 {
						return fmt.Errorf("bad listen format: %s", server)
					}
					port, err := strconv.Atoi(l[1])
					if err != nil {
						return fmt.Errorf("bad listen port format: %s", server)
					}
					listenParam = append(listenParam, ISCSIPortalListenIp{
						Ip:   l[0],
						Port: port,
					})
				}

				params := CreateISCSIPortalParams{
					Comment:             comment,
					DiscoveryAuthMethod: authMethod,
					Listen:              listenParam,
				}

				if authGroupId >= 0 {
					params.DiscoveryAuthGroup = authGroupId
				}
				if data, err := t.addISCSIPortal(params); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&comment, "comment", "", "")
		cmd.Flags().IntVar(&authGroupId, "auth-group-id", -1, "")
		cmd.Flags().StringVar(&authMethod, "auth-method", "", "")
		cmd.Flags().StringSliceVar(&listen, "listen", []string{"0.0.0.0:3261"}, "")
		return cmd
	}
	newAddISCSITargetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "target",
			Short: "create a iscsi target",
			RunE: func(_ *cobra.Command, _ []string) error {
				params := CreateISCSITargetParams{
					Name: name,
				}
				if data, err := t.addISCSITarget(params); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newAddISCSITargetGroupCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "targetgroup",
			Short: "create a iscsi targetgroup",
			RunE: func(_ *cobra.Command, _ []string) error {
				params := AddISCSITargetGroupOptions{
					AuthMethod:    authMethod,
					Auth:          auth,
					PortalId:      portalId,
					Target:        target,
					InitiatorName: initiatorName,
					InitiatorId:   initiatorId,
				}
				if data, err := t.addISCSITargetGroup(params); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().IntVar(&portalId, "portal-id", 1, "")
		cmd.Flags().StringVar(&target, "target", "", "")
		cmd.Flags().StringVar(&auth, "auth", "", "")
		cmd.Flags().StringVar(&authMethod, "auth-method", "NONE", "")
		cmd.Flags().StringVar(&initiatorName, "initiatorgroup", "", "")
		cmd.Flags().StringVar(&initiatorName, "initiator", "", "")
		cmd.Flags().IntVar(&initiatorId, "initiator-id", -1, "")
		cmd.PersistentFlags().MarkHidden("initiatorgroup")
		return cmd
	}
	newDelISCSITargetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "target",
			Short: "delete a iscsi target",
			RunE: func(_ *cobra.Command, _ []string) error {
				if data, err := t.delISCSITarget(id); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().IntVar(&id, "id", -1, "")
		return cmd
	}
	newDelISCSIInitiatorCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "initiator",
			Short: "delete a iscsi initiator",
			RunE: func(_ *cobra.Command, _ []string) error {
				if data, err := t.delISCSIInitiator(id); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().IntVar(&id, "id", -1, "")
		return cmd
	}
	newAddISCSIInitiatorCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "initiator",
			Short: "create a iscsi initiator",
			RunE: func(_ *cobra.Command, _ []string) error {
				params := CreateISCSIInitiatorParams{
					Initiators:  initiators,
					AuthNetwork: authNetworks,
					Comment:     comment,
				}
				if data, err := t.addISCSIInitiator(params); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringSliceVar(&initiators, "initiator", []string{}, "")
		cmd.Flags().StringSliceVar(&authNetworks, "auth-network", []string{}, "")
		cmd.Flags().StringVar(&name, "comment", "", "")
		return cmd
	}
	newAddISCSIExtentCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extent",
			Short: "create a iscsi extent",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := AddISCSIExtentOptions{
					Name:        name,
					Disk:        disk,
					Blocksize:   blocksize,
					InsecureTPC: insecureTPC,
				}
				if data, err := t.AddISCSIExtent(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&disk, "disk", "", "")
		cmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
		cmd.Flags().BoolVar(&insecureTPC, "insecure-tpc", false, "")
		return cmd
	}
	newDelISCSIExtentCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extent",
			Short: "delete a iscsi extent",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := DelISCSIExtentOptions{
					Id:   id,
					Name: name,
				}
				if data, err := t.DelISCSIExtent(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().IntVar(&id, "id", -1, "")
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetISCSICmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "iscsi",
			Short: "iscsi subsystem",
		}
		return cmd
	}
	newGetISCSIPortalsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "portals",
			Short: "get iscsi portals",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpISCSIPortals()
			},
		}
		return cmd
	}
	newGetISCSITargetsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "targets",
			Short: "get iscsi targets",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpISCSITargets()
			},
		}
		return cmd
	}
	newGetISCSITargetExtentsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "targetextents",
			Short: "get iscsi targetextents",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpISCSITargetExtents()
			},
		}
		return cmd
	}
	newGetISCSIExtentCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extent",
			Short: "get iscsi extent",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpISCSIExtent(name)
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetISCSIExtentsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extents",
			Short: "get iscsi extents",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpISCSIExtents()
			},
		}
		return cmd
	}
	newGetISCSIInitiatorsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "initiators",
			Short: "get iscsi initiators",
			RunE: func(_ *cobra.Command, _ []string) error {
				return t.dumpISCSIInitiators()
			},
		}
		return cmd
	}
	newUpdateCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "update",
			Short: "update commands",
		}
		return cmd
	}
	newUpdateZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "dataset",
			Short: "update a dataset",
			RunE: func(_ *cobra.Command, _ []string) error {
				params := UpdateDatasetParams{}
				var (
					initialSize int64
					sign        string
				)
				if strings.HasPrefix(size, "+") || strings.HasPrefix(size, "-") {
					sign = string(size[0])
					size = size[1:]
					if ds, err := t.GetDataset(name); err != nil {
						return err
					} else if i, err := sizeconv.FromSize(ds.Volsize.Rawvalue); err != nil {
						return err
					} else {
						initialSize = i
					}
				}
				if i, err := sizeconv.FromSize(size); err != nil {
					return err
				} else {
					switch sign {
					case "+":
						initialSize += i
					case "-":
						initialSize -= i
					}
					params.Volsize = &initialSize
				}
				if data, err := t.UpdateDataset(name, params); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		return cmd
	}

	parent := newParent()

	// skip past the --array <array> arguments
	parent.SetArgs(array.SkipArgs())

	addCmd := newAddCmd()
	addCmd.AddCommand(newAddDiskCmd())
	addCmd.AddCommand(newAddZvolCmd())
	parent.AddCommand(addCmd)

	delCmd := newDelCmd()
	delCmd.AddCommand(newDelDiskCmd())
	delCmd.AddCommand(newDelZvolCmd())
	parent.AddCommand(delCmd)

	getCmd := newGetCmd()
	getCmd.AddCommand(newGetPoolsCmd())
	getCmd.AddCommand(newGetDatasetsCmd())
	getCmd.AddCommand(newGetDatasetCmd())
	getCmd.AddCommand(newGetDiskCmd())
	getCmd.AddCommand(newGetSystemInfoCmd())
	parent.AddCommand(getCmd)

	addISCSICmd := newAddISCSICmd()
	addISCSICmd.AddCommand(newAddISCSIExtentCmd())
	addISCSICmd.AddCommand(newAddISCSIInitiatorCmd())
	addISCSICmd.AddCommand(newAddISCSIPortalCmd())
	addISCSICmd.AddCommand(newAddISCSITargetCmd())
	addISCSICmd.AddCommand(newAddISCSITargetGroupCmd())
	addISCSICmd.AddCommand(newAddISCSIZvolCmd())
	addCmd.AddCommand(addISCSICmd)

	delISCSICmd := newDelISCSICmd()
	delISCSICmd.AddCommand(newDelISCSIExtentCmd())
	delISCSICmd.AddCommand(newDelISCSIInitiatorCmd())
	delISCSICmd.AddCommand(newDelISCSITargetCmd())
	delCmd.AddCommand(delISCSICmd)

	getISCSICmd := newGetISCSICmd()
	getISCSICmd.AddCommand(newGetISCSIPortalsCmd())
	getISCSICmd.AddCommand(newGetISCSITargetsCmd())
	getISCSICmd.AddCommand(newGetISCSITargetExtentsCmd())
	getISCSICmd.AddCommand(newGetISCSIExtentCmd())
	getISCSICmd.AddCommand(newGetISCSIExtentsCmd())
	getISCSICmd.AddCommand(newGetISCSIInitiatorsCmd())
	getCmd.AddCommand(getISCSICmd)

	mapCmd := newMapCmd()
	mapCmd.AddCommand(newMapDiskCmd())
	parent.AddCommand(mapCmd)

	mapISCSICmd := newMapISCSICmd()
	mapISCSICmd.AddCommand(newMapISCSIZvolCmd())
	mapCmd.AddCommand(mapISCSICmd)

	unmapCmd := newUnmapCmd()
	parent.AddCommand(unmapCmd)

	unmapISCSICmd := newUnmapISCSICmd()
	unmapISCSICmd.AddCommand(newUnmapISCSIZvolCmd())
	unmapCmd.AddCommand(unmapISCSICmd)

	updateCmd := newUpdateCmd()
	updateCmd.AddCommand(newUpdateZvolCmd())
	parent.AddCommand(updateCmd)

	return parent.Execute()
}

func (t Array) DelZvol(name string) (*Dataset, error) {
	datasets, err := t.GetDatasets()
	if err != nil {
		return nil, err
	}
	dataset, ok := datasets.GetByName(name)
	if !ok {
		return nil, fmt.Errorf("dataset not found")
	}
	path := fmt.Sprintf("/pool/dataset/%s", dataset.Id)
	req, err := t.newRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return dataset, err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return dataset, nil
}

func (t Array) delISCSIExtent(extent ISCSIExtent) error {
	path := fmt.Sprintf("/iscsi/extent/%d", extent.Id)
	req, err := t.newRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return err
	}
	return nil
}

func (t Array) DelISCSIExtent(opt DelISCSIExtentOptions) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents()
	if err != nil {
		return nil, err
	}
	var extent *ISCSIExtent
	if opt.Id >= 0 {
		extent = extents.GetById(opt.Id)
	} else if opt.Name != "" {
		extent = extents.GetByPath("zvol/" + opt.Name)
	}
	if extent == nil {
		return nil, fmt.Errorf("extent %v not found (%d scanned)", opt, len(extents))
	}
	return extent, t.delISCSIExtent(*extent)
}

func (t Array) AddISCSIExtent(opt AddISCSIExtentOptions) (*ISCSIExtent, error) {
	extent, err := t.GetISCSIExtent(opt.Name)
	if err != nil {
		return nil, err
	}
	if extent != nil {
		return extent, nil
	}
	params := CreateISCSIExtentParams{
		Name:        opt.Name,
		Disk:        opt.Disk,
		Type:        "DISK",
		InsecureTPC: opt.InsecureTPC,
	}
	if i, err := sizeconv.FromSize(opt.Blocksize); err != nil {
		return nil, err
	} else {
		params.Blocksize = int(i)
	}
	return t.createISCSIExtent(params)
}

func (t Array) AddZvol(opt AddZvolOptions) (*Dataset, error) {
	params, err := opt.Params()
	if err != nil {
		return nil, err
	}
	params.Type = &DatasetTypeVolume
	dataset, err := t.GetDataset(params.Name)
	if err != nil {
		return nil, err
	}
	if dataset != nil {
		return dataset, nil
	}
	return t.CreateDataset(params)
}

func (t Array) DelDisk(name string) (*Disk, error) {
	disk, err := t.GetDisk(name)
	if err != nil {
		return nil, err
	}
	if _, err := t.DelISCSIExtent(DelISCSIExtentOptions{Name: name}); err != nil {
		return nil, err
	}
	if _, err := t.DelZvol(name); err != nil {
		return nil, err
	}
	return disk, nil
}

func (t Array) AddDisk(opt AddDiskOptions) (*Disk, error) {
	disk := Disk{
		ISCSI: &DiskISCSI{},
	}
	if data, err := t.AddZvol(opt.AddZvolOptions); err != nil {
		return nil, err
	} else {
		disk.Dataset = data
	}

	// Extent
	extent, err := t.AddISCSIExtent(AddISCSIExtentOptions{
		Name:        opt.Name,
		Disk:        "zvol/" + opt.Name,
		Blocksize:   opt.Blocksize,
		InsecureTPC: opt.InsecureTPC,
	})
	if err != nil {
		return nil, err
	}
	disk.ISCSI.Extent = extent

	// targetExtent
	targetExtent, err := t.MapDisk(MapDiskOptions{
		Name:    opt.Name,
		Mapping: opt.Mapping,
		LunId:   opt.LunId,
	})
	if err != nil {
		return nil, err
	}
	disk.ISCSI.TargetExtents = targetExtent

	return &disk, nil
}

func (t Array) timeout() time.Duration {
	if timeout := t.Config().GetDuration(t.Key("timeout")); timeout == nil {
		return RequestTimeout
	} else {
		return *timeout
	}
}

func (t Array) insecure() bool {
	return t.Config().GetBool(t.Key("insecure"))
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t Array) passwordSec() (object.Sec, error) {
	secPathStr := t.Key("password")
	secName, err := t.Config().GetStringStrict(secPathStr)
	if err != nil {
		return nil, err
	}
	secPath, err := naming.ParsePath(secName)
	if err != nil {
		return nil, err
	}
	return object.NewSec(secPath, object.WithVolatile(true))
}

func (t Array) password() (string, error) {
	sec, err := t.passwordSec()
	if err != nil {
		return "", err
	}
	b, err := sec.DecodeKey("password")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (t Array) api() string {
	return t.Config().GetString(t.Key("api"))
}

func (t Array) GetPoolByName(name string) (Pool, error) {
	pools, err := t.GetPools()
	if err != nil {
		return Pool{}, err
	}
	for _, pool := range pools {
		if pool.Name == name {
			return pool, nil
		}
	}
	return Pool{}, fmt.Errorf("pool %s not found", name)
}

func dump(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func (t Array) dumpPools() error {
	data, err := t.GetPools()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) UpdateDataset(id string, params UpdateDatasetParams) (*Dataset, error) {
	path := fmt.Sprintf("/pool/dataset/id/%s", id)
	req, err := t.newRequest(http.MethodPut, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data Dataset
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) CreateDataset(params CreateDatasetParams) (*Dataset, error) {
	path := fmt.Sprintf("/pool/dataset")
	req, err := t.newRequest(http.MethodPost, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data Dataset
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) DeleteDataset(id string) (*Dataset, error) {
	dataset, err := t.GetDataset(id)
	if err != nil {
		return nil, err
	}
	if dataset == nil {
		return nil, fmt.Errorf("dataset %s does not exist", id)
	}
	path := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(id))
	req, err := t.newRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return dataset, err
	}
	return dataset, nil
}

func (t Array) GetPools() ([]Pool, error) {
	path := fmt.Sprintf("/pool")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make([]Pool, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (t Array) dumpISCSIPortals() error {
	data, err := t.GetISCSIPortals()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIPortals() ([]any, error) {
	path := fmt.Sprintf("/iscsi/portal")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (t Array) dumpISCSITargets() error {
	data, err := t.GetISCSITargets()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t *Array) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := t.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := validateResponse(resp); err != nil {
		return resp, fmt.Errorf("validate response: %w", err)
	}

	err = decodeResponse(resp, v)
	if err != nil {
		return resp, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}

func (t Array) GetISCSITargets() (ISCSITargets, error) {
	path := fmt.Sprintf("/iscsi/target")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make(ISCSITargets, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (t Array) GetISCSIExtent(name string) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents()
	if err != nil {
		return nil, err
	}
	return extents.GetByName(name), nil
}

func (t Array) dumpISCSIExtent(name string) error {
	data, err := t.GetISCSIExtent(name)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) dumpISCSITargetExtents() error {
	data, err := t.GetISCSITargetExtents()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSITargetExtents() (ISCSITargetExtents, error) {
	path := fmt.Sprintf("/iscsi/targetextent")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make(ISCSITargetExtents, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (t Array) dumpISCSIExtents() error {
	data, err := t.GetISCSIExtents()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIExtents() (ISCSIExtents, error) {
	path := fmt.Sprintf("/iscsi/extent")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make(ISCSIExtents, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (t Array) dumpISCSIInitiators() error {
	data, err := t.GetISCSIInitiators()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIInitiators() (ISCSIInitiators, error) {
	path := fmt.Sprintf("/iscsi/initiator")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make(ISCSIInitiators, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (t Array) dumpSystemInfo() error {
	data, err := t.GetSystemInfo()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetSystemInfo() (*SystemInfo, error) {
	path := fmt.Sprintf("/system/info")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var data SystemInfo
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) dumpDisk(name string) error {
	data, err := t.GetDisk(name)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDisk(name string) (*Disk, error) {
	disk := Disk{
		ISCSI: &DiskISCSI{},
	}

	dataset, err := t.GetDataset(name)
	if err != nil {
		return nil, err
	}
	disk.Dataset = dataset

	extents, err := t.GetISCSIExtents()
	if err != nil {
		return nil, err
	}
	switch dataset.Type {
	case "VOLUME":
		extents = extents.WithType("DISK").WithPath("zvol/" + name)
	case "FILESYSTEM":
		extents = extents.WithType("FILE").WithPath(*dataset.Mountpoint)
	default:
		return nil, errors.Errorf("unsupported %s dataset type: %s", name, dataset.Type)
	}
	if len(extents) == 1 {
		extent := extents[0]
		disk.ISCSI.Extent = &extent
		if targetExtents, err := t.GetISCSITargetExtents(); err != nil {
			return nil, err
		} else {
			disk.ISCSI.TargetExtents = targetExtents.WithExtent(extent)
		}
	}
	return &disk, nil
}

func (t Array) dumpDatasets() error {
	data, err := t.GetDatasets()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDatasets() (Datasets, error) {
	path := fmt.Sprintf("/pool/dataset")
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	items := make(Datasets, 0)
	_, err = t.Do(req, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (t Array) dumpDataset(id string) error {
	data, err := t.GetDataset(id)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDataset(id string) (*Dataset, error) {
	path := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(id))
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var data Dataset
	resp, err := t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	return &data, nil
}

func (t Array) UnmapDisk(opt UnmapDiskOptions) (ISCSITargetExtents, error) {
	deletedTargetExtents := make(ISCSITargetExtents, 0)
	paths, err := san.ParseMapping(opt.Mapping)
	if err != nil {
		return deletedTargetExtents, err
	} else if len(paths) == 0 {
		return deletedTargetExtents, nil
	}
	targets, err := t.GetISCSITargets()
	if err != nil {
		return deletedTargetExtents, err
	}
	extents, err := t.GetISCSIExtents()
	if err != nil {
		return deletedTargetExtents, err
	}
	targetextents, err := t.GetISCSITargetExtents()
	if err != nil {
		return deletedTargetExtents, err
	}
	for _, p := range paths {
		target, ok := targets.GetByName(p.Target.Name)
		if !ok {
			continue
		}
		extentName := "zvol/" + opt.Name
		extent := extents.GetByPath(extentName)
		if extent == nil {
			continue
		}
		filteredTargetextents := targetextents.WithExtent(*extent).WithTarget(target)
		if len(filteredTargetextents) == 0 {
			continue
		} else if len(filteredTargetextents) > 1 {
			return deletedTargetExtents, fmt.Errorf("too many (%d) target extents for path %s", len(filteredTargetextents), p)
		}
		filteredTargetextent := filteredTargetextents[0]
		if err := t.delISCSITargetExtent(filteredTargetextent.Id); err != nil {
			return deletedTargetExtents, err
		}
		deletedTargetExtents = append(deletedTargetExtents, filteredTargetextent)
	}
	return deletedTargetExtents, nil
}

func (t Array) MapDisk(opt MapDiskOptions) (ISCSITargetExtents, error) {
	missingTargetExtents := make(ISCSITargetExtents, 0)
	paths, err := san.ParseMapping(opt.Mapping)
	if err != nil {
		return missingTargetExtents, err
	} else if len(paths) == 0 {
		return missingTargetExtents, nil
	}
	targets, err := t.GetISCSITargets()
	if err != nil {
		return missingTargetExtents, err
	}
	extents, err := t.GetISCSIExtents()
	if err != nil {
		return missingTargetExtents, err
	}
	targetextents, err := t.GetISCSITargetExtents()
	if err != nil {
		return missingTargetExtents, err
	}
	for _, p := range paths {
		target, ok := targets.GetByName(p.Target.Name)
		if !ok {
			return missingTargetExtents, fmt.Errorf("target %s not found (%d scanned)", p.Target.Name, len(targets))
		}
		extentName := "zvol/" + opt.Name
		extent := extents.GetByPath(extentName)
		if extent == nil {
			return missingTargetExtents, fmt.Errorf("extent %s not found (%d scanned)", extentName, len(extents))
		}
		filteredTargetextents := targetextents.WithExtent(*extent).WithTarget(target)
		if len(filteredTargetextents) == 1 {
			missingTargetExtents = append(missingTargetExtents, filteredTargetextents[0])
			continue
		}
		params := CreateISCSITargetExtentParams{
			Target: target.Id,
			Extent: extent.Id,
			LunId:  opt.LunId,
		}
		d, err := t.createISCSITargetExtent(params)
		if err != nil {
			return missingTargetExtents, err
		}
		missingTargetExtents = append(missingTargetExtents, *d)
	}
	return missingTargetExtents, nil
}

func (t Array) createISCSITargetExtent(params CreateISCSITargetExtentParams) (*ISCSITargetExtent, error) {
	path := fmt.Sprintf("/iscsi/targetextent")
	req, err := t.newRequest(http.MethodPost, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data ISCSITargetExtent
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) createISCSIExtent(params CreateISCSIExtentParams) (*ISCSIExtent, error) {
	path := fmt.Sprintf("/iscsi/extent")
	req, err := t.newRequest(http.MethodPost, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data ISCSIExtent
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) getISCSITarget(id int) (*ISCSITarget, error) {
	path := fmt.Sprintf("/iscsi/target/id/%d", id)
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var data ISCSITarget
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) delISCSITargetExtent(id int) error {
	path := fmt.Sprintf("/iscsi/targetextent/id/%d", id)
	// true as a body payload forces the delete of a in-use target extent
	req, err := t.newRequest(http.MethodDelete, path, nil, true)
	if err != nil {
		return err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return err
	}
	return nil
}

func (t Array) delISCSITarget(id int) (*ISCSITarget, error) {
	target, err := t.getISCSITarget(id)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/iscsi/target/id/%d", id)
	req, err := t.newRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return target, err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return target, err
	}
	return target, nil
}

func (t Array) getISCSIInitiator(id int) (*ISCSIInitiator, error) {
	path := fmt.Sprintf("/iscsi/initiator/id/%d", id)
	req, err := t.newRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var data ISCSIInitiator
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) delISCSIInitiator(id int) (*ISCSIInitiator, error) {
	initiator, err := t.getISCSIInitiator(id)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/iscsi/initiator/id/%d", id)
	req, err := t.newRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return initiator, err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return initiator, err
	}
	return initiator, nil
}

func (t Array) addISCSIInitiator(params CreateISCSIInitiatorParams) (*ISCSIInitiator, error) {
	path := fmt.Sprintf("/iscsi/initiator")
	req, err := t.newRequest(http.MethodPost, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data ISCSIInitiator
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) addISCSITargetGroup(opt AddISCSITargetGroupOptions) (ISCSITargets, error) {
	initiators := make(ISCSIInitiators, 0)
	if opt.InitiatorId >= 0 {
		initiator, err := t.getISCSIInitiator(opt.InitiatorId)
		if err != nil {
			return nil, err
		}
		if initiator == nil {
			return nil, fmt.Errorf("initiator id %d not found", opt.InitiatorId)
		}
		initiators = append(initiators, *initiator)
	} else if opt.InitiatorName != "" {
		l, err := t.GetISCSIInitiators()
		if err != nil {
			return nil, err
		}
		initiators = l.WithName(opt.InitiatorName)
	}

	targets := make(ISCSITargets, 0)
	if opt.Target != "" {
		l, err := t.GetISCSITargets()
		if err != nil {
			return nil, err
		}
		targets = l.WithName(opt.Target)
	}

	targetsChanged := make(map[int]ISCSITarget)

	targetHasGroup := func(target ISCSITarget, targetGroup ISCSITargetGroup) bool {
		for _, i := range target.Groups {
			if i.PortalId == targetGroup.PortalId && i.InitiatorId == targetGroup.InitiatorId && i.AuthMethod == targetGroup.AuthMethod {
				return true
			}
		}
		return false
	}

	for _, target := range targets {
		for _, initiator := range initiators {
			targetGroup := ISCSITargetGroup{
				PortalId:    opt.PortalId,
				InitiatorId: initiator.Id,
				AuthMethod:  opt.AuthMethod,
			}
			if opt.Auth != "" {
				targetGroup.Auth = &opt.Auth
			}
			if targetHasGroup(target, targetGroup) {
				continue
			}
			target.Groups = append(target.Groups, targetGroup)
		}
		path := fmt.Sprintf("/iscsi/target/id/%d", target.Id)
		params := UpdateISCSITargetParams{
			Name:   target.Name,
			Mode:   target.Mode,
			Groups: target.Groups,
		}
		if target.Alias != nil {
			params.Alias = *target.Alias
		}
		req, err := t.newRequest(http.MethodPut, path, nil, params)
		if err != nil {
			return nil, err
		}
		var data ISCSITarget
		_, err = t.Do(req, &data)
		if err != nil {
			return nil, err
		}
		targetsChanged[target.Id] = data
	}

	targetsChangedSlice := make(ISCSITargets, 0)
	for _, target := range targetsChanged {
		targetsChangedSlice = append(targetsChangedSlice, target)
	}
	return targetsChangedSlice, nil
}

func (t Array) addISCSITarget(params CreateISCSITargetParams) (*ISCSITarget, error) {
	path := fmt.Sprintf("/iscsi/target")
	req, err := t.newRequest(http.MethodPost, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data ISCSITarget
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t Array) addISCSIPortal(params CreateISCSIPortalParams) (*ISCSIPortal, error) {
	path := fmt.Sprintf("/iscsi/portal")
	req, err := t.newRequest(http.MethodPost, path, nil, params)
	if err != nil {
		return nil, err
	}
	var data ISCSIPortal
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (t *Array) client() *http.Client {
	return &http.Client{
		Timeout: t.timeout(),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: t.insecure(),
			},
		},
	}
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (t *Array) newRequest(method string, path string, params map[string]string, data interface{}) (*http.Request, error) {
	fpath := t.api() + Head + path
	baseURL, err := url.Parse(fpath)
	if err != nil {
		return nil, err
	}
	if params != nil {
		ps := url.Values{}
		for k, v := range params {
			ps.Set(k, v)
		}
		baseURL.RawQuery = ps.Encode()
	}
	req, err := http.NewRequest(method, baseURL.String(), nil)
	if err != nil {
		return nil, err
	}
	if data != nil {
		jsonString, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, baseURL.String(), bytes.NewBuffer(jsonString))
		if err != nil {
			return nil, err
		}
	}

	password, err := t.password()
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Basic "+basicAuth(t.username(), password))
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	return req, err
}

// DiskId return the NAA from the created disk dataset
func (t Array) DiskId(disk Disk) string {
	return strings.TrimPrefix(disk.ISCSI.Extent.NAA, "0x")
}

// DiskPaths return the san paths list from the created disk dataset and api query responses
func (t Array) DiskPaths(disk Disk) (san.Paths, error) {
	paths := san.Paths{}
	targets, err := t.GetISCSITargets()
	if err != nil {
		return paths, err
	}
	initiators, err := t.GetISCSIInitiators()
	if err != nil {
		return paths, err
	}
	for _, targetextent := range disk.ISCSI.TargetExtents {
		target, ok := targets.GetById(targetextent.TargetId)
		if !ok {
			return paths, fmt.Errorf("target id %d not found", targetextent.TargetId)
		}
		pathTarget := san.Target{
			Name: target.Name,
			Type: san.ISCSI,
		}
		for _, group := range target.Groups {
			initiator, ok := initiators.GetById(group.InitiatorId)
			if !ok {
				return paths, fmt.Errorf("initiator id %d not found", group.InitiatorId)
			}
			for _, iqn := range initiator.Initiators {
				pathInitiator := san.Initiator{
					Name: iqn,
					Type: san.ISCSI,
				}
				paths = append(paths, san.Path{
					Initiator: pathInitiator,
					Target:    pathTarget,
				})
			}
		}
	}
	return paths, nil
}

func validateResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	return fmt.Errorf("Response code: %d, Response body: %s", r.StatusCode, bodyString)
}

// decodeResponse function reads the http response body into an interface.
func decodeResponse(r *http.Response, v interface{}) error {
	if r.StatusCode == 204 {
		return nil
	}
	if v == nil {
		return fmt.Errorf("nil interface provided to decodeResponse")
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	if len(bodyBytes) == 0 {
		return nil
	}

	bodyString := string(bodyBytes)
	//fmt.Println(bodyString)

	err := json.Unmarshal([]byte(bodyString), &v)

	return err
}

func (t AddZvolOptions) Params() (CreateDatasetParams, error) {
	dedupParam := strings.ToUpper(t.Deduplication)
	compressionParam := t.Compression
	compressionParam = strings.ToUpper(compressionParam)

	params := CreateDatasetParams{
		Name:          t.Name,
		Volblocksize:  &t.Blocksize,
		Sparse:        &t.Sparse,
		Deduplication: &dedupParam,
	}
	if compressionParam != "INHERIT" {
		params.Compression = &compressionParam
	}
	if i, err := sizeconv.FromSize(t.Size); err != nil {
		return params, err
	} else {
		params.Volsize = &i
	}
	return params, nil
}
