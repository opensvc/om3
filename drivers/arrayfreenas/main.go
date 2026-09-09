package arrayfreenas

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/uri"
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
		Name     string
		Mappings []string
	}

	MapDiskOptions struct {
		Name     string
		Mappings []string
		LunId    *int
	}

	DelISCSIExtentOptions struct {
		Id   int
		Name string
	}

	AddDiskOptions struct {
		AddZvolOptions
		InsecureTPC bool
		Mappings    []string
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

// Run builds the command tree of this array and runs the arguments through
// it. What the tree holds is declared in Actions.
func (t *Array) Run(args []string) error {
	return array.RunActions(context.Background(), t.Actions(), args, os.Stdout)
}
func (t Array) DelZvol(ctx context.Context, name string) (*Dataset, error) {
	dataset, err := t.GetDataset(ctx, name)
	if err != nil {
		return nil, err
	}
	if dataset == nil {
		return nil, fmt.Errorf("dataset not found")
	}
	err = t.delZvolById(ctx, dataset.Id)
	return dataset, err
}

func (t Array) delZvolById(ctx context.Context, id string) error {
	path := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(id))
	req, err := t.newRequest(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	var data any
	_, err = t.Do(req, &data)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	return nil
}

func (t Array) delISCSIExtent(ctx context.Context, extent ISCSIExtent) error {
	path := fmt.Sprintf("/iscsi/extent/id/%d", extent.Id)
	req, err := t.newRequest(ctx, http.MethodDelete, path, nil, nil)
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

func (t Array) DelISCSIExtent(ctx context.Context, opt DelISCSIExtentOptions) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return nil, err
	}
	var extent *ISCSIExtent
	if opt.Id >= 0 {
		extent = extents.GetById(opt.Id)
	} else if opt.Name != "" {
		extent = extents.GetByName(opt.Name)
	}
	if extent == nil {
		return nil, fmt.Errorf("extent %#v not found (%d scanned)", opt, len(extents))
	}
	return extent, t.delISCSIExtent(ctx, *extent)
}

func (t Array) AddISCSIExtent(ctx context.Context, opt AddISCSIExtentOptions) (*ISCSIExtent, error) {
	extent, err := t.GetISCSIExtent(ctx, opt.Name)
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
	return t.createISCSIExtent(ctx, params)
}

func (t Array) AddZvol(ctx context.Context, opt AddZvolOptions) (*Dataset, error) {
	params, err := opt.Params()
	if err != nil {
		return nil, err
	}
	params.Type = &DatasetTypeVolume
	dataset, err := t.GetDataset(ctx, params.Name)
	if err != nil {
		return nil, err
	}
	if dataset != nil {
		return dataset, nil
	}
	return t.CreateDataset(ctx, params)
}

func (t Array) DelDisk(ctx context.Context, name string) (*Disk, error) {
	disk, err := t.GetDisk(ctx, name)
	if err != nil {
		return nil, err
	}
	if disk != nil {
		if disk.ISCSI != nil && disk.ISCSI.Extent != nil {
			if _, err := t.DelISCSIExtent(ctx, DelISCSIExtentOptions{Id: disk.ISCSI.Extent.Id}); err != nil {
				return disk, err
			}
		}
		if disk.Dataset != nil {
			if err := t.delZvolById(ctx, disk.Dataset.Id); err != nil {
				return disk, err
			}
		}
	}
	return disk, nil
}

