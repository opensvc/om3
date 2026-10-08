package arrayfreenas

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/uri"
)

var (
	// listParams are the parameters of a listing. The api pages a listing
	// unless told not to, and an item past the first page is not there to
	// the code looking for it: an extent not found is a name free to take.
	listParams = map[string]string{"limit": "0"}

	Head                  = "/api/v2.0"
	DatasetTypeVolume     = "VOLUME"
	DatasetTypeFilesystem = "FILESYSTEM"
	RequestTimeout        = 10 * time.Second
)

type (
	Array struct {
		*array.Array

		// secret, when set, replaces the decoding of the configured
		// password secret, which needs a cluster to hold it: the tests
		// talk to a fake array, with no cluster around.
		secret func() (string, error)
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

		// ExtentName is the name of the iscsi extent exporting the zvol.
		// Left empty, the extent is named as the zvol is.
		ExtentName  string
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
		Extent        *ISCSIExtent       `json:"extent,omitempty"`
		TargetExtents ISCSITargetExtents `json:"targetextents,omitempty"`
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

// DelZvol deletes a zvol no extent exports. A zvol an extent exports is a
// disk, which "del disk" deletes with its extent and its mappings: deleting
// the zvol alone pulls the storage from under initiators still reaching it.
func (t Array) DelZvol(ctx context.Context, name string) (*Dataset, error) {
	ref, err := t.resolveZvol(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(ref.extents) > 0 {
		return nil, fmt.Errorf("zvol %s is exported by the iscsi extent(s) %s: delete the disk instead", ref.dataset.Name, ref.extents.Names())
	}
	if err := t.delZvolById(ctx, ref.dataset.Id); err != nil {
		return nil, err
	}
	return &ref.dataset, nil
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

// AddISCSIExtent creates an extent. An extent of that name, or exporting
// that disk, is refused rather than returned: it may be another client's,
// and the caller is about to map it.
func (t Array) AddISCSIExtent(ctx context.Context, opt AddISCSIExtentOptions) (*ISCSIExtent, error) {
	extents, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return nil, err
	}
	if err := extents.checkFree(opt.Name, opt.Disk); err != nil {
		return nil, err
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

// AddZvol creates a zvol. A dataset of that name is refused rather than
// returned: it holds data of its own, maybe another client's, and is not of
// the size asked for.
func (t Array) AddZvol(ctx context.Context, opt AddZvolOptions) (*Dataset, error) {
	params, err := opt.Params()
	if err != nil {
		return nil, err
	}
	if err := t.checkNoDataset(ctx, params.Name); err != nil {
		return nil, err
	}
	return t.CreateDataset(ctx, params)
}

// checkNoDataset returns an error when a dataset is named name.
func (t Array) checkNoDataset(ctx context.Context, name string) error {
	dataset, err := t.GetDataset(ctx, name)
	if err != nil {
		return err
	}
	if dataset != nil {
		return fmt.Errorf("dataset %s already exists (type %s): refusing to export storage this command did not create", name, dataset.Type)
	}
	return nil
}

// DelDisk unexports a zvol and deletes it, in the order v2 does: the
// targetextents, the extent, then the zvol. Each step deletes only what the
// one before left unreachable, so a failure midway leaves no exported disk
// without storage behind it.
func (t Array) DelDisk(ctx context.Context, name string) (*Disk, error) {
	disk, err := t.GetDisk(ctx, name)
	if err != nil {
		return nil, err
	}
	var done []string
	fail := func(err error) (*Disk, error) {
		if len(done) == 0 {
			return disk, err
		}
		return disk, fmt.Errorf("%w; already deleted: %s", err, strings.Join(done, ", "))
	}
	if extent := disk.ISCSI.Extent; extent != nil {
		for _, targetExtent := range disk.ISCSI.TargetExtents {
			if err := t.delISCSITargetExtent(ctx, targetExtent.Id); err != nil {
				return fail(fmt.Errorf("delete targetextent %d of extent %d: %w", targetExtent.Id, extent.Id, err))
			}
			done = append(done, fmt.Sprintf("targetextent %d", targetExtent.Id))
		}
		if err := t.delISCSIExtent(ctx, *extent); err != nil {
			return fail(fmt.Errorf("delete extent %d (%s): %w", extent.Id, extent.Name, err))
		}
		done = append(done, fmt.Sprintf("extent %d (%s)", extent.Id, extent.Name))
	}
	if err := t.delZvolById(ctx, disk.Dataset.Id); err != nil {
		return fail(fmt.Errorf("delete zvol %s: %w", disk.Dataset.Name, err))
	}
	return disk, nil
}

// leftovers names what an add made on the array before failing.
//
// The add does not delete them: a cleanup guessing what a failure left is
// the way to delete what it did not make, so the error names them for the
// operator to judge.
type leftovers []string

func (t leftovers) wrap(err error) error {
	if len(t) == 0 {
		return err
	}
	return fmt.Errorf("%w; made on the array and left in place: %s", err, strings.Join(t, ", "))
}

// madeAnyway returns t with what a write that answered an error may have
// made nonetheless: an error, as a timeout, does not say the array did not
// act.
//
// Found after the error, the object is only maybe this command's: another
// one can have made it since the check that it did not exist, as when the
// write failed because it did, and naming it as made would invite the
// operator to delete another client's disk.
func (t leftovers) madeAnyway(what string, check func() (bool, error)) leftovers {
	if ok, err := check(); err != nil {
		return append(t, fmt.Sprintf("maybe %s (could not check: %s)", what, err))
	} else if ok {
		return append(t, fmt.Sprintf("maybe %s (it exists after the error: this command or another one made it)", what))
	}
	return t
}

// AddDisk creates a zvol, the extent exporting it and the targetextents
// mapping it.
//
// What can be refused without writing is checked before the first write:
// the size, the targets of the mappings, and that neither the zvol nor its
// extent exist. A failure after a write returns an error naming what was
// made.
func (t Array) AddDisk(ctx context.Context, opt AddDiskOptions) (*Disk, error) {
	params, err := opt.AddZvolOptions.Params()
	if err != nil {
		return nil, err
	}
	extentName := opt.ExtentName
	if extentName == "" {
		extentName = opt.Name
	}
	extentDisk := "zvol/" + opt.Name
	extentParams := CreateISCSIExtentParams{
		Name:        extentName,
		Disk:        extentDisk,
		Type:        "DISK",
		InsecureTPC: opt.InsecureTPC,
	}
	if i, err := sizeconv.FromSize(opt.Blocksize); err != nil {
		return nil, fmt.Errorf("blocksize: %w", err)
	} else {
		extentParams.Blocksize = int(i)
	}
	targets, err := t.mappedTargets(ctx, opt.Mappings)
	if err != nil {
		return nil, err
	}
	if err := t.checkNoDataset(ctx, opt.Name); err != nil {
		return nil, err
	}
	if extents, err := t.GetISCSIExtents(ctx); err != nil {
		return nil, err
	} else if err := extents.checkFree(extentName, extentDisk); err != nil {
		return nil, err
	}

	var made leftovers
	disk := Disk{
		ISCSI: &DiskISCSI{},
	}
	dataset, err := t.CreateDataset(ctx, params)
	if err != nil {
		made = made.madeAnyway("zvol "+opt.Name, func() (bool, error) {
			ds, err := t.GetDataset(ctx, opt.Name)
			return ds != nil, err
		})
		return nil, made.wrap(fmt.Errorf("create zvol %s: %w", opt.Name, err))
	}
	made = append(made, "zvol "+opt.Name)
	disk.Dataset = dataset

	extent, err := t.createISCSIExtent(ctx, extentParams)
	if err != nil {
		made = made.madeAnyway("extent "+extentName, func() (bool, error) {
			extents, err := t.GetISCSIExtents(ctx)
			return extents.GetByName(extentName) != nil, err
		})
		return nil, made.wrap(fmt.Errorf("create extent %s: %w", extentName, err))
	}
	if extent.Id == 0 {
		// The id is what the targetextents are created with: guessing it
		// from a listing maps whatever extent the guess finds.
		return nil, made.wrap(fmt.Errorf("create extent %s: the array answered no extent id", extentName))
	}
	made = append(made, fmt.Sprintf("extent %d (%s)", extent.Id, extentName))
	disk.ISCSI.Extent = extent

	targetExtents, err := t.mapExtent(ctx, *extent, targets, opt.LunId)
	for _, targetExtent := range targetExtents {
		made = append(made, fmt.Sprintf("targetextent %d", targetExtent.Id))
	}
	if err != nil {
		return nil, made.wrap(err)
	}
	disk.ISCSI.TargetExtents = targetExtents
	return &disk, nil
}

// mapExtent attaches the extent to each target, and returns the
// targetextents it created. On error, they are the ones created before it,
// and the one that failed when the array made it nonetheless.
func (t Array) mapExtent(ctx context.Context, extent ISCSIExtent, targets ISCSITargets, lunId *int) (ISCSITargetExtents, error) {
	created := make(ISCSITargetExtents, 0, len(targets))
	for _, target := range targets {
		params := CreateISCSITargetExtentParams{
			Target: target.Id,
			Extent: extent.Id,
			LunId:  lunId,
		}
		d, err := t.createISCSITargetExtent(ctx, params)
		if err != nil {
			err = fmt.Errorf("attach extent %d to target %d (%s): %w", extent.Id, target.Id, target.Name, err)
			if l, lerr := t.GetISCSITargetExtents(ctx); lerr != nil {
				err = fmt.Errorf("%w; could not check whether the array attached it nonetheless: %s", err, lerr)
			} else {
				created = append(created, l.WithExtent(extent).WithTarget(target)...)
			}
			return created, err
		}
		created = append(created, *d)
	}
	return created, nil
}

// mappedTargets returns the targets the mappings name, each once: the
// initiators of the nodes of a cluster reach a disk through the same
// targets, and an extent is attached to a target once. A target the array
// does not have is an error, returned before anything is made.
func (t Array) mappedTargets(ctx context.Context, mappings []string) (ISCSITargets, error) {
	paths, err := san.ParseMappings(mappings)
	if err != nil {
		return nil, err
	}
	targets := make(ISCSITargets, 0)
	if len(paths) == 0 {
		return targets, nil
	}
	all, err := t.GetISCSITargets(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, p := range paths {
		if seen[p.Target.Name] {
			continue
		}
		seen[p.Target.Name] = true
		target, ok := all.GetByName(p.Target.Name)
		if !ok {
			return nil, fmt.Errorf("target %s not found (%d scanned)", p.Target.Name, len(all))
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// zvolRef is a zvol named on a command line, and the extents exporting it.
type zvolRef struct {
	dataset Dataset
	extents ISCSIExtents
}

// resolveZvol returns the zvol a command line names, the way v2 reads the
// name: the name of the extent exporting it, or its naa, else the zvol
// name itself. The collector knows a disk by the name of its extent, which
// is not the name of its zvol.
//
// A name naming nothing is an error: the collector forgets a disk a delete
// says it deleted.
func (t Array) resolveZvol(ctx context.Context, name string) (zvolRef, error) {
	var ref zvolRef
	if name == "" {
		return ref, fmt.Errorf("a name is required")
	}
	extents, err := t.GetISCSIExtents(ctx)
	if err != nil {
		return ref, err
	}
	zvolName := name
	extent := extents.GetByName(name)
	if extent == nil {
		extent = extents.GetByNAA(name)
	}
	if extent != nil {
		if s, ok := extent.zvol(); !ok {
			return ref, fmt.Errorf("extent %d (%s) is a %s extent of %q, not a zvol", extent.Id, extent.Name, extent.Type, extent.diskPath())
		} else {
			zvolName = s
		}
	}
	dataset, err := t.GetDataset(ctx, zvolName)
	if err != nil {
		return ref, err
	}
	switch {
	case dataset == nil && extent != nil:
		return ref, fmt.Errorf("extent %d (%s) exports zvol %s, which does not exist", extent.Id, extent.Name, zvolName)
	case dataset == nil:
		return ref, fmt.Errorf("no extent named %s or of this naa, and no dataset named %s", name, name)
	case dataset.Type != DatasetTypeVolume:
		return ref, fmt.Errorf("dataset %s is a %s, not a zvol", dataset.Name, dataset.Type)
	}
	ref.dataset = *dataset
	ref.extents = extents.WithZvol(zvolName)
	return ref, nil
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
	if t.secret != nil {
		return t.secret()
	}
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
	path := fmt.Sprintf("/pool/dataset/id/%s", url.PathEscape(id))
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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

// GetDisk returns the zvol a command line names, the extent exporting it and
// the targetextents mapping it. See resolveZvol for how a name is read.
//
// A zvol exported by more than one extent is an error: which one a command
// is about is a guess, and a delete deleting the one guessed leaves the
// others exporting a zvol gone.
func (t Array) GetDisk(ctx context.Context, name string) (*Disk, error) {
	ref, err := t.resolveZvol(ctx, name)
	if err != nil {
		return nil, err
	}
	disk := Disk{
		Dataset: &ref.dataset,
		ISCSI:   &DiskISCSI{},
	}
	switch len(ref.extents) {
	case 0:
	case 1:
		extent := ref.extents[0]
		disk.ISCSI.Extent = &extent
		if targetExtents, err := t.GetISCSITargetExtents(ctx); err != nil {
			return nil, err
		} else {
			disk.ISCSI.TargetExtents = targetExtents.WithExtent(extent)
		}
	default:
		return nil, fmt.Errorf("zvol %s is exported by %d iscsi extents: %s", ref.dataset.Name, len(ref.extents), ref.extents.Names())
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
	req, err := t.newRequest(ctx, http.MethodGet, path, listParams, nil)
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
		"name":  name,
		"limit": "0",
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
	// The name is a filter the api may not apply, and the first dataset of
	// an unfiltered listing is not the one asked for.
	dataset, _ := data.GetByName(name)
	return dataset, nil
}

// exportedDisk returns the disk a command line names, which must be exported
// by an extent.
func (t Array) exportedDisk(ctx context.Context, name string) (*Disk, error) {
	disk, err := t.GetDisk(ctx, name)
	if err != nil {
		return nil, err
	}
	if disk.ISCSI.Extent == nil {
		return nil, fmt.Errorf("zvol %s is exported by no iscsi extent", disk.Dataset.Name)
	}
	return disk, nil
}

// UnmapDisk detaches the extent of a disk from the targets of the mappings,
// and returns the targetextents it deleted.
//
// A targetextent is the disk on a target for every initiator the target
// admits, not for the initiator of a mapping alone: a mapping naming an
// initiator the target does not admit is refused, as deleting the
// targetextent would cut the disk from the hosts that do reach it there. A
// target unknown to the array is refused too, rather than skipped as done.
// Every mapping is checked before the first targetextent is deleted.
func (t Array) UnmapDisk(ctx context.Context, opt UnmapDiskOptions) (ISCSITargetExtents, error) {
	deletedTargetExtents := make(ISCSITargetExtents, 0)
	paths, err := san.ParseMappings(opt.Mappings)
	if err != nil {
		return deletedTargetExtents, err
	} else if len(paths) == 0 {
		return deletedTargetExtents, fmt.Errorf("no mapping: --mappings names no <hba>:<tgt> path")
	}
	disk, err := t.exportedDisk(ctx, opt.Name)
	if err != nil {
		return deletedTargetExtents, err
	}
	extent := *disk.ISCSI.Extent
	targets, err := t.GetISCSITargets(ctx)
	if err != nil {
		return deletedTargetExtents, err
	}
	initiators, err := t.GetISCSIInitiators(ctx)
	if err != nil {
		return deletedTargetExtents, err
	}
	toDelete := make(ISCSITargetExtents, 0)
	seen := make(map[string]bool)
	for _, p := range paths {
		target, ok := targets.GetByName(p.Target.Name)
		if !ok {
			return deletedTargetExtents, fmt.Errorf("target %s not found", p.Target.Name)
		}
		if ok, err := targetAdmits(target, p.Initiator.Name, initiators); err != nil {
			return deletedTargetExtents, err
		} else if !ok {
			return deletedTargetExtents, fmt.Errorf("target %s does not admit initiator %s: detaching extent %d from it would cut the disk from the hosts it admits", target.Name, p.Initiator.Name, extent.Id)
		}
		if seen[p.Target.Name] {
			continue
		}
		seen[p.Target.Name] = true
		filteredTargetextents := disk.ISCSI.TargetExtents.WithTarget(target)
		if len(filteredTargetextents) == 0 {
			// Already detached.
			continue
		} else if len(filteredTargetextents) > 1 {
			return deletedTargetExtents, fmt.Errorf("too many (%d) target extents for extent %d and target %s", len(filteredTargetextents), extent.Id, target.Name)
		}
		toDelete = append(toDelete, filteredTargetextents[0])
	}
	for _, targetExtent := range toDelete {
		if err := t.delISCSITargetExtent(ctx, targetExtent.Id); err != nil {
			return deletedTargetExtents, fmt.Errorf("delete targetextent %d, after deleting %d of %d: %w", targetExtent.Id, len(deletedTargetExtents), len(toDelete), err)
		}
		deletedTargetExtents = append(deletedTargetExtents, targetExtent)
	}
	return deletedTargetExtents, nil
}

// MapDisk attaches the extent of a disk to the targets of the mappings it is
// not attached to yet, and returns the targetextents of these targets. Every
// target is found before the first is attached.
func (t Array) MapDisk(ctx context.Context, opt MapDiskOptions) (ISCSITargetExtents, error) {
	targetExtents := make(ISCSITargetExtents, 0)
	targets, err := t.mappedTargets(ctx, opt.Mappings)
	if err != nil {
		return targetExtents, err
	} else if len(targets) == 0 {
		return targetExtents, nil
	}
	disk, err := t.exportedDisk(ctx, opt.Name)
	if err != nil {
		return targetExtents, err
	}
	missing := make(ISCSITargets, 0)
	for _, target := range targets {
		if l := disk.ISCSI.TargetExtents.WithTarget(target); len(l) > 0 {
			targetExtents = append(targetExtents, l...)
		} else {
			missing = append(missing, target)
		}
	}
	created, err := t.mapExtent(ctx, *disk.ISCSI.Extent, missing, opt.LunId)
	targetExtents = append(targetExtents, created...)
	if err != nil {
		var made leftovers
		for _, targetExtent := range created {
			made = append(made, fmt.Sprintf("targetextent %d", targetExtent.Id))
		}
		return targetExtents, made.wrap(err)
	}
	return targetExtents, nil
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
			// The transport of the ssrf-safe client carries no tls
			// configuration: it sets a DialContext and nothing else. Writing
			// InsecureSkipVerify through the nil one panicked, which is every
			// call to an https array api.
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}
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
		req, err = http.NewRequestWithContext(ctx, method, baseURL.String(), bytes.NewBuffer(jsonString))
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
	// A zvol no extent exports has no naa, and a delete of one returns it.
	if disk.ISCSI == nil || disk.ISCSI.Extent == nil {
		return ""
	}
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

	// The volblocksize is left to the array, as v2 leaves it. The
	// blocksize option is the one the extent exports, and a zvol of 512 B
	// blocks is a zvol of eight times the metadata.
	params := CreateDatasetParams{
		Name:          t.Name,
		Type:          &DatasetTypeVolume,
		Sparse:        &t.Sparse,
		Deduplication: &dedupParam,
	}
	if t.Name == "" {
		return params, fmt.Errorf("a zvol name is required")
	}
	if compressionParam != "INHERIT" {
		params.Compression = &compressionParam
	}
	size, err := array.ParseSize(t.Size)
	if err != nil {
		return params, err
	}
	if size.Relative {
		return params, fmt.Errorf("size %s: a new zvol has no size to grow from", t.Size)
	}
	params.Volsize = &size.Bytes
	return params, nil
}
