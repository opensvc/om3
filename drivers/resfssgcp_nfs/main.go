// Package resfssgcp_nfs provides the SGCP NFS filesystem driver
package resfssgcp_nfs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resfshost"
	"github.com/opensvc/om3/v3/drivers/sgcphelper"
	"github.com/opensvc/om3/v3/util/httpclientcache"
	"github.com/opensvc/om3/v3/util/sgcp"
)

type (
	// NfsClient represents an NFS client configuration
	NfsClient struct {
		UUID               string `json:"uuid"`
		Host               string `json:"host"`
		Permission         string `json:"permission"`
		Protocol           string `json:"protocol"`
		ConsistencyGroupID string `json:"consistencyGroupId,omitempty"`
	}

	// FilesystemInfo represents the filesystem information from the API
	FilesystemInfo struct {
		UUID               string      `json:"uuid"`
		ConsistencyGroupID string      `json:"consistencyGroupId"`
		NFSClients         []NfsClient `json:"nfsClients"`
		Status             string      `json:"status"`
	}

	// T represents the SGCP NFS filesystem resource
	T struct {
		resource.T
		resource.Restart
		datarecv.DataRecv

		// Configuration parameters
		UUID       string `json:"uuid"`
		Host       string `json:"host,omitempty"`
		Permission string `json:"permission,omitempty"`
		Exclusive  bool   `json:"exclusive,omitempty"`
		Protocol   string `json:"protocol,omitempty"`
		Secret     string `json:"secret,omitempty"`
		Endpoint   string `json:"endpoint,omitempty"`
		Path       string `json:"path,omitempty"`

		// Configuration parameters for the underlying filesystem resource
		MountPoint   string         `json:"mnt"`
		Device       string         `json:"dev"`
		Type         string         `json:"type"`
		MountOptions string         `json:"mnt_opt"`
		StatTimeout  *time.Duration `json:"stat_timeout"`
		StartTimeout *time.Duration `json:"start_timeout"`
		Zone         string         `json:"zone"`
		CheckRead    bool           `json:"check_read"`

		// Internal state
		resFs         fsDriver
		fileInfoCache *FilesystemInfo
		mgr           *nfsClientMgr
		authInfoer    GetAuthInfoer
	}

	GetAuthInfoer interface {
		GetAuthInfo(string) (*sgcp.AuthInfo, error)
	}

	fsDriver interface {
		Start(context.Context) error
		Stop(context.Context) error
		Status(context.Context) status.T
		StatusLog() resource.StatusLogger
		Label(context.Context) string
		Head() string
		CanInstall(context.Context) (bool, error)
	}
)

// NfsClientIgnored is a list of NFS client hosts to ignore
var NfsClientIgnored = []string{}

// New creates a new SGCP NFS filesystem resource driver
func New() resource.Driver {
	return &T{}
}

// Configure sets up the resource
func (t *T) Configure() error {
	cfg := sgcp.GetConfig()

	if cfg == nil {
		return fmt.Errorf("mandatory config file is required: %s", sgcp.DefaultConfigPath)
	}

	// set defaults when kw are not set
	if t.Secret == "" {
		t.Secret = cfg.Auth.DefaultSecret
	}
	if t.Secret == "" {
		return fmt.Errorf("secret is required (neither defined into secret keyword nor config file %s", sgcp.DefaultConfigPath)
	} else {
		cfg = cfg.WithAuthSecret(t.Secret)
	}

	if t.Endpoint == "" {
		t.Endpoint = cfg.Files.BaseURL
	}
	if t.Endpoint == "" {
		return fmt.Errorf("file endpoint is required (neither defined into endpoint keyword nor config file %s", sgcp.DefaultConfigPath)
	} else {
		cfg = cfg.WithFileURL(t.Endpoint)
	}

	if t.Permission == "" {
		t.Permission = DefaultPermission
	}
	if t.Protocol == "" {
		t.Protocol = DefaultProtocol
	}

	if err := t.configureMgr(cfg); err != nil {
		return fmt.Errorf("configure mgr: %w", err)
	}

	if err := t.configureUnderlyingFilesystem("nfs4"); err != nil {
		return fmt.Errorf("configure underlying filesystem: %w", err)
	}

	return nil
}

