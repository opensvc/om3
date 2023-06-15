package arrayfreenas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/sizeconv"
)

var (
	DatasetTypeVolume     = "VOLUME"
	DatasetTypeFilesystem = "FILESYSTEM"
)

type (
	Array struct {
		*array.Array
	}
	Disk struct {
		Dataset *Dataset   `json:"dataset"`
		ISCSI   *DiskISCSI `json:"iscsi,omitempty"`
	}
	DiskISCSI struct {
		Extent        *ISCSIExtent        `json:"extent,omitempty"`
		TargetExtents []ISCSITargetExtent `json:"targetextents,omitempty"`
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
		blocksize   string
		name        string
		disk        string
		size        string
		mapping     string
		sparse      bool
		insecureTPC bool
		lunID       int
	)
	newParent := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "array",
			Short: "Manage a truenas storage array",
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
	newUnmapCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "unmap",
			Short: "unmap commands",
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

	newUnmapDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "unmap a zvol-type dataset",
			Run: func(_ *cobra.Command, _ []string) {
				fmt.Println("TODO")
			},
		}
		return cmd
	}
	newMapDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "map a zvol-type dataset",
			Run: func(cmd *cobra.Command, _ []string) {
				var lunIDPtr *int
				if cmd.Flag("lun").Changed {
					lunIDPtr = &lunID
				}
				if data, err := t.MapDisk(name, mapping, lunIDPtr); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.Flags().IntVar(&lunID, "lun", -1, "")
		return cmd
	}
	newDelDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "unmap a zvol-type dataset and delete",
			Run: func(_ *cobra.Command, _ []string) {
				if data, err := t.DelDisk(name); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
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
			Run: func(cmd *cobra.Command, _ []string) {
				var lunIDPtr *int
				if cmd.Flag("lun").Changed {
					lunIDPtr = &lunID
				}
				if data, err := t.AddDisk(name, size, blocksize, sparse, insecureTPC, mapping, lunIDPtr); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		cmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
		cmd.Flags().BoolVar(&sparse, "sparse", true, "")
		cmd.Flags().BoolVar(&insecureTPC, "insecure-tpc", false, "")
		cmd.Flags().StringVar(&mapping, "mapping", "", "")
		cmd.Flags().IntVar(&lunID, "lun", -1, "")
		return cmd
	}
	newAddZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "zvol",
			Short: "add a zvol-type dataset",
			Run: func(_ *cobra.Command, _ []string) {
				if data, err := t.addZvol(name, size, blocksize, sparse); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		cmd.Flags().BoolVar(&sparse, "sparse", true, "")
		return cmd
	}
	newDelZvolCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "zvol",
			Short: "del a zvol-type dataset",
			Run: func(_ *cobra.Command, _ []string) {
				if data, err := t.DeleteDataset(name); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
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
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpPools()
			},
		}
		return cmd
	}
	newGetDatasetsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "datasets",
			Short: "get datasets",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpDatasets()
			},
		}
		return cmd
	}
	newGetDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "get dataset, extent and targetextents",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpDisk(name)
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetDatasetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "dataset",
			Short: "get dataset",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpDataset(name)
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetSystemInfoCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "system",
			Short: "get system information",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpSystemInfo()
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
	newAddISCSIExtentCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extent",
			Short: "create a iscsi extent",
			Run: func(_ *cobra.Command, _ []string) {
				if data, err := t.addISCSIExtent(name, disk, blocksize, insecureTPC); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
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
			Short: "create a iscsi extent",
			Run: func(_ *cobra.Command, _ []string) {
				if data, err := t.addISCSIExtent(name, disk, blocksize, insecureTPC); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
				}
			},
		}
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
	newGetISCSITargetsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "targets",
			Short: "get iscsi targets",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpISCSITargets()
			},
		}
		return cmd
	}
	newGetISCSITargetExtentsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "targetextents",
			Short: "get iscsi targetextents",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpISCSITargetExtents()
			},
		}
		return cmd
	}
	newGetISCSIExtentCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extent",
			Short: "get iscsi extent",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpISCSIExtent(name)
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		return cmd
	}
	newGetISCSIExtentsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "extents",
			Short: "get iscsi extents",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpISCSIExtents()
			},
		}
		return cmd
	}
	newGetISCSIInitiatorsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "initiators",
			Short: "get iscsi initiators",
			Run: func(_ *cobra.Command, _ []string) {
				t.dumpISCSIInitiators()
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
			Run: func(_ *cobra.Command, _ []string) {
				params := UpdateDatasetParams{}
				var (
					initialSize int64
					sign        string
				)
				if strings.HasPrefix(size, "+") || strings.HasPrefix(size, "-") {
					sign = string(size[0])
					size = size[1:]
					if ds, err := t.GetDataset(name); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					} else if i, err := sizeconv.FromSize(ds.Volsize.Rawvalue); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					} else {
						initialSize = i
					}
				}
				if i, err := sizeconv.FromSize(size); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
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
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				} else {
					dump(data)
				}
			},
		}
		cmd.Flags().StringVar(&name, "name", "", "")
		cmd.Flags().StringVar(&size, "size", "", "")
		return cmd
	}

	parent := newParent()

	// skip past the --array <array> arguments
	parent.SetArgs(os.Args[4:])

	addCmd := newAddCmd()
	addCmd.AddCommand(newAddDiskCmd())
	addCmd.AddCommand(newAddZvolCmd())
	addCmd.AddCommand(newAddISCSICmd())
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
	addCmd.AddCommand(addISCSICmd)

	delISCSICmd := newDelISCSICmd()
	delISCSICmd.AddCommand(newDelISCSIExtentCmd())
	delCmd.AddCommand(delISCSICmd)

	getISCSICmd := newGetISCSICmd()
	getISCSICmd.AddCommand(newGetISCSITargetsCmd())
	getISCSICmd.AddCommand(newGetISCSITargetExtentsCmd())
	getISCSICmd.AddCommand(newGetISCSIExtentCmd())
	getISCSICmd.AddCommand(newGetISCSIExtentsCmd())
	getISCSICmd.AddCommand(newGetISCSIInitiatorsCmd())
	getCmd.AddCommand(getISCSICmd)

	mapCmd := newMapCmd()
	mapCmd.AddCommand(newMapDiskCmd())
	parent.AddCommand(mapCmd)

	unmapCmd := newUnmapCmd()
	unmapCmd.AddCommand(newUnmapDiskCmd())
	parent.AddCommand(unmapCmd)

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
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	r, err := client.DeleteDatasetWithResponse(context.Background(), fmt.Sprint(dataset.Id))
	if err != nil {
		return nil, err
	}
	if err := parseRespError(r.Body); err != nil {
		return nil, err
	}
	return &dataset, nil
}

