package arrayhoc

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/sizeconv"
)

var (
	RenewStatus    = 401
	RequestTimeout = 10 * time.Second
	DefaultDelay   = 1 * time.Second
	Head           = "/v1"

	JobStatusInProgress        = "IN_PROGRESS"
	JobStatusSuccess           = "SUCCESS"
	JobStatusSuccessWithErrors = "SUCCESS_WITH_ERRORS"
	JobStatusFailed            = "FAILED"
)

type (
	itemser interface {
		Items() []any
		ItemsTotal() int
		ItemsNextToken() string
	}

	resizeMethod int

	OptGetItems struct {
		Volume OptVolume
		Filter string
	}

	OptMapping struct {
		Mappings       []string
		HostGroupNames []string
		LUN            int
	}

	OptVolume struct {
		ID     int
		Name   string
		Serial string
	}

	OptHost struct {
		Host      string
		HostGroup string
	}

	OptMapDisk struct {
		Volume  OptVolume
		Mapping OptMapping
	}

	OptUnmapDisk struct {
		Volume  OptVolume
		Mapping OptMapping
	}

	OptResizeDisk struct {
		Volume OptVolume
		Size   string
	}

	OptDelDisk struct {
		Volume OptVolume
	}

	OptAddVolume struct {
		Name          string
		Size          string
		PoolId        string
		Compression   bool
		Deduplication bool
	}

	OptAddDisk struct {
		Volume  OptAddVolume
		Mapping OptMapping
	}

	Array struct {
		*array.Array
		token string
	}

	hocStorageSystem struct {
		Accessible               bool    `json:"accessible,omitempty"`
		AllocatedToPool          int64   `json:"allocatedToPool,omitempty"`
		AvailableCapacity        int64   `json:"availableCapacity,omitempty"`
		CacheCapacity            *int64  `json:"cacheCapacity,omitempty"`
		CapacityEfficiencyRate   float32 `json:"capacityEfficiencyRate,omitempty"`
		CompressionAcceleration  *string `json:"compressionAcceleration,omitempty"`
		DataReductionSavingsRate float32 `json:"dataReductionSavingsRate,omitempty"`
		FirmwareVersion          string  `json:"firmwareVersion,omitempty"`
		GadSummary               string  `json:"gadSummary,omitempty"`
		Gum1IpAddress            string  `json:"gum1IpAddress,omitempty"`
		Gum2IpAddress            string  `json:"gum2IpAddress,omitempty"`
		HorcmVersion             *string `json:"horcmVersion,omitempty"`
		LastRefreshedTime        int64   `json:"lastRefreshedTime,omitempty"`
		MigrationTaskCount       int     `json:"migrationTaskCount,omitempty"`
		Model                    string  `json:"model,omitempty"`
		PrimaryGumNumber         int     `json:"primaryGumNumber,omitempty"`
		RmiPortNumber            *int    `json:"rmiPortNumber,omitempty"`
		StatusMessage            *string `json:"statusMessage,omitempty"`
		StorageSystemId          string  `json:"storageSystemId,omitempty"`
		StorageSystemName        string  `json:"storageSystemName,omitempty"`
		SubscribedCapacity       int64   `json:"subscribedCapacity,omitempty"`
		SvpFlashState            *string `json:"svpFlashState,omitempty"`
		SvpHttpsPortNumber       *int    `json:"svpHttpsPortNumber,omitempty"`
		SvpIpAddress             *string `json:"svpIpAddress,omitempty"`
		TotalEfficiency          any     `json:"totalEfficiency,omitempty"`
		TotalUsableCapacity      int64   `json:"totalUsableCapacity,omitempty"`
		UnallocatedToPool        int64   `json:"unallocatedToPool,omitempty"`
		Unified                  bool    `json:"unified,omitempty"`
		UnusedDisks              int     `json:"unusedDisks,omitempty"`
		UnusedDisksCapacity      int64   `json:"unusedDisksCapacity,omitempty"`
		UsedCapacity             int64   `json:"usedCapacity,omitempty"`
		Username                 string  `json:"username,omitempty"`
	}

	hocVolume struct {
		VolumeId                         int                                 `json:"volumeId,omitempty"`
		StorageSystemId                  string                              `json:"storageSystemId,omitempty"`
		StorageSystemName                string                              `json:"storageSystemName,omitempty"`
		PoolId                           string                              `json:"poolId,omitempty"`
		PoolName                         string                              `json:"poolName,omitempty"`
		Label                            string                              `json:"label,omitempty"`
		Size                             int64                               `json:"size,omitempty"`
		UsedCapacity                     int64                               `json:"usedCapacity,omitempty"`
		AvailableCapacity                int64                               `json:"availableCapacity,omitempty"`
		Utilization                      int                                 `json:"utilization,omitempty"`
		Attributes                       []string                            `json:"attributes,omitempty"`
		Status                           string                              `json:"status,omitempty"`
		Type                             string                              `json:"type,omitempty"`
		ProvisioningStatus               string                              `json:"provisioningStatus,omitempty"`
		PortIds                          []string                            `json:"portIds,omitempty"`
		HostGroupNames                   []string                            `json:"hostGroupNames,omitempty"`
		LUNs                             []int                               `json:"luns,omitempty"`
		NumberOfLunPaths                 int                                 `json:"numberOfLunPaths,omitempty"`
		DkcDataSavingType                string                              `json:"dkcDataSavingType,omitempty"`
		virtualStorageMachineInformation hocVirtualStorageMachineInformation `json:"virtualStorageMachineInformation,omitempty"`
		ResourceGroupId                  int                                 `json:"resourceGroupId,omitempty"`
		ResourceGroupName                string                              `json:"resourceGroupName,omitempty"`
		AluaEnabled                      bool                                `json:"aluaEnabled,omitempty"`
		TieringPolicy                    hocTieringPolicy                    `json:"tieringPolicy,omitempty"`
		T10PiEnabled                     bool                                `json:"t10PiEnabled,omitempty"`
		CompressionAcceleration          bool                                `json:"compressionAcceleration,omitempty"`
		CommandDevice                    *string                             `json:"commandDevice,omitempty"`
		AttachedVolumeServerSummary      []hocAttachedVolumeServerSummary    `json:"attachedVolumeServerSummary,omitempty"`
	}

	hocAttachedVolumeServerSummary struct {
		ServerId int       `json:"serverId,omitempty"`
		Paths    []hocPath `json:"paths,omitempty"`
	}

	hocPath struct {
		StoragePortId       string   `json:"storagePortId,omitempty"`
		StoragePortSystemId string   `json:"storagePortSystemId,omitempty"`
		LUN                 int      `json:"lun,omitempty"`
		HostGroupId         string   `json:"hostGroupId,omitempty"`
		Name                string   `json:"name,omitempty"`
		HostMode            string   `json:"hostMode,omitempty"`
		WWNs                []string `json:"wwns,omitempty"`
		HostModeOptions     []int    `json:"hostModeOptions,omitempty"`
		PreferredPath       bool     `json:"preferredPath,omitempty"`
	}

	hocTieringPolicy struct {
		Id          int    `json:"id,omitempty"`
		Name        string `json:"name,omitempty"`
		UserDefined bool   `json:"userDefined,omitempty"`
	}

	hocVirtualStorageMachineInformation struct {
		VirtualStorageMachineId string `json:"virtualStorageMachineId,omitempty"`
		StorageSystemId         string `json:"storageSystemId,omitempty"`
		Model                   string `json:"model,omitempty"`
		VirtualVolumeId         int    `json:"virtualVolumeId,omitempty"`
	}

	hocJob struct {
		JobId         string      `json:"jobId,omitempty"`
		Text          string      `json:"text,omitempty"`
		MessageCode   string      `json:"messageCode,omitempty"`
		Parameters    any         `json:"parameters,omitempty"`
		User          string      `json:"user,omitempty"`
		Status        string      `json:"status,omitempty"`
		CreatedDate   int64       `json:"createdDate,omitempty"`
		ScheduledDate int64       `json:"scheduledDate,omitempty"`
		StartDate     int64       `json:"startDate,omitempty"`
		ParentJobId   string      `json:"parentJobId,omitempty"`
		Reports       []hocReport `json:"reports,omitempty"`
		Links         []hocLink   `json:"links,omitempty"`
		Tags          []hocTag    `json:"tags,omitempty"`
		IsSystem      bool        `json:"isSystem,omitempty"`
	}

	hocReport struct {
		CreationDate  int64            `json:"creationDate,omitempty"`
		ReportMessage hocReportMessage `json:"reportMessage,omitempty"`
		Severity      string           `json:"severity,omitempty"`
	}

	hocReportMessage struct {
		MessageCode string         `json:"messageCode,omitempty"`
		Parameters  map[string]any `json:"parameters,omitempty"`
		Text        string         `json:"text,omitempty"`
	}

	hocTag struct {
		Tag string `json:"tag,omitempty"`
	}

	hocLink struct {
		Rel  string `json:"rel,omitempty"`
		Href string `json:"href,omitempty"`
	}

	hocBaseResponse struct {
		Total     int `json:"total,omitempty"`
		NextToken any `json:"nextToken,omitempty"`
	}

	hocResponse struct {
		hocBaseResponse
		Resources []any `json:"resources,omitempty"`
	}

	hocResponseJobs struct {
		hocBaseResponse
		Jobs []any `json:"jobs,omitempty"`
	}

	hocResponseVolumes struct {
		Total     int         `json:"total,omitempty"`
		NextToken any         `json:"nextToken,omitempty"`
		Resources []hocVolume `json:"resources,omitempty"`
	}
)