func (t *T) configureMgr(cfg *sgcp.Config) error {
	// Initialize SGCP API client
	httpClient, err := httpclientcache.Client(httpclientcache.Options{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if t.authInfoer == nil {
		t.authInfoer = &sgcphelper.GetAuthInfoFromDatastorePather{}
	}

	authInfo, err := t.authInfoer.GetAuthInfo(t.Secret)
	if err != nil {
		return fmt.Errorf("get auth info: %w", err)
	}
	tk := sgcp.NewTokenFactory(t.Log(), httpClient, &cfg.Auth, authInfo)

	t.mgr = &nfsClientMgr{
		uuid:        t.UUID,
		host:        t.Host,
		permission:  t.Permission,
		protocol:    t.Protocol,
		log:         t.Log(),
		nfsIgnored:  NfsClientIgnored,
		api:         sgcp.NewFilesAPI(cfg, httpClient, t.Log(), tk),
		cacheConfig: &cfg.Cache,
	}
	return nil
}

func (t *T) configureUnderlyingFilesystem(resType string) error {
	// Prepare the underlying filesystem resource
	r := resfshost.New().(*resfshost.T)
	if err := r.SetRID(t.RID()); err != nil {
		return fmt.Errorf("set underlying fs rid: %w", err)
	}
	if o := t.GetObject(); o != nil {
		r.SetObject(t.GetObject())
	}
	r.Type = resType
	r.Device = t.Device
	r.MountPoint = t.MountPoint
	r.MountOptions = t.MountOptions
	r.StatTimeout = t.StatTimeout
	r.DataRecv = t.DataRecv
	r.Perm = t.Perm
	if err := r.Configure(); err != nil {
		return fmt.Errorf("configure underlying fs: %w", err)
	}
	t.resFs = r

	t.DataRecv.SetReceiver(r)
	return nil
}

// Start starts the SGCP NFS filesystem resource
func (t *T) Start(ctx context.Context) error {
	if t.StartTimeout != nil && *t.StartTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.StartTimeout)
		defer cancel()
	}
	deadline, ok := ctx.Deadline()
	if ok {
		t.Log().Tracef("action context deadline: %v", deadline)
	}

	// Start the XaaS (SGCP API) part
	if err := t.fileStart(ctx); err != nil {
		return err
	}

	// Start underlying fs, this includes the data receive step
	if err := t.resFs.Start(ctx); err != nil {
		return err
	}

	return nil
}

// Stop stops the SGCP NFS filesystem resource
func (t *T) Stop(ctx context.Context) error {
	// stop the underlying filesystem
	if err := t.resFs.Stop(ctx); err != nil {
		return err
	}

	// Stop the XaaS part
	return t.fileStop(ctx)
}

// Status returns the combined status of the file and fs
func (t *T) Status(ctx context.Context) status.T {
	fileStatus := t.fileStatus(ctx)

	// Get underlying filesystem status
	fsStatus := t.resFs.Status(ctx)
	t.StatusLog().Merge(t.resFs.StatusLog())

	fileStatus.Add(fsStatus)
	return fileStatus
}

// Head returns the head path for the data receiver
func (t *T) Head() string {
	return t.resFs.Head()
}

// Label returns a label for the resource
func (t *T) Label(ctx context.Context) string {
	return t.resFs.Label(ctx)
}

// CanInstall checks if the resource can be installed
func (t *T) CanInstall(ctx context.Context) (bool, error) {
	return t.resFs.CanInstall(ctx)
}

// Boot stops the resource (for daemon bootstrap)
func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}

// fileStart handles the SGCP API part of starting the filesystem
func (t *T) fileStart(ctx context.Context) error {
	defer func() {
		_ = t.clearFileStatusCache()
	}()
	if sgcp.IsDisabled(rawconfig.NodeVarDir()) {
		t.Log().Infof("skipping file start %s: SGCP API disabled", t.UUID)
		return nil
	}

	t.Log().Infof("starting file %s", t.UUID)

	// Clear cache to get fresh data
	if err := t.clearFileStatusCache(); err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}

	// Check current status
	if t.fileStatus(ctx) == status.Up {
		t.Log().Infof("permission %s from host %s already created", t.Permission, t.Host)
		return nil
	}

	// Start the NFS client
	return t.mgr.Start(ctx, t.Exclusive)
}