func (t Array) delISCSIExtent(extent ISCSIExtent) error {
	client, err := t.client()
	if err != nil {
		return err
	}
	r, err := client.DeleteISCSIExtentWithResponse(context.Background(), fmt.Sprint(extent.Id))
	if err != nil {
		return err
	}
	if err := parseRespError(r.Body); err != nil {
		return err
	}
	return nil
}

func (t Array) DelISCSIExtent(name string) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents()
	if err != nil {
		return nil, err
	}
	extent, ok := extents.GetByPath("zvol/" + name)
	if !ok {
		return nil, fmt.Errorf("extent %s not found (%d scanned)", name, len(extents))
	}
	return &extent, t.delISCSIExtent(extent)
}

func (t Array) addISCSIExtent(name, disk, blocksize string, insecureTPC bool) (*ISCSIExtent, error) {
	extent, err := t.GetISCSIExtent(name)
	if err != nil {
		return nil, err
	}
	if extent != nil {
		return extent, nil
	}
	params := CreateISCSIExtentParams{
		Name:        name,
		Disk:        disk,
		Type:        "DISK",
		InsecureTPC: insecureTPC,
	}
	if i, err := sizeconv.FromSize(blocksize); err != nil {
		return nil, err
	} else {
		params.Blocksize = int(i)
	}
	return t.createISCSIExtent(params)
}

