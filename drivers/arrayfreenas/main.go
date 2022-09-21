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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/array"
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/sizeconv"
)

var (
	DatasetTypeVolume     = "VOLUME"
	DatasetTypeFilesystem = "FILESYSTEM"
)

type (
	Array struct {
		*array.Array
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
		datasetName string
		size        string
		sparse      bool
	)
	parent := &cobra.Command{
		Use:   "array",
		Short: "Manage a truenas storage array",
	}
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "add commands",
	}
	addZvolCmd := &cobra.Command{
		Use:   "zvol",
		Short: "add a zvol-type dataset",
		Run: func(_ *cobra.Command, _ []string) {
			params := CreateDatasetParams{
				Name:         datasetName,
				Type:         &DatasetTypeVolume,
				Volblocksize: &blocksize,
				Sparse:       &sparse,
			}
			if i, err := sizeconv.FromSize(size); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			} else {
				params.Volsize = &i
			}
			if data, err := t.CreateDataset(params); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			} else {
				dump(data)
			}
		},
	}
	addZvolCmd.Flags().StringVar(&datasetName, "name", "", "")
	addZvolCmd.Flags().StringVar(&blocksize, "blocksize", "512", "")
	addZvolCmd.Flags().StringVar(&size, "size", "", "")
	addZvolCmd.Flags().BoolVar(&sparse, "sparse", true, "")

	delCmd := &cobra.Command{
		Use:   "del",
		Short: "del commands",
	}
	delZvolCmd := &cobra.Command{
		Use:   "zvol",
		Short: "del a zvol-type dataset",
		Run: func(_ *cobra.Command, _ []string) {
			if data, err := t.DeleteDataset(datasetName); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			} else {
				dump(data)
			}
		},
	}
	delZvolCmd.Flags().StringVar(&datasetName, "name", "", "")

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "get commands",
	}
	getPoolsCmd := &cobra.Command{
		Use:   "pools",
		Short: "get pools",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpPools()
		},
	}
	getDatasetsCmd := &cobra.Command{
		Use:   "datasets",
		Short: "get datasets",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpDatasets()
		},
	}
	getDatasetCmd := &cobra.Command{
		Use:   "dataset",
		Short: "get dataset",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpDataset(datasetName)
		},
	}
	getDatasetCmd.Flags().StringVar(&datasetName, "name", "", "")
	getSystemInfoCmd := &cobra.Command{
		Use:   "system",
		Short: "get system information",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpSystemInfo()
		},
	}
	getISCSICmd := &cobra.Command{
		Use:   "iscsi",
		Short: "iscsi subsystem",
	}
	getISCSITargetsCmd := &cobra.Command{
		Use:   "targets",
		Short: "get iscsi targets",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpISCSITargets()
		},
	}
	getISCSITargetExtentsCmd := &cobra.Command{
		Use:   "targetextents",
		Short: "get iscsi targetextents",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpISCSITargetExtents()
		},
	}
	getISCSIExtentsCmd := &cobra.Command{
		Use:   "extents",
		Short: "get iscsi extents",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpISCSIExtents()
		},
	}
	getISCSIInitiatorsCmd := &cobra.Command{
		Use:   "initiators",
		Short: "get iscsi initiators",
		Run: func(_ *cobra.Command, _ []string) {
			t.dumpISCSIInitiators()
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "update commands",
	}
	updateZvolCmd := &cobra.Command{
		Use:   "zvol",
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
				if ds, err := t.GetDataset(datasetName); err != nil {
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
			if data, err := t.UpdateDataset(datasetName, params); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			} else {
				dump(data)
			}
		},
	}
	updateZvolCmd.Flags().StringVar(&datasetName, "name", "", "")
	updateZvolCmd.Flags().StringVar(&size, "size", "", "")

	parent.SetArgs(os.Args[4:])

	parent.AddCommand(addCmd)
	addCmd.AddCommand(addZvolCmd)
	parent.AddCommand(delCmd)
	delCmd.AddCommand(delZvolCmd)
	parent.AddCommand(getCmd)
	getCmd.AddCommand(getPoolsCmd)
	getCmd.AddCommand(getDatasetsCmd)
	getCmd.AddCommand(getDatasetCmd)
	getCmd.AddCommand(getSystemInfoCmd)
	getCmd.AddCommand(getISCSICmd)
	getISCSICmd.AddCommand(getISCSITargetsCmd)
	getISCSICmd.AddCommand(getISCSITargetExtentsCmd)
	getISCSICmd.AddCommand(getISCSIExtentsCmd)
	getISCSICmd.AddCommand(getISCSIInitiatorsCmd)
	parent.AddCommand(updateCmd)
	updateCmd.AddCommand(updateZvolCmd)

	return parent.Execute()
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
	return Pool{}, errors.Errorf("pool %s not found", name)
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
	return nil, errors.Errorf("%s", string(r.Body))
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
	return nil, errors.Errorf("%s", string(r.Body))
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
	return nil, errors.Errorf("%s", r.Status())
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

func (t Array) GetISCSITargets() ([]ISCSITarget, error) {
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

func (t Array) dumpISCSITargetExtents() error {
	data, err := t.GetISCSITargetExtents()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSITargetExtents() ([]ISCSITargetExtent, error) {
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

func (t Array) GetISCSIExtents() ([]ISCSIExtent, error) {
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

func (t Array) GetISCSIInitiators() ([]ISCSIInitiator, error) {
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

func (t Array) dumpDatasets() error {
	data, err := t.GetDatasets()
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDatasets() ([]Dataset, error) {
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

func (t Array) client() (*ClientWithResponses, error) {
	api, err := t.api()
	if err != nil {
		return nil, errors.Wrap(err, "load api url")
	}
	username := t.username()
	password, err := t.password()
	if err != nil {
		return nil, errors.Wrap(err, "load password")
	}
	basicAuthProvider, err := securityprovider.NewSecurityProviderBasicAuth(username, password)
	if err != nil {
		return nil, errors.Wrap(err, "new openapi basic auth security provider")
	}
	client, err := NewClientWithResponses(api.String(), WithRequestEditorFn(basicAuthProvider.Intercept))
	if err != nil {
		return nil, errors.Wrap(err, "new open api client")
	}
	return client, nil
}