func (t hocBaseResponse) ItemsTotal() int {
	return t.Total
}

func (t hocBaseResponse) ItemsNextToken() string {
	if t.NextToken == nil {
		return ""
	}
	return t.NextToken.(string)
}

func (t hocResponseJobs) Items() []any {
	return t.Jobs
}

func (t hocResponse) Items() []any {
	return t.Resources
}

const (
	// Resize methods
	ResizeExact resizeMethod = iota
	ResizeUp
	ResizeDown
)

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "hoc"), NewDriver)
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
	newParent := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:           "array",
			Short:         "Manage a hocstorage storage array",
			SilenceUsage:  true,
			SilenceErrors: true,
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
	newResizeCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "resize",
			Short: "resize commands",
		}
		return cmd
	}

	newResizeDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "resize a volume",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptResizeDisk{
					Volume: OptVolume{
						ID:     volumeId,
						Name:   name,
						Serial: serial,
					},
					Size: size,
				}
				if data, err := t.ResizeDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagVolumeID(cmd)
		useFlagName(cmd)
		useFlagSerial(cmd)
		useFlagSize(cmd)
		return cmd
	}
	newUnmapDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "unmap a volume",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := OptUnmapDisk{
					Volume: OptVolume{
						ID:     volumeId,
						Name:   name,
						Serial: serial,
					},
					Mapping: OptMapping{
						Mappings:       mappings,
						HostGroupNames: hostGroups,
					},
				}
				if data, err := t.UnmapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagVolumeID(cmd)
		useFlagName(cmd)
		useFlagMapping(cmd)
		useFlagHostGroup(cmd)
		useFlagSerial(cmd)
		return cmd
	}
	newMapDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "map a volume",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := OptMapDisk{
					Volume: OptVolume{
						ID:     volumeId,
						Name:   name,
						Serial: serial,
					},
					Mapping: OptMapping{
						Mappings:       mappings,
						HostGroupNames: hostGroups,
						LUN:            lun,
					},
				}
				if data, err := t.MapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagVolumeID(cmd)
		useFlagName(cmd)
		useFlagMapping(cmd)
		useFlagLUN(cmd)
		useFlagHostGroup(cmd)
		useFlagSerial(cmd)
		return cmd
	}
	newDelDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "unmap a volume and delete",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptDelDisk{
					Volume: OptVolume{
						ID:     volumeId,
						Name:   name,
						Serial: serial,
					},
				}
				if data, err := t.DelDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagName(cmd)
		useFlagVolumeID(cmd)
		useFlagSerial(cmd)
		return cmd
	}
	newAddDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "add a volume and map",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := OptAddDisk{
					Volume: OptAddVolume{
						Name:          name,
						Size:          size,
						PoolId:        poolId,
						Compression:   compression,
						Deduplication: deduplication,
					},
					Mapping: OptMapping{
						Mappings:       mappings,
						LUN:            lun,
						HostGroupNames: hostGroups,
					},
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
		useFlagPoolID(cmd)
		useFlagMapping(cmd)
		useFlagLUN(cmd)
		useFlagHostGroup(cmd)
		useFlagCompression(cmd)
		useFlagDeduplication(cmd)
		return cmd
	}
	newGetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "get",
			Short: "get commands",
		}
		return cmd
	}
	newGetVolumesCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "volumes",
			Short: "get volumes",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{
					Volume: OptVolume{
						ID:     volumeId,
						Name:   name,
						Serial: serial,
					},
					Filter: filter,
				}
				data, err := t.GetVolumes(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagVolumeID(cmd)
		useFlagName(cmd)
		useFlagSerial(cmd)
		useFlagFilter(cmd)
		return cmd
	}
	newGetHostGroupsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "host-groups",
			Short: "get host groups",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetHostGroups(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetStoragePortsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "storage-ports",
			Short: "get storage ports",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetStoragePorts(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetStoragePoolsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "storage-pools",
			Short: "get storage pools",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetStoragePools(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetDisksCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disks",
			Short: "get disks",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetDisks(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetStorageSystemCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "storage-system",
			Short: "get storage system",
			RunE: func(_ *cobra.Command, _ []string) error {
				data, err := t.GetStorageSystem()
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		return cmd
	}
	newGetStorageSystemsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "storage-systems",
			Short: "get storage systems",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetStorageSystems(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetJobsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "jobs",
			Short: "get jobs",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetJobs(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetSystemTasksCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "system-tasks",
			Short: "get system tasks",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetSystemTasks(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}

	parent := newParent()

	// skip past the --array <array> arguments
	parent.SetArgs(os.Args[4:])

	addCmd := newAddCmd()
	addCmd.AddCommand(newAddDiskCmd())
	parent.AddCommand(addCmd)

	resizeCmd := newResizeCmd()
	resizeCmd.AddCommand(newResizeDiskCmd())
	parent.AddCommand(resizeCmd)

	delCmd := newDelCmd()
	delCmd.AddCommand(newDelDiskCmd())
	parent.AddCommand(delCmd)

	getCmd := newGetCmd()
	getCmd.AddCommand(newGetSystemTasksCmd())
	getCmd.AddCommand(newGetJobsCmd())
	getCmd.AddCommand(newGetHostGroupsCmd())
	getCmd.AddCommand(newGetStoragePortsCmd())
	getCmd.AddCommand(newGetStoragePoolsCmd())
	getCmd.AddCommand(newGetDisksCmd())
	getCmd.AddCommand(newGetStorageSystemCmd())
	getCmd.AddCommand(newGetStorageSystemsCmd())
	getCmd.AddCommand(newGetVolumesCmd())
	parent.AddCommand(getCmd)

	mapCmd := newMapCmd()
	mapCmd.AddCommand(newMapDiskCmd())
	parent.AddCommand(mapCmd)

	unmapCmd := newUnmapCmd()
	unmapCmd.AddCommand(newUnmapDiskCmd())
	parent.AddCommand(unmapCmd)

	return parent.Execute()
}

func (t Array) wwidPrefix() string {
	return t.Config().GetString(t.Key("wwid_prefix"))
}

func (t Array) api() string {
	return t.Config().GetString(t.Key("api"))
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t Array) delay() time.Duration {
	if d := t.Config().GetDuration(t.Key("delay")); d == nil {
		return DefaultDelay
	} else {
		return *d
	}
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

func (t Array) storageSystemId() string {
	return t.Config().GetString(t.Key("name"))
}

func (t Array) secret() string {
	return t.Config().GetString(t.Key("password"))
}

func (t *Array) sec() (object.Sec, error) {
	s, err := t.Config().GetStringStrict(t.Key("password"))
	if err != nil {
		return nil, err
	}
	return object.NewSec(s, object.WithVolatile(true))
}

func (t *Array) password() (string, error) {
	sec, err := t.sec()
	if err != nil {
		return "", err
	}
	b, err := sec.DecodeKey("password")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (t *Array) getToken() (string, error) {
	if t.token != "" {
		return t.token, nil
	}
	if err := t.newToken(); err != nil {
		return "", err
	}
	return t.token, nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
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

func (t *Array) newToken() error {
	authURL := fmt.Sprintf("%s/%s/security/tokens", t.api(), Head)
	req, err := http.NewRequest(http.MethodPost, authURL, nil)
	if err != nil {
		return err
	}
	password, err := t.password()
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Basic "+basicAuth(t.username(), password))
	req.Header.Add("Cache-Control", "no-cache")

	resp, err := t.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := validateResponse(resp); err != nil {
		return err
	}

	t.token = resp.Header.Get("X-Auth-Token")
	return nil
}

func (t *Array) DoJob(req *http.Request) (hocJob, error) {
	var job hocJob
	var jobRequestPath string

	getJob := func() error {
		req, err := t.newRequest(http.MethodGet, jobRequestPath, nil, nil)
		if err != nil {
			return err
		}
		if _, err := t.Do(req, &job); err != nil {
			return err
		}
		return nil
	}
	jobFinished := func() bool {
		switch job.Status {
		case JobStatusInProgress:
			return false
		default:
			return true
		}
	}

	_, err := t.Do(req, &job)
	if err != nil {
		return job, err
	}
	if jobFinished() {
		return job, nil
	}
	jobRequestPath = fmt.Sprintf("/jobs/%s", job.JobId)

	timeout := time.NewTicker(t.timeout())
	defer timeout.Stop()
	ticker := time.NewTicker(t.delay())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := getJob(); err != nil {
				return job, err
			}
			if jobFinished() {
				return job, nil
			}
		case <-timeout.C:
			return job, fmt.Errorf("timeout waiting for job to finish: %#v", job)
		}
	}

	return job, fmt.Errorf("unexpected")
}

func (t *Array) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := t.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := validateResponse(resp); err != nil {
		return nil, fmt.Errorf("validate response: %w", err)
	}

	err = decodeResponse(resp, v)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}

func dump(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func validateOptMapping(opt OptMapping) error {
	if len(opt.Mappings) == 0 && len(opt.HostGroupNames) > 0 {
		return fmt.Errorf("--mapping or --hostgroup is required")
	}
	if len(opt.Mappings) > 0 && len(opt.HostGroupNames) > 0 {
		return fmt.Errorf("--mapping and --hostgroup are mutually exclusive")
	}
	return nil
}

func validateOptVolume(opt OptVolume) error {
	if opt.Name == "" && opt.ID < 0 && opt.Serial == "" {
		return fmt.Errorf("--name, --id or --serial is required")
	}
	if opt.Name != "" && opt.ID >= 0 {
		return fmt.Errorf("--name and --id are mutually exclusive")
	}
	if opt.Name != "" && opt.Serial != "" {
		return fmt.Errorf("--name and --serial are mutually exclusive")
	}
	if opt.ID >= 0 && opt.Serial != "" {
		return fmt.Errorf("--serial and --id are mutually exclusive")
	}
	return nil
}

func (t *Array) ResizeDisk(opt OptResizeDisk) (hocVolume, error) {
	if err := validateOptVolume(opt.Volume); err != nil {
		return hocVolume{}, err
	}
	if opt.Size == "" {
		return hocVolume{}, fmt.Errorf("--size is required")
	}
	var method resizeMethod
	if len(opt.Size) > 1 {
		switch opt.Size[0] {
		case '+':
			opt.Size = opt.Size[1:]
			method = ResizeUp
		case '-':
			opt.Size = opt.Size[1:]
			method = ResizeDown
		}
	}
	sizeBytes, err := sizeconv.FromSize(opt.Size)
	if err != nil {
		return hocVolume{}, err
	}
	volume, err := t.getVolume(opt.Volume)
	if err != nil {
		return hocVolume{}, err
	}
	if method != ResizeExact {
		switch method {
		case ResizeUp:
			sizeBytes = volume.Size + sizeBytes
		case ResizeDown:
			sizeBytes = volume.Size - sizeBytes
		}
	}
	params := map[string]string{
		"ids": fmt.Sprint(volume.VolumeId),
	}
	data := map[string]string{
		"provisioned": fmt.Sprint(sizeBytes),
	}
	req, err := t.newRequest(http.MethodPatch, "/volumes", params, data)
	if err != nil {
		return hocVolume{}, err
	}
	var responseData hocResponseVolumes
	if _, err := t.Do(req, &responseData); err != nil {
		return hocVolume{}, err
	}
	if len(responseData.Resources) == 0 {
		return hocVolume{}, fmt.Errorf("no item in response")
	}
	return responseData.Resources[0], nil
}

func (t *Array) WWN(id int) string {
	s := fmt.Sprintf("%s%06x", t.wwidPrefix(), id)
	return strings.ToLower(s)
}

func (t *Array) AddDisk(opt OptAddDisk) (array.Disk, error) {
	var disk array.Disk
	driverData := make(map[string]any)
	volume, err := t.addVolume(opt.Volume)
	if err != nil {
		return disk, err
	}
	driverData["volume"] = volume
	disk.DriverData = driverData
	disk.DiskID = t.WWN(volume.VolumeId)
	disk.DevID = fmt.Sprint(volume.VolumeId)
	conns, err := t.MapDisk(OptMapDisk{
		Volume: OptVolume{
			ID: volume.VolumeId,
		},
		Mapping: opt.Mapping,
	})
	if err != nil {
		return disk, err
	}
	driverData["mappings"] = conns
	disk.DriverData = driverData
	return disk, nil
}

func (t *Array) addVolume(opt OptAddVolume) (hocVolume, error) {
	if opt.Name == "" {
		return hocVolume{}, fmt.Errorf("--name is required")
	}
	if opt.Size == "" {
		return hocVolume{}, fmt.Errorf("--size is required")
	}
	sizeBytes, err := sizeconv.FromSize(opt.Size)
	if err != nil {
		return hocVolume{}, err
	}
	var dkcDataSavingTypeOptions []string
	if opt.Deduplication {
		dkcDataSavingTypeOptions = append(dkcDataSavingTypeOptions, "DEDUPLICATION")
	}
	if opt.Compression {
		dkcDataSavingTypeOptions = append(dkcDataSavingTypeOptions, "COMPRESSION")
	}
	params := map[string]string{
		"names": opt.Name,
	}
	data := map[string]string{
		"poolId":            opt.PoolId,
		"capacityInBytes":   fmt.Sprint(sizeBytes),
		"label":             opt.Name,
		"dkcDataSavingType": strings.Join(dkcDataSavingTypeOptions, "_AND_"),
	}
	path := fmt.Sprintf("/storage-systems/%s/volumes", t.storageSystemId())
	req, err := t.newRequest(http.MethodPost, path, params, data)
	if err != nil {
		return hocVolume{}, err
	}
	var volume hocVolume
	job, err := t.DoJob(req)
	if err != nil {
		return volume, err
	}
	volumeId := findVolumeIdInJob(job)
	if volumeId < 0 {
		return volume, fmt.Errorf("volumme id not found in job reports")
	}
	return t.getVolume(OptVolume{ID: volumeId})
}

func findVolumeIdInJob(job hocJob) int {
	for _, report := range job.Reports {
		if i, ok := report.ReportMessage.Parameters["volumeId"]; !ok {
			continue
		} else {
			s, ok := i.(string)
			if !ok {
				return -1
			}
			s = strings.Fields(s)[0]
			if d, err := strconv.Atoi(s); err != nil {
				return -1
			} else {
				return d
			}
		}
	}
	return -1
}

func (t *Array) getHostName(hbaID string) (string, error) {
	/*
		opt := OptGetItems{
			Filter: fmt.Sprintf("wwns='%s'", hbaID),
		}
		hosts, err := t.GetHosts(opt)
		if err != nil {
			return "", err
		}
		l := make([]string, 0)
		for _, host := range hosts {
			if !host.IsLocal {
				continue
			}
			l = append(l, hosts[0].Name)
		}
		if n := len(l); n == 1 {
			return l[0], nil
		} else if n == 0 {
			return "", fmt.Errorf("no host found for hba id %s", hbaID)
		} else {
			return "", fmt.Errorf("too many hosts found for hba id %s: %s", hbaID, l)
		}
	*/
	return "", fmt.Errorf("TODO")
}

func formatWWN(s string) (string, error) {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	if len(s) != 16 {
		return "", fmt.Errorf("input wwn must be formatted as 524a9373b4a75e11 or 0x524a9373b4a75e11")
	}
	s = strings.ToUpper(s)
	return s[0:2] + ":" + s[2:4] + ":" + s[4:6] + ":" + s[6:8] + ":" + s[8:10] + ":" + s[10:12] + ":" + s[12:14] + ":" + s[14:], nil
}

func (t *Array) getHostsFromMappings(mappings []string) (map[string][]string, error) {
	/*
		m := make(map[string][]string)
		for _, mapping := range mappings {
			elements := strings.Split(mapping, ":")
			if len(elements) != 2 {
				return nil, fmt.Errorf("invalid mapping: %s: must be <hba>:<tgt>[,<tgt>...]", mapping)
			}
			hbaID := elements[0]
			wwn, err := formatWWN(hbaID)
			if err != nil {
				return nil, err
			}
			if len(elements[1]) == 0 {
				return nil, fmt.Errorf("invalid mapping: %s: must be <hba>:<tgt>[,<tgt>...]", mapping)
			}
			targets := strings.Split(elements[1], ",")
			if len(targets) == 0 {
				return nil, fmt.Errorf("invalid mapping: %s: must be <hba>:<tgt>[,<tgt>...]", mapping)
			}
			hostName, err := t.getHostName(wwn)
			if err != nil {
				return nil, err
			}
			for _, target := range targets {
				wwn, err := formatWWN(target)
				if err != nil {
					return nil, err
				}
				opt := OptGetItems{
					Filter: fmt.Sprintf("fc.wwn='%s' and services='scsi-fc' and enabled='true'", wwn),
				}
				networkInterfaces, err := t.GetNetworkInterfaces(opt)
				if err != nil {
					return nil, err
				}
				if len(networkInterfaces) == 0 {
					continue
				}
				if v, ok := m[hostName]; ok {
					m[hostName] = append(v, wwn)
				} else {
					m[hostName] = []string{wwn}
				}
			}
		}
		return m, nil
	*/
	return nil, fmt.Errorf("TODO")
}

func (t *Array) mapVolume(volumeName, hostName, hostGroupName string, lun int) (hocVolume, error) {
	params := map[string]string{
		"volume_names": volumeName,
	}
	if hostName != "" {
		params["host_names"] = hostName
	}
	if hostGroupName != "" {
		params["host_group_names"] = hostGroupName
	}
	if lun >= 0 {
		params["lun"] = fmt.Sprint(lun)
	}
	/*
		req, err := t.newRequest(http.MethodPost, "/connections", params, nil)
		if err != nil {
			return hocVolume{}, err
		}
			var responseData hocResponseVolumeConnections
			_, err = t.Do(req, &responseData)
			if err != nil {
				return hocVolume{}, err
			}
			if len(responseData.Resources) == 0 {
				return hocVolume{}, fmt.Errorf("no connection item in response")
			}
			return responseData.Resources[0], nil
	*/
	return hocVolume{}, fmt.Errorf("TODO")
}

func (t *Array) deleteAllVolumeConnections(volumeName string) (hocVolume, error) {
	/*
		conns, err := t.deleteHostGroupVolumeConnections(volumeName)
		if err != nil {
			return conns, err
		}
		_, err = t.deleteHostVolumeConnections(volumeName)
		if err != nil {
			return conns, err
		}
		return conns, nil
	*/
	return hocVolume{}, fmt.Errorf("TODO")
}

func (t *Array) deleteHostGroupVolumeConnections(volumeName string) (hocVolume, error) {
	/*
		opt := OptGetItems{
			Filter: fmt.Sprintf("volume.name='%s'", volumeName),
		}
		conns, err := t.GetConnections(opt)
		if err != nil {
			return []hocVolumeConnection{}, nil
		}
		hostGroups := make(map[string]any, 0)
		for _, conn := range conns {
			if conn.HostGroup.Name != "" {
				hostGroups[conn.HostGroup.Name] = nil
			}
		}
		if len(hostGroups) > 0 {
			params := map[string]string{
				"volume_names":     volumeName,
				"host_group_names": strings.Join(xmap.Keys(hostGroups), ","),
			}
			req, err := t.newRequest(http.MethodDelete, "/connections", params, nil)
			if err != nil {
				return conns, err
			}
			var responseData any
			_, err = t.Do(req, &responseData)
			if err != nil {
				return conns, err
			}
		}
		return conns, nil
	*/
	return hocVolume{}, fmt.Errorf("TODO")
}

func (t *Array) deleteHostVolumeConnections(volumeName string) (hocVolume, error) {
	/*
		opt := OptGetItems{
			Filter: fmt.Sprintf("volume.name='%s'", volumeName),
		}
		conns, err := t.GetConnections(opt)
		if err != nil {
			return []hocVolumeConnection{}, nil
		}
		hosts := make(map[string]any, 0)
		for _, conn := range conns {
			if conn.Host.Name != "" {
				hosts[conn.Host.Name] = nil
			}
		}
		if len(hosts) > 0 {
			params := map[string]string{
				"volume_names": volumeName,
				"host_names":   strings.Join(xmap.Keys(hosts), ","),
			}
			req, err := t.newRequest(http.MethodDelete, "/connections", params, nil)
			if err != nil {
				return conns, err
			}
			var responseData any
			_, err = t.Do(req, &responseData)
			if err != nil {
				return conns, err
			}
		}
		return conns, nil
	*/
	return hocVolume{}, fmt.Errorf("TODO")
}

func (t *Array) unmapVolume(volumeName, hostName, hostGroupName string) error {
	params := map[string]string{
		"volume_names": volumeName,
	}
	if hostName != "" {
		params["host_names"] = hostName
	}
	if hostGroupName != "" {
		params["host_group_names"] = hostGroupName
	}
	req, err := t.newRequest(http.MethodDelete, "/connections", params, nil)
	if err != nil {
		return err
	}
	var responseData any
	_, err = t.Do(req, &responseData)
	if err != nil {
		return err
	}
	return nil
}

func (t *Array) UnmapDisk(opt OptUnmapDisk) (hocVolume, error) {
	/*
		if err := validateOptVolume(opt.Volume); err != nil {
			return nil, err
		}
		if err := validateOptMapping(opt.Mapping); err != nil {
			return nil, err
		}
		volume, err := t.getVolume(opt.Volume)
		if err != nil {
			return nil, err
		}
		ConnectionsDeleted := make([]hocVolumeConnection, 0)
		hostGroupsDeleted := make(map[string]any)
		switch {
		case len(mappings) > 0:
			hosts, err := t.getHostsFromMappings(opt.Mapping.Mappings)
			if err != nil {
				return nil, err
			}
			for hostName, _ := range hosts {
				queryOpt := OptGetItems{
					Filter: fmt.Sprintf("volume.name='%s' and host.name='%s'", volume.Label, hostName),
				}
				conns, err := t.GetConnections(queryOpt)
				if err != nil {
					return nil, err
				}
				if len(conns) < 1 {
					return nil, fmt.Errorf("connection not found: volume.name='%s' and host.name='%s'", volume.Label, hostName)
				} else if len(conns) > 1 {
					return nil, fmt.Errorf("too many connections found: %d matches with filter volume.name='%s' and host.name='%s'", len(conns), volume.Label, hostName)
				}
				if err := t.unmapVolume(volume.Label, hostName, ""); err != nil {
					return nil, err
				}
				ConnectionsDeleted = append(ConnectionsDeleted, conns[0])
			}
		default:
			queryOpt := OptGetItems{
				Filter: fmt.Sprintf("volume.name='%s'", volume.Label),
			}
			conns, err := t.GetConnections(queryOpt)
			if err != nil {
				return nil, err
			}
				for _, conn := range conns {
					switch {
					case opt.Mapping.HostName != "":
						if conn.Host.Name != opt.Mapping.HostName {
							continue
						}
						if err := t.unmapVolume(volume.Label, opt.Mapping.HostName, ""); err != nil {
							return ConnectionsDeleted, err
						} else {
							ConnectionsDeleted = append(ConnectionsDeleted, conn)
						}
					case opt.Mapping.HostGroupName != "":
						if conn.HostGroup.Name != opt.Mapping.HostGroupName {
							continue
						}
						if _, ok := hostGroupsDeleted[opt.Mapping.HostGroupName]; ok {
							continue
						} else if err := t.unmapVolume(volume.Label, "", opt.Mapping.HostGroupName); err != nil {
							return ConnectionsDeleted, err
						} else {
							ConnectionsDeleted = append(ConnectionsDeleted, conn)
							hostGroupsDeleted[opt.Mapping.HostGroupName] = nil
						}
					case conn.HostGroup.Name != "":
						if _, ok := hostGroupsDeleted[conn.HostGroup.Name]; ok {
							continue
						} else if err := t.unmapVolume(volume.Label, "", conn.HostGroup.Name); err != nil {
							return ConnectionsDeleted, err
						} else {
							ConnectionsDeleted = append(ConnectionsDeleted, conn)
							hostGroupsDeleted[conn.HostGroup.Name] = nil
						}
					case conn.Host.Name != "":
						if err := t.unmapVolume(volume.Label, conn.Host.Name, ""); err != nil {
							return ConnectionsDeleted, err
						} else {
							ConnectionsDeleted = append(ConnectionsDeleted, conn)
						}
					}
				}
		}
		return ConnectionsDeleted, nil
	*/
	return hocVolume{}, fmt.Errorf("TODO")
}

func (t *Array) MapDisk(opt OptMapDisk) (any, error) {
	if err := validateOptVolume(opt.Volume); err != nil {
		return nil, err
	}
	if err := validateOptMapping(opt.Mapping); err != nil {
		return nil, err
	}
	/*
		volume, err := t.getVolume(opt.Volume)
		if err != nil {
			return nil, err
		}
				ConnectionsAdded := make([]hocVolumeConnection, 0)
				switch {
				case len(opt.Mapping.Mappings) > 0:
					hosts, err := t.getHostsFromMappings(opt.Mapping.Mappings)
					if err != nil {
						return nil, err
					}
					for hostName, _ := range hosts {
						if conn, err := t.mapVolume(volume.Label, hostName, "", opt.Mapping.LUN); err != nil {
							return nil, err
						} else {
							ConnectionsAdded = append(ConnectionsAdded, conn)
						}
					}
						case opt.Mapping.HostGroupName != "":
							if conn, err := t.mapVolume(volume.Label, "", opt.Mapping.HostGroupName, opt.Mapping.LUN); err != nil {
								return ConnectionsAdded, err
							} else {
								ConnectionsAdded = append(ConnectionsAdded, conn)
							}
				}
			return ConnectionsAdded, nil
	*/
	return hocVolume{}, fmt.Errorf("TODO")
}

func (opt OptVolume) Filter() string {
	if opt.ID > 0 {
		return fmt.Sprintf("volumeId:%d", opt.ID)
	} else if opt.Name != "" {
		return fmt.Sprintf("label:%s", opt.Name)
	} else if opt.Serial != "" {
		return fmt.Sprintf("serial:%s", opt.Serial)
	} else {
		return ""
	}
}

func (t *Array) getVolume(opt OptVolume) (hocVolume, error) {
	var (
		volume   hocVolume
		items    []hocVolume
		err      error
		queryOpt OptGetItems
	)
	if s := opt.Filter(); s == "" {
		return volume, fmt.Errorf("id, name and serial are empty. refuse to get all volumes")
	} else {
		queryOpt.Filter = s
	}
	items, err = t.GetVolumes(queryOpt)
	if err != nil {
		return volume, err
	}
	if n := len(items); n > 1 {
		return volume, fmt.Errorf("%d volumes found matching %s", n, queryOpt.Filter)
	}
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return volume, err
		}
		err = json.Unmarshal(b, &volume)
		if err != nil {
			return volume, err
		}
		return volume, nil
	}
	return volume, fmt.Errorf("no volume found matching %s", filter)
}

func (t *Array) DelDisk(opt OptDelDisk) (array.Disk, error) {
	var disk array.Disk

	/*
		conns, err := t.deleteAllVolumeConnections(volume.Label)
		if err != nil {
			return disk, err
		}
		driverData["mappings"] = conns
		disk.DriverData = driverData

	*/
	volume, err := t.delVolume(opt)
	if err != nil {
		return disk, err
	}
	disk.DiskID = t.WWN(volume.VolumeId)
	disk.DevID = fmt.Sprint(volume.VolumeId)
	driverData := make(map[string]any)
	driverData["volume"] = volume
	disk.DriverData = driverData

	return disk, nil
}

func (t *Array) delVolume(opt OptDelDisk) (hocVolume, error) {
	if err := validateOptVolume(opt.Volume); err != nil {
		return hocVolume{}, err
	}
	filter := opt.Volume.Filter()
	if filter == "" {
		return hocVolume{}, fmt.Errorf("no volume selector")
	}
	volumes, err := t.GetVolumes(OptGetItems{Filter: filter})
	if err != nil {
		return hocVolume{}, err
	}
	if n := len(volumes); n == 0 {
		return hocVolume{}, fmt.Errorf("no volume found for selector %s", filter)
	} else if n > 1 {
		return hocVolume{}, fmt.Errorf("%d volumes found for selector %s", n, filter)
	}

	volume := volumes[0]
	path := fmt.Sprintf("/storage-systems/%s/volumes/%d", t.storageSystemId(), volume.VolumeId)
	req, err := t.newRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return volume, err
	}
	job, err := t.DoJob(req)
	if err != nil {
		return volume, err
	}
	if job.Status == JobStatusFailed {
		return volume, fmt.Errorf("job failed: %#v", job)
	}
	return volume, nil
}

func (t *Array) GetStorageSystem() (hocStorageSystem, error) {
	storageSystems, err := t.GetStorageSystems(OptGetItems{
		Filter: "storageSystemId:" + t.storageSystemId(),
	})
	if err != nil {
		return hocStorageSystem{}, err
	}
	if len(storageSystems) == 0 {
		return hocStorageSystem{}, fmt.Errorf("storage system %s not found", t.storageSystemId())
	}
	return storageSystems[0], nil
}

func (t *Array) GetStorageSystems(opt OptGetItems) ([]hocStorageSystem, error) {
	params := getParams(opt)
	l, err := t.doGet("GET", "/storage-systems", params, nil)
	if err != nil {
		return nil, err
	}
	storageSystems := make([]hocStorageSystem, len(l))
	for i, item := range l {
		var storageSystem hocStorageSystem
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &storageSystem)
		storageSystems[i] = storageSystem
	}
	return storageSystems, nil
}

func (t *Array) GetVolumes(opt OptGetItems) ([]hocVolume, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/volumes", t.storageSystemId())
	l, err := t.doGet("GET", path, params, nil)
	if err != nil {
		return nil, err
	}
	volumes := make([]hocVolume, len(l))
	for i, item := range l {
		var volume hocVolume
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &volume)
		volumes[i] = volume
	}
	return volumes, nil
}

func (t *Array) GetVolumeGroups(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/volume-groups", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func (t *Array) GetControllers(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/controllers", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func (t *Array) GetJobs(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/jobs")
	var r hocResponseJobs
	return t.doGetIn("GET", path, params, nil, &r)
}

func (t *Array) GetSystemTasks(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/system-tasks", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func (t *Array) GetDisks(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/disks", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func (t *Array) GetStoragePools(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/storage-pools", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func (t *Array) GetStoragePorts(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/storage-ports", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func (t *Array) GetHostGroups(opt OptGetItems) (any, error) {
	params := getParams(opt)
	path := fmt.Sprintf("/storage-systems/%s/host-groups", t.storageSystemId())
	return t.doGet("GET", path, params, nil)
}

func getParams(opt OptGetItems) map[string]string {
	params := make(map[string]string)
	if opt.Filter != "" {
		params["q"] = opt.Filter
	} else if filter := opt.Volume.Filter(); filter != "" {
		params["q"] = filter
	}
	return params
}

func (t *Array) doGet(method string, path string, params map[string]string, data interface{}) ([]any, error) {
	var r hocResponse
	return t.doGetIn(method, path, params, data, &r)
}

func (t *Array) doGetIn(method string, path string, params map[string]string, data interface{}, r itemser) ([]any, error) {
	req, err := t.newRequest(method, path, params, data)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	_, err = t.Do(req, r)
	if err != nil {
		return nil, err
	}
	for len(items) < r.ItemsTotal() {
		itemsBatch := r.Items()
		if len(itemsBatch) == 0 {
			break
		}
		items = append(items, itemsBatch...)

		if len(items) < r.ItemsTotal() {
			if r.ItemsNextToken() != "" {
				if params == nil {
					params = map[string]string{"nextToken": r.ItemsNextToken()}
				} else {
					params["nextToken"] = r.ItemsNextToken()
				}
				req, err := t.newRequest(method, path, params, data)
				if err != nil {
					return nil, err
				}

				_, err = t.Do(req, r)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return items, nil
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

	token, err := t.getToken()
	if err != nil {
		return nil, err
	}

	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Auth-Token", token)

	return req, err
}

// decodeResponse function reads the http response body into an interface.
func decodeResponse(r *http.Response, v interface{}) error {
	if r.StatusCode == 204 {
		return nil
	}
	if v == nil {
		return fmt.Errorf("nil interface provided to decodeResponse")
	}

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	if len(bodyBytes) == 0 {
		return nil
	}

	bodyString := string(bodyBytes)

	err := json.Unmarshal([]byte(bodyString), &v)

	return err
}

// validateResponse checks that the http response is within the 200 range.
// Some functionality needs to be added here to check for some specific errors,
// and probably add the equivlents to hocError and hocHTTPError from the Python
// REST client.
func validateResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	return fmt.Errorf("Response code: %d, Response body: %s", r.StatusCode, bodyString)
}