func (t Array) addZvol(name, size, blocksize string, sparse bool) (*Dataset, error) {
	dataset, err := t.GetDataset(name)
	if err != nil {
		return nil, err
	}
	if dataset != nil {
		return dataset, nil
	}
	params := CreateDatasetParams{
		Name:         name,
		Type:         &DatasetTypeVolume,
		Volblocksize: &blocksize,
		Sparse:       &sparse,
	}
	if i, err := sizeconv.FromSize(size); err != nil {
		return nil, err
	} else {
		params.Volsize = &i
	}
	return t.CreateDataset(params)
}

func (t Array) DelDisk(name string) (*Disk, error) {
	disk, err := t.GetDisk(name)
	if err != nil {
		return nil, err
	}
	if _, err := t.DelISCSIExtent(name); err != nil {
		return nil, err
	}
	if _, err := t.DelZvol(name); err != nil {
		return nil, err
	}
	return disk, nil
}

func (t Array) AddDisk(name, size, blocksize string, sparse, insecureTPC bool, mapping string, lunID *int) (*Disk, error) {
	disk := Disk{
		ISCSI: &DiskISCSI{},
	}
	if data, err := t.addZvol(name, size, blocksize, sparse); err != nil {
		return nil, err
	} else {
		disk.Dataset = data
	}
	if data, err := t.addISCSIExtent(name, "zvol/"+name, blocksize, insecureTPC); err != nil {
		return nil, err
	} else {
		disk.ISCSI.Extent = data
	}
	if data, err := t.MapDisk(name, mapping, lunID); err != nil {
		return nil, err
	} else {
		disk.ISCSI.TargetExtents = data
	}
	return &disk, nil
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t Array) timeout() string {
	return t.Config().GetString(t.Key("timeout"))
}