// fileStop handles the SGCP API part of stopping the filesystem
func (t *T) fileStop(ctx context.Context) error {
	defer func() {
		_ = t.clearFileStatusCache()
	}()
	if sgcp.IsDisabled(rawconfig.NodeVarDir()) {
		t.Log().Infof("skipping file stop %s: SGCP API disabled", t.UUID)
		return nil
	}
	t.Log().Infof("stopping file %s", t.UUID)

	// Clear cache to get fresh data
	if err := t.clearFileStatusCache(); err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}

	// Check current status
	if t.fileStatus(ctx) == status.Down {
		t.Log().Infof("permission %s from host %s already deleted", t.Permission, t.Host)
		return nil
	}

	return t.mgr.Stop(ctx)
}

// fileStatus returns the status of the filesystem from the SGCP API
func (t *T) fileStatus(ctx context.Context) status.T {
	// Check if XaaS status is disabled
	if sgcp.IsDisabled(rawconfig.NodeVarDir()) {
		t.Log().Debugf("skipping file status %s: SGCP API disabled", t.UUID)
		return status.NotApplicable
	}

	fileInfo, err := t.getFileInfo(ctx)
	if err != nil {
		t.Log().Debugf("file object not visible in api: %s", err)
		t.StatusLog().Info("file object not visible in api")
		return status.Down
	}

	if fileInfo == nil {
		t.StatusLog().Info("file object not visible in api")
		return status.Down
	}

	clients := t.getNFSClients(fileInfo)
	n := len(clients)

	if t.Exclusive {
		if n > 1 {
			t.StatusLog().Warn(fmt.Sprintf("too many grants (%d)", n))
		}
	}

	if n == 0 {
		t.StatusLog().Info("no access granted")
		return status.Down
	}

	// Filter clients to only those matching our host and permission
	var matchingClients []NfsClient
	for _, client := range clients {
		if client.Host == t.Host && client.Permission == t.Permission {
			matchingClients = append(matchingClients, client)
		}
	}

	if len(matchingClients) == 0 {
		t.StatusLog().Info("access not in current grants")
		return status.Down
	}

	if fileInfo.Status == "passive" {
		t.StatusLog().Info("status passive, access granted")
		return status.Down
	}

	return status.Up
}

// getFileInfo retrieves filesystem information from the API
func (t *T) getFileInfo(ctx context.Context) (*FilesystemInfo, error) {
	// Use cached value if available
	if t.fileInfoCache != nil {
		return t.fileInfoCache, nil
	}

	fileInfo, err := t.mgr.getFileInfo(ctx)
	if err == nil {
		// Cache the result
		t.fileInfoCache = fileInfo
	}

	return fileInfo, err
}

// getNFSClients returns the NFS clients for the filesystem, filtered by ignored hosts
func (t *T) getNFSClients(fileInfo *FilesystemInfo) []NfsClient {
	if fileInfo == nil {
		return []NfsClient{}
	}

	var filteredClients []NfsClient
	for _, client := range fileInfo.NFSClients {
		if !t.isClientIgnored(client.Host) {
			filteredClients = append(filteredClients, client)
		}
	}

	return filteredClients
}

// isClientIgnored checks if a client host should be ignored
func (t *T) isClientIgnored(host string) bool {
	for _, ignored := range NfsClientIgnored {
		if host == ignored {
			return true
		}
	}
	return false
}

// clearFileStatusCache clears the filesystem info cache
func (t *T) clearFileStatusCache() error {
	var errs error
	t.fileInfoCache = nil
	for _, s := range []string{"getFileInfo"} {
		errs = errors.Join(errs, t.mgr.cacheClear(s))
	}
	return errs
}

// String returns a string representation of an NfsClient
func (c *NfsClient) String() string {
	if c == nil {
		return "NfsClient <nil>"
	}
	return fmt.Sprintf("NfsClient uuid:%s, host:%s, protocol:%s, permission:%s",
		c.UUID, c.Host, c.Protocol, c.Permission)
}