func (t Array) AddDisk(ctx context.Context, opt AddDiskOptions) (*Disk, error) {
	disk := Disk{
		ISCSI: &DiskISCSI{},
	}
	if data, err := t.AddZvol(ctx, opt.AddZvolOptions); err != nil {
		return nil, err
	} else {
		disk.Dataset = data
	}

	// Extent
	extent, err := t.AddISCSIExtent(ctx, AddISCSIExtentOptions{
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
	targetExtent, err := t.MapDisk(ctx, MapDiskOptions{
		Name:     opt.Name,
		Mappings: opt.Mappings,
		LunId:    opt.LunId,
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

func (t Array) password() (string, error) {
	var km datarecv.KeyMeta
	s, err := t.Config().GetStringStrict(t.Key("password"))
	if err != nil {
		return "", err
	}
	// Parse key reference with backward compatibility
	// New format: password = from system/sec/array1 key password
	// Old format: password = system/sec/array1 (uses default key "password")
	km, err = datarecv.ParseKeyMetaRelWithFallback(s, naming.NsSys, "password")
	if err != nil {
		return "", err
	}
	b, err := km.RootDecode()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (t Array) api() string {
	return t.Config().GetString(t.Key("api"))
}

func (t Array) GetPoolByName(ctx context.Context, name string) (Pool, error) {
	pools, err := t.GetPools(ctx)
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

func (t Array) dumpPools(ctx context.Context) error {
	data, err := t.GetPools(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) UpdateDataset(ctx context.Context, id string, params UpdateDatasetParams) (*Dataset, error) {
	path := fmt.Sprintf("/pool/dataset/id/%s", id)
	req, err := t.newRequest(ctx, http.MethodPut, path, nil, params)
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

func (t Array) CreateDataset(ctx context.Context, params CreateDatasetParams) (*Dataset, error) {
	path := fmt.Sprintf("/pool/dataset")
	req, err := t.newRequest(ctx, http.MethodPost, path, nil, params)
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

func (t Array) DeleteDataset(ctx context.Context, name string) (*Dataset, error) {
	dataset, err := t.GetDataset(ctx, name)
	if err != nil {
		return nil, err
	}
	if dataset == nil {
		return nil, fmt.Errorf("dataset %s does not exist", name)
	}
	path := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(dataset.Id))
	req, err := t.newRequest(ctx, http.MethodDelete, path, nil, nil)
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

func (t Array) GetPools(ctx context.Context) ([]Pool, error) {
	path := fmt.Sprintf("/pool")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpISCSIPortals(ctx context.Context) error {
	data, err := t.GetISCSIPortals(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIPortals(ctx context.Context) ([]any, error) {
	path := fmt.Sprintf("/iscsi/portal")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpISCSITargets(ctx context.Context) error {
	data, err := t.GetISCSITargets(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t *Array) Do(req *http.Request, v interface{}) (*http.Response, error) {
	cli, err := t.safeClient(req.URL)
	if err != nil {
		return nil, fmt.Errorf("safe client: %w", err)
	}
	var resp *http.Response
	resp, err = cli.Do(req)
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

func (t Array) GetISCSITargets(ctx context.Context) (ISCSITargets, error) {
	path := fmt.Sprintf("/iscsi/target")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) GetISCSIExtent(ctx context.Context, name string) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return nil, err
	}
	return extents.GetByName(name), nil
}

func (t Array) dumpISCSIExtent(ctx context.Context, name string) error {
	data, err := t.GetISCSIExtent(ctx, name)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) dumpISCSITargetExtents(ctx context.Context) error {
	data, err := t.GetISCSITargetExtents(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSITargetExtents(ctx context.Context) (ISCSITargetExtents, error) {
	path := fmt.Sprintf("/iscsi/targetextent")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpISCSIExtents(ctx context.Context) error {
	data, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIExtents(ctx context.Context) (ISCSIExtents, error) {
	path := fmt.Sprintf("/iscsi/extent")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpISCSIInitiators(ctx context.Context) error {
	data, err := t.GetISCSIInitiators(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetISCSIInitiators(ctx context.Context) (ISCSIInitiators, error) {
	path := fmt.Sprintf("/iscsi/initiator")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpSystemInfo(ctx context.Context) error {
	data, err := t.GetSystemInfo(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	path := fmt.Sprintf("/system/info")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpDisk(ctx context.Context, name string) error {
	data, err := t.GetDisk(ctx, name)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDisk(ctx context.Context, name string) (*Disk, error) {
	disk := Disk{
		ISCSI: &DiskISCSI{},
	}

	dataset, err := t.GetDataset(ctx, name)
	if err != nil {
		return nil, err
	}
	if dataset == nil {
		return nil, nil
	}
	disk.Dataset = dataset

	extents, err := t.GetISCSIExtents(ctx)
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
		if targetExtents, err := t.GetISCSITargetExtents(ctx); err != nil {
			return nil, err
		} else {
			disk.ISCSI.TargetExtents = targetExtents.WithExtent(extent)
		}
	}
	return &disk, nil
}

func (t Array) dumpDatasets(ctx context.Context) error {
	data, err := t.GetDatasets(ctx)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDatasets(ctx context.Context) (Datasets, error) {
	path := fmt.Sprintf("/pool/dataset")
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) dumpDataset(ctx context.Context, name string) error {
	data, err := t.GetDataset(ctx, name)
	if err != nil {
		return err
	}
	return dump(data)
}

func (t Array) GetDataset(ctx context.Context, name string) (*Dataset, error) {
	path := fmt.Sprintf("/pool/dataset")
	params := map[string]string{
		"name": name,
	}
	req, err := t.newRequest(ctx, http.MethodGet, path, params, nil)
	if err != nil {
		return nil, err
	}
	var data Datasets
	_, err = t.Do(req, &data)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return &data[0], nil
}

func (t Array) UnmapDisk(ctx context.Context, opt UnmapDiskOptions) (ISCSITargetExtents, error) {
	deletedTargetExtents := make(ISCSITargetExtents, 0)
	paths, err := san.ParseMappings(opt.Mappings)
	if err != nil {
		return deletedTargetExtents, err
	} else if len(paths) == 0 {
		return deletedTargetExtents, nil
	}
	targets, err := t.GetISCSITargets(ctx)
	if err != nil {
		return deletedTargetExtents, err
	}
	extents, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return deletedTargetExtents, err
	}
	targetextents, err := t.GetISCSITargetExtents(ctx)
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
		if err := t.delISCSITargetExtent(ctx, filteredTargetextent.Id); err != nil {
			return deletedTargetExtents, err
		}
		deletedTargetExtents = append(deletedTargetExtents, filteredTargetextent)
	}
	return deletedTargetExtents, nil
}

func (t Array) MapDisk(ctx context.Context, opt MapDiskOptions) (ISCSITargetExtents, error) {
	missingTargetExtents := make(ISCSITargetExtents, 0)
	paths, err := san.ParseMappings(opt.Mappings)
	if err != nil {
		return missingTargetExtents, err
	} else if len(paths) == 0 {
		return missingTargetExtents, nil
	}
	targets, err := t.GetISCSITargets(ctx)
	if err != nil {
		return missingTargetExtents, err
	}
	extents, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return missingTargetExtents, err
	}
	targetextents, err := t.GetISCSITargetExtents(ctx)
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
		d, err := t.createISCSITargetExtent(ctx, params)
		if err != nil {
			return missingTargetExtents, err
		}
		missingTargetExtents = append(missingTargetExtents, *d)
	}
	return missingTargetExtents, nil
}

func (t Array) createISCSITargetExtent(ctx context.Context, params CreateISCSITargetExtentParams) (*ISCSITargetExtent, error) {
	path := fmt.Sprintf("/iscsi/targetextent")
	req, err := t.newRequest(ctx, http.MethodPost, path, nil, params)
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

func (t Array) createISCSIExtent(ctx context.Context, params CreateISCSIExtentParams) (*ISCSIExtent, error) {
	path := fmt.Sprintf("/iscsi/extent")
	req, err := t.newRequest(ctx, http.MethodPost, path, nil, params)
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

func (t Array) getISCSITarget(ctx context.Context, id int) (*ISCSITarget, error) {
	path := fmt.Sprintf("/iscsi/target/id/%d", id)
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) delISCSITargetExtent(ctx context.Context, id int) error {
	path := fmt.Sprintf("/iscsi/targetextent/id/%d", id)
	// true as a body payload forces the delete of a in-use target extent
	req, err := t.newRequest(ctx, http.MethodDelete, path, nil, true)
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

func (t Array) delISCSITarget(ctx context.Context, id int) (*ISCSITarget, error) {
	target, err := t.getISCSITarget(ctx, id)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/iscsi/target/id/%d", id)
	req, err := t.newRequest(ctx, http.MethodDelete, path, nil, nil)
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

func (t Array) getISCSIInitiator(ctx context.Context, id int) (*ISCSIInitiator, error) {
	path := fmt.Sprintf("/iscsi/initiator/id/%d", id)
	req, err := t.newRequest(ctx, http.MethodGet, path, nil, nil)
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

func (t Array) delISCSIInitiator(ctx context.Context, id int) (*ISCSIInitiator, error) {
	initiator, err := t.getISCSIInitiator(ctx, id)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/iscsi/initiator/id/%d", id)
	req, err := t.newRequest(ctx, http.MethodDelete, path, nil, nil)
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

func (t Array) addISCSIInitiator(ctx context.Context, params CreateISCSIInitiatorParams) (*ISCSIInitiator, error) {
	path := fmt.Sprintf("/iscsi/initiator")
	req, err := t.newRequest(ctx, http.MethodPost, path, nil, params)
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

func (t Array) addISCSITargetGroup(ctx context.Context, opt AddISCSITargetGroupOptions) (ISCSITargets, error) {
	initiators := make(ISCSIInitiators, 0)
	if opt.InitiatorId >= 0 {
		initiator, err := t.getISCSIInitiator(ctx, opt.InitiatorId)
		if err != nil {
			return nil, err
		}
		if initiator == nil {
			return nil, fmt.Errorf("initiator id %d not found", opt.InitiatorId)
		}
		initiators = append(initiators, *initiator)
	} else if opt.InitiatorName != "" {
		l, err := t.GetISCSIInitiators(ctx)
		if err != nil {
			return nil, err
		}
		initiators = l.WithName(opt.InitiatorName)
	}

	targets := make(ISCSITargets, 0)
	if opt.Target != "" {
		l, err := t.GetISCSITargets(ctx)
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
		req, err := t.newRequest(ctx, http.MethodPut, path, nil, params)
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

func (t Array) addISCSITarget(ctx context.Context, params CreateISCSITargetParams) (*ISCSITarget, error) {
	path := fmt.Sprintf("/iscsi/target")
	req, err := t.newRequest(ctx, http.MethodPost, path, nil, params)
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

func (t Array) addISCSIPortal(ctx context.Context, params CreateISCSIPortalParams) (*ISCSIPortal, error) {
	path := fmt.Sprintf("/iscsi/portal")
	req, err := t.newRequest(ctx, http.MethodPost, path, nil, params)
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

func (t *Array) safeClient(u *url.URL) (*http.Client, error) {
	c, err := uri.SafeHttpClient(u.String())
	if err != nil {
		return nil, err
	}
	c.Timeout = t.timeout()
	if u.Scheme == "https" {
		if transport, ok := c.Transport.(*http.Transport); ok {
			transport.TLSClientConfig.InsecureSkipVerify = t.insecure()
			c.Transport = transport
		}
	}
	return c, nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (t *Array) newRequest(ctx context.Context, method string, path string, params map[string]string, data interface{}) (*http.Request, error) {
	fpath := t.api() + Head + path
	// Verify URL
	if _, _, err := uri.CheckHttpUrl(fpath); err != nil {
		return nil, err
	}
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
	req, err := http.NewRequestWithContext(ctx, method, baseURL.String(), nil)
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
func (t Array) DiskPaths(ctx context.Context, disk Disk) (san.Paths, error) {
	paths := san.Paths{}
	targets, err := t.GetISCSITargets(ctx)
	if err != nil {
		return paths, err
	}
	initiators, err := t.GetISCSIInitiators(ctx)
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