func (t Array) passwordSec() (object.Sec, error) {
	secPathStr := t.Key("password")
	secName, err := t.Config().GetStringStrict(secPathStr)
	if err != nil {
		return nil, err
	}
	secPath, err := path.Parse(secName)
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

func (t Array) api() (*url.URL, error) {
	k := t.Key("api")
	s, err := t.Config().GetStringStrict(k)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/v2.0"
	return u, nil
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
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	r, err := client.UpdateDatasetWithResponse(context.Background(), id, params)
	if err != nil {
		return nil, err
	}
	if r.JSON200 != nil {
		return r.JSON200, nil
	}
	if err := parseRespError(r.Body); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", string(r.Body))
}

func (t Array) CreateDataset(params CreateDatasetParams) (*Dataset, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	r, err := client.CreateDatasetWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	if r.JSON200 != nil {
		return r.JSON200, nil
	}
	if err := parseRespError(r.Body); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", string(r.Body))
}

func (t Array) DeleteDataset(id string) (*Dataset, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	dataset, err := t.GetDataset(id)
	if err != nil {
		return nil, err
	}
	r, err := client.DeleteDatasetWithResponse(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if r.StatusCode() == http.StatusOK {
		return dataset, nil
	}
	return nil, fmt.Errorf("%s", r.Status())
}

func (t Array) GetPools() ([]Pool, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetPoolsParams{}
	r, err := client.GetPoolsWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	data := *r.JSON200
	return data, nil
}

func (t Array) dumpISCSITargets() error {
	data, err := t.GetISCSITargets()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSITargets() (ISCSITargets, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetISCSITargetsParams{}
	r, err := client.GetISCSITargetsWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	data := *r.JSON200
	return data, nil
}

func (t Array) GetISCSIExtent(name string) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents()
	if err != nil {
		return nil, err
	}
	extent, ok := extents.GetByName(name)
	if !ok {
		return nil, err
	}
	return &extent, nil
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
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetISCSITargetExtentsParams{}
	r, err := client.GetISCSITargetExtentsWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	data := *r.JSON200
	return data, nil
}

func (t Array) dumpISCSIExtents() error {
	data, err := t.GetISCSIExtents()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIExtents() (ISCSIExtents, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetISCSIExtentsParams{}
	r, err := client.GetISCSIExtentsWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	data := *r.JSON200
	return data, nil
}

func (t Array) dumpISCSIInitiators() error {
	data, err := t.GetISCSIInitiators()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIInitiators() (ISCSIInitiators, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetISCSIInitiatorsParams{}
	r, err := client.GetISCSIInitiatorsWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	data := *r.JSON200
	return data, nil
}

func (t Array) dumpSystemInfo() error {
	data, err := t.GetSystemInfo()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetSystemInfo() (*SystemInfo, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	r, err := client.GetSystemInfoWithResponse(context.Background())
	if err != nil {
		return nil, err
	}
	data := r.JSON200
	return data, nil
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
	if data, err := t.GetDataset(name); err != nil {
		return nil, err
	} else {
		disk.Dataset = data
	}
	if data, err := t.GetISCSIExtent(name); err != nil {
		return nil, err
	} else if data != nil {
		disk.ISCSI.Extent = data
		if data, err := t.GetISCSITargetExtents(); err != nil {
			return nil, err
		} else {
			disk.ISCSI.TargetExtents = data.WithExtent(*disk.ISCSI.Extent)
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
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetDatasetsParams{}
	r, err := client.GetDatasetsWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	data := r.JSON200
	return data, nil
}

func (t Array) dumpDataset(id string) error {
	data, err := t.GetDataset(id)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDataset(id string) (*Dataset, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	params := &GetDatasetParams{}
	r, err := client.GetDatasetWithResponse(context.Background(), id, params)
	if err != nil {
		return nil, err
	}
	return r.JSON200, err
}

func (t Array) MapDisk(name, mapping string, lunID *int) (ISCSITargetExtentsResponse, error) {
	missingTargetExtents := make(ISCSITargetExtentsResponse, 0)
	paths, err := san.ParseMapping(mapping)
	if err != nil {
		return missingTargetExtents, err
	} else if len(paths) == 0 {
		return missingTargetExtents, fmt.Errorf("no paths parsed from mapping")
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
		extentName := "zvol/" + name
		extent, ok := extents.GetByPath(extentName)
		if !ok {
			return missingTargetExtents, fmt.Errorf("extent %s not found (%d scanned)", extentName, len(extents))
		}
		filteredTargetextents := targetextents.WithExtent(extent).WithTarget(target)
		if len(filteredTargetextents) == 1 {
			missingTargetExtents = append(missingTargetExtents, filteredTargetextents[0])
			continue
		}
		params := CreateISCSITargetExtentParams{
			Target: target.Id,
			Extent: extent.Id,
			LunID:  lunID,
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
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	r, err := client.CreateISCSITargetExtentWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	if r.JSON200 != nil {
		return r.JSON200, nil
	}
	if err := parseRespError(r.Body); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", string(r.Body))
}

func (t Array) createISCSIExtent(params CreateISCSIExtentParams) (*ISCSIExtent, error) {
	client, err := t.client()
	if err != nil {
		return nil, err
	}
	r, err := client.CreateISCSIExtentWithResponse(context.Background(), params)
	if err != nil {
		return nil, err
	}
	if r.JSON200 != nil {
		return r.JSON200, nil
	}
	if err := parseRespError(r.Body); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", string(r.Body))
}

func (t Array) client() (*ClientWithResponses, error) {
	api, err := t.api()
	if err != nil {
		return nil, fmt.Errorf("load api url: %w", err)
	}
	username := t.username()
	password, err := t.password()
	if err != nil {
		return nil, fmt.Errorf("load password: %w", err)
	}
	basicAuthProvider, err := securityprovider.NewSecurityProviderBasicAuth(username, password)
	if err != nil {
		return nil, fmt.Errorf("new openapi basic auth security provider: %w", err)
	}
	client, err := NewClientWithResponses(api.String(), WithRequestEditorFn(basicAuthProvider.Intercept))
	if err != nil {
		return nil, fmt.Errorf("new openapi client: %w", err)
	}
	return client, nil
}

// DiskID return the NAA from the created disk dataset
func (t Array) DiskID(disk Disk) string {
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
		target, ok := targets.GetByID(targetextent.TargetId)
		if !ok {
			return paths, fmt.Errorf("target id %d not found", targetextent.TargetId)
		}
		pathTarget := san.Target{
			Name: target.Name,
			Type: san.ISCSI,
		}
		for _, group := range target.Groups {
			initiator, ok := initiators.GetByID(group.InitiatorId)
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
