package arraypure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devans10/pugo/pure1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/opensvc/om3/util/xmap"
)

var (
	WWIDPrefix      = "624a9370"
	RenewStatus     = 403
	ItemsPerPage    = 100
	MaxPages        = 1000
	DayMilliseconds = 24 * 60 * 60 * 1000
	RequestTimeout  = 10
	Head            = "/api/2.8"
)

type (
	resizeMethod int

	OptGetItems struct {
		Filter string
	}

	OptMapping struct {
		Mappings      []string
		HostName      string
		HostGroupName string
		LUN           int
	}

	OptVolume struct {
		ID     string
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
		Volume   OptVolume
		Size     string
		Truncate bool
	}

	OptDelDisk struct {
		Volume OptVolume
		Now    bool
	}

	OptAddDisk struct {
		Name     string
		Size     string
		Mappings []string
		LUN      int
	}

	Array struct {
		*array.Array
		token *pureToken
	}

	pureNetworkInterfaceEth struct {
		Address       *string `json:"address,omitempty"`
		Gateway       *string `json:"gateway,omitempty"`
		MacAddress    *string `json:"mac_address,omitempty"`
		Mtu           *int32  `json:"mtu,omitempty"`
		Netmask       *string `json:"netmask,omitempty"`
		Subinterfaces *[]struct {
			Name *string `json:"name,omitempty"`
		} `json:"subinterfaces,omitempty"`
		Subnet *struct {
			Name *string `json:"name,omitempty"`
		} `json:"subnet,omitempty"`
		Subtype *string `json:"subtype,omitempty"`
		VLAN    *int32  `json:"vlan,omitempty"`
	}

	pureNetworkInterfaceFC struct {
		WWN *string `json:"wwn,omitempty"`
	}

	pureNetworkInterface struct {
		Enabled       *bool                    `json:"enabled,omitempty"`
		Eth           *pureNetworkInterfaceEth `json:"eth,omitempty"`
		FC            *pureNetworkInterfaceFC  `json:"fc,omitempty"`
		InterfaceType *string                  `json:"interface_type,omitempty"`
		Name          *string                  `json:"name,omitempty"`
		Services      *[]string                `json:"services,omitempty"`
		Speed         *int64                   `json:"speed,omitempty"`
	}

	pureSourceIdentifiers struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	}

	purePodIdentifiers struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	}

	pureVolumeGroupIdentifiers struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	}

	pureVolumeIdentifiers struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	}

	pureVolumePriorityAdjustment struct {
		PriorityAdjustmentOperator string `json:"priority_adjustment_operator,omitempty"`
		PriorityAdjustmentValue    int32  `json:"priority_adjustment_value,omitempty"`
	}

	pureVolumeQOS struct {
		BandwidthLimit int64 `json:"bandwidth_limit,omitempty"`
		IOPSLimit      int64 `json:"iops_limit,omitempty"`
	}

	pureVolume struct {
		ID                      string                       `json:"id,omitempty"`
		Name                    string                       `json:"name,omitempty"`
		ConnectionCount         int64                        `json:"connection_count,omitempty"`
		Created                 int64                        `json:"created,omitempty"`
		Destroyed               bool                         `json:"destroyed,omitempty"`
		HostEncryptionKeyStatus string                       `json:"host_encryption_key_status,omitempty"`
		Provisioned             int64                        `json:"provisioned,omitempty"`
		QOS                     pureVolumeQOS                `json:"qos,omitempty"`
		PriorityAdjustment      pureVolumePriorityAdjustment `json:"priority_adjustment,omitempty"`
		Serial                  string                       `json:"serial,omitempty"`
		Space                   map[string]any               `json:"space,omitempty"`
		TimeRemaining           int64                        `json:"time_remaining,omitempty"`
		Pod                     purePodIdentifiers           `json:"pod,omitempty"`
		Source                  pureSourceIdentifiers        `json:"source,omitempty"`
		SubType                 string                       `json:"subtype,omitempty"`
		VolumeGroup             pureVolumeGroupIdentifiers   `json:"volume_group,omitempty"`
		RequestedPromotionState string                       `json:"requested_promotion_state,omitempty"`
		PromotionStatus         string                       `json:"promotion_status,omitempty"`
		Priority                int32                        `json:"priority,omitempty"`
	}

	pureVolumeConnection struct {
		Host             pureHostIdentifiers      `json:"host"`
		HostGroup        pureHostGroupIdentifiers `json:"host_group"`
		LUN              int64                    `json:"lun"`
		ProtocolEndpoint map[string]string        `json:"protocol_endpoint"`
		Volume           pureVolumeIdentifiers    `json:"volume"`
	}

	pureChap struct {
		HostPassword   string `json:"host_password"`
		HostUser       string `json:"host_user"`
		TargetPassword string `json:"target_password"`
		TargetUser     string `json:"target_user"`
	}

	pureHostIdentifiers struct {
		Name string `json:"name"`
	}

	pureHostGroupIdentifiers struct {
		Name string `json:"name"`
	}

	pureHostPortConnectivity struct {
		Details string `json:"details"`
		Status  string `json:"status"`
	}

	pureArray struct {
		ID                 string         `json:"id"`
		Name               string         `json:"name"`
		Banner             string         `json:"banner"`
		Capacity           int64          `json:"capacity"`
		ConsoleLockEnabled bool           `json:"console_lock_enabled"`
		Encryption         any            `json:"encryption"`
		EradicationConfig  any            `json:"eradication_config"`
		IdleTimeout        int32          `json:"idle_timeout"`
		NTPServers         []string       `json:"ntp_servers"`
		OS                 string         `json:"os"`
		Parity             float32        `json:"parity"`
		SCSITimeout        int32          `json:"scsi_timeout"`
		Space              pureArraySpace `json:"space"`
		Version            string         `json:"version"`
	}

	pureArraySpace struct {
		DataReduction      float32 `json:"data_reduction"`
		Shared             int64   `json:"shared"`
		Snapshots          int64   `json:"snapshots"`
		System             int64   `json:"system"`
		ThinProvisioning   float32 `json:"thin_provisioning"`
		TotalPhysical      int64   `json:"total_physical"`
		TotalProvisioned   int64   `json:"total_provisioned"`
		TotalReduction     float32 `json:"total_reduction"`
		Unique             int64   `json:"unique"`
		Virtual            int64   `json:"virtual"`
		Replication        int64   `json:"replication"`
		SharedEffective    int64   `json:"shared_effective"`
		SnapshotsEffective int64   `json:"snapshots_effective"`
		UniqueEffective    int64   `json:"unique_effective"`
		TotalEffective     int64   `json:"total_effective"`
	}

	pureHostSpace struct {
		DataReduction    float32 `json:"data_reduction"`
		Shared           int64   `json:"shared"`
		Snapshots        int64   `json:"snapshots"`
		System           int64   `json:"system"`
		ThinProvisioning float32 `json:"thin_provisioning"`
		TotalPhysical    int64   `json:"total_physical"`
		TotalProvisioned int64   `json:"total_provisioned"`
		TotalReduction   float32 `json:"total_reduction"`
		Unique           int64   `json:"unique"`
		Virtual          int64   `json:"virtual"`
	}

	pureArrayIdentifiers struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	pureHost struct {
		Name             string                   `json:"name"`
		Chap             pureChap                 `json:"chap"`
		ConnectionCount  int64                    `json:"connection_count"`
		HostGroup        pureHostGroupIdentifiers `json:"host_group"`
		IQNs             []string                 `json:"iqns"`
		NQNs             []string                 `json:"nqns"`
		Personality      string                   `json:"personality"`
		PortConnectivity pureHostPortConnectivity `json:"port_connectivity"`
		Space            pureHostSpace            `json:"space"`
		PreferredArrays  []pureArrayIdentifiers   `json:"preferred_arrays"`
		WWNs             []string                 `json:"wwns"`
		IsLocal          bool                     `json:"is_local"`
		VLAN             string                   `json:"vlan"`
	}

	pureResponse struct {
		TotalItems        int   `json:"total_item_count,omitempty"`
		ContinuationToken any   `json:"continuation_token,omitempty"`
		Items             []any `json:"items,omitempty"`
	}

	pureResponseVolumeConnections struct {
		TotalItems        int                    `json:"total_item_count,omitempty"`
		ContinuationToken any                    `json:"continuation_token,omitempty"`
		Items             []pureVolumeConnection `json:"items,omitempty"`
	}

	pureResponseVolumes struct {
		TotalItems        int          `json:"total_item_count,omitempty"`
		ContinuationToken any          `json:"continuation_token,omitempty"`
		Items             []pureVolume `json:"items,omitempty"`
	}

	pureToken struct {
		AccessToken     string `json:"access_token,omitempty"`
		IssuedTokenType string `json:"issued_token_type,omitempty"`
		TokenType       string `json:"token_type,omitempty"`
		ExpiresIn       int    `json:"expires_in,omitempty"`
	}
)

const (
	// Resize methods
	ResizeExact resizeMethod = iota
	ResizeUp
	ResizeDown
)

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "pure"), NewDriver)
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
			Short:         "Manage a purestorage storage array",
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
						ID:     id,
						Name:   name,
						Serial: serial,
					},
					Size:     size,
					Truncate: truncate,
				}
				if data, err := t.ResizeDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagID(cmd)
		useFlagName(cmd)
		useFlagSerial(cmd)
		useFlagSize(cmd)
		useFlagTruncate(cmd)
		return cmd
	}
	newUnmapDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "unmap a volume",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := OptUnmapDisk{
					Volume: OptVolume{
						ID:     id,
						Name:   name,
						Serial: serial,
					},
					Mapping: OptMapping{
						Mappings:      mappings,
						HostName:      host,
						HostGroupName: hostGroup,
					},
				}
				if data, err := t.UnmapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagID(cmd)
		useFlagName(cmd)
		useFlagMapping(cmd)
		useFlagHost(cmd)
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
						ID:     id,
						Name:   name,
						Serial: serial,
					},
					Mapping: OptMapping{
						Mappings:      mappings,
						HostName:      host,
						HostGroupName: hostGroup,
						LUN:           lun,
					},
				}
				if data, err := t.MapDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagID(cmd)
		useFlagName(cmd)
		useFlagMapping(cmd)
		useFlagLUN(cmd)
		useFlagHost(cmd)
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
						ID:     id,
						Name:   name,
						Serial: serial,
					},
					Now: now,
				}
				if data, err := t.DelDisk(opt); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagName(cmd)
		useFlagNow(cmd)
		useFlagID(cmd)
		useFlagSerial(cmd)
		return cmd
	}
	newAddDiskCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "disk",
			Short: "add a volume and map",
			RunE: func(cmd *cobra.Command, _ []string) error {
				opt := OptAddDisk{
					Name:     name,
					Size:     size,
					Mappings: mappings,
					LUN:      lun,
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
		useFlagLUN(cmd)
		return cmd
	}
	newGetCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "get",
			Short: "get commands",
		}
		return cmd
	}
	newGetHostsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "hosts",
			Short: "get hosts",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetHosts(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetConnectionsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "connections",
			Short: "get connections",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetConnections(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetVolumesCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "volumes",
			Short: "get volumes",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetVolumes(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetControllersCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "controllers",
			Short: "get controllers",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetControllers(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetDrivesCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "drives",
			Short: "get drives",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetDrives(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetPodsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "pods",
			Short: "get pods",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetPods(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetPortsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "ports",
			Short: "get ports",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetPorts(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetNetworkInterfacesCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "interfaces",
			Short: "get network interfaces",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetNetworkInterfaces(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetVolumeGroupsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "volumegroups",
			Short: "get volumegroups",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetVolumeGroups(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetHostGroupsCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "hostgroups",
			Short: "get hostgroups",
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
	newGetArraysCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "arrays",
			Short: "get arrays",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetArrays(opt)
				if err != nil {
					return err
				}
				return dump(data)
			},
		}
		useFlagFilter(cmd)
		return cmd
	}
	newGetHardwareCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "hardware",
			Short: "get hardware",
			RunE: func(_ *cobra.Command, _ []string) error {
				opt := OptGetItems{Filter: filter}
				data, err := t.GetHardware(opt)
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
	parent.SetArgs(array.SkipArgs())

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
	getCmd.AddCommand(newGetHostsCmd())
	getCmd.AddCommand(newGetConnectionsCmd())
	getCmd.AddCommand(newGetControllersCmd())
	getCmd.AddCommand(newGetDrivesCmd())
	getCmd.AddCommand(newGetHardwareCmd())
	getCmd.AddCommand(newGetArraysCmd())
	getCmd.AddCommand(newGetHostGroupsCmd())
	getCmd.AddCommand(newGetVolumeGroupsCmd())
	getCmd.AddCommand(newGetNetworkInterfacesCmd())
	getCmd.AddCommand(newGetPortsCmd())
	getCmd.AddCommand(newGetPodsCmd())
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

func (t Array) api() string {
	return t.Config().GetString(t.Key("api"))
}

func (t Array) clientID() string {
	return t.Config().GetString(t.Key("client_id"))
}

func (t Array) keyID() string {
	return t.Config().GetString(t.Key("key_id"))
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t Array) issuer() string {
	return t.Config().GetString(t.Key("issuer"))
}

func (t Array) insecure() bool {
	return t.Config().GetBool(t.Key("insecure"))
}

func (t Array) secret() string {
	return t.Config().GetString(t.Key("secret"))
}

func (t *Array) sec() (object.Sec, error) {
	s, err := t.Config().GetStringStrict(t.Key("secret"))
	if err != nil {
		return nil, err
	}
	path, err := naming.ParsePath(s)
	if err != nil {
		return nil, err
	}
	return object.NewSec(path, object.WithVolatile(true))
}

func (t *Array) privateKey() ([]byte, error) {
	sec, err := t.sec()
	if err != nil {
		return nil, err
	}
	return sec.DecodeKey("private_key")
}

func (t *Array) getToken() (*pureToken, error) {
	if t.token != nil {
		return t.token, nil
	}
	if err := t.newToken(); err != nil {
		return nil, err
	}
	return t.token, nil
}

func (t *Array) newToken() error {
	username := t.username()
	issuer := t.issuer()
	now := time.Now().Unix()
	if issuer == "" {
		issuer = username
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud": t.clientID(),
		"sub": username,
		"iss": issuer,
		"iat": now,
		"exp": now + int64(DayMilliseconds),
	})
	token.Header["kid"] = t.keyID()

	privateKey, err := t.privateKey()
	if err != nil {
		return err
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return err
	}

	// Generate encoded token and send it as response.
	tokenStr, err := token.SignedString(key)
	if err != nil {
		return err
	}

	values := url.Values{}
	values.Add("content-type", "application/json")
	values.Add("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	values.Add("subject_token", tokenStr)
	values.Add("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	pure1URL := fmt.Sprintf("%s/oauth2/%s/token", t.api(), "1.0")
	req, err := http.NewRequest(http.MethodPost, pure1URL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := validateResponse(resp); err != nil {
		return err
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	pToken := &pureToken{}
	if err := json.Unmarshal(responseBody, pToken); err != nil {
		return err
	}

	t.token = pToken
	return nil
}

func (t *Array) client() (*pure1.Client, error) {
	restVersion := ""
	clientID := t.clientID()
	privateKey, err := t.privateKey()
	if err != nil {
		return nil, err
	}
	pureCli, err := pure1.NewClient(clientID, privateKey, restVersion)
	if err != nil {
		return nil, err
	}
	return pureCli, nil
}

func (c *Array) Do(req *http.Request, v interface{}, reestablishSession bool) (*http.Response, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
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
	if len(opt.Mappings) == 0 && opt.HostName == "" && opt.HostGroupName == "" {
		return fmt.Errorf("--mapping, --host or --hostgroup is required")
	}
	if len(opt.Mappings) > 0 && opt.HostName != "" {
		return fmt.Errorf("--mapping and --host are mutually exclusive")
	}
	if len(opt.Mappings) > 0 && opt.HostGroupName != "" {
		return fmt.Errorf("--mapping and --hostgroup are mutually exclusive")
	}
	if opt.HostName != "" && opt.HostGroupName != "" {
		return fmt.Errorf("--host and --hostgroup are mutually exclusive")
	}
	return nil
}

func validateOptVolume(opt OptVolume) error {
	if opt.Name == "" && opt.ID == "" && opt.Serial == "" {
		return fmt.Errorf("--name, --id or --serial is required")
	}
	if opt.Name != "" && opt.ID != "" {
		return fmt.Errorf("--name and --id are mutually exclusive")
	}
	if opt.Name != "" && opt.Serial != "" {
		return fmt.Errorf("--name and --serial are mutually exclusive")
	}
	if opt.ID != "" && opt.Serial != "" {
		return fmt.Errorf("--serial and --id are mutually exclusive")
	}
	return nil
}

func (t *Array) ResizeDisk(opt OptResizeDisk) (pureVolume, error) {
	if err := validateOptVolume(opt.Volume); err != nil {
		return pureVolume{}, err
	}
	if opt.Size == "" {
		return pureVolume{}, fmt.Errorf("--size is required")
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
		return pureVolume{}, err
	}
	volume, err := t.getVolume(opt.Volume)
	if err != nil {
		return pureVolume{}, err
	}
	if method != ResizeExact {
		switch method {
		case ResizeUp:
			sizeBytes = volume.Provisioned + sizeBytes
		case ResizeDown:
			sizeBytes = volume.Provisioned - sizeBytes
		}
	}
	params := map[string]string{
		"ids": volume.ID,
	}
	if opt.Truncate {
		params["truncate"] = "true"
	}
	data := map[string]string{
		"provisioned": fmt.Sprint(sizeBytes),
	}
	req, err := t.newRequest(http.MethodPatch, "/volumes", params, data)
	if err != nil {
		return pureVolume{}, err
	}
	var responseData pureResponseVolumes
	if _, err := t.Do(req, &responseData, true); err != nil {
		return pureVolume{}, err
	}
	if len(responseData.Items) == 0 {
		return pureVolume{}, fmt.Errorf("no item in response")
	}
	return responseData.Items[0], nil
}

func (t pureVolume) WWN() string {
	return WWIDPrefix + strings.ToLower(t.Serial)
}

func (t *Array) AddDisk(opt OptAddDisk) (array.Disk, error) {
	var disk array.Disk
	driverData := make(map[string]any)
	volume, err := t.addVolume(opt.Name, opt.Size)
	if err != nil {
		return disk, err
	}
	driverData["volume"] = volume
	disk.DriverData = driverData
	disk.DiskID = volume.WWN()
	disk.DevID = volume.ID
	conns, err := t.MapDisk(OptMapDisk{
		Volume: OptVolume{
			ID: volume.ID,
		},
		Mapping: OptMapping{
			Mappings: opt.Mappings,
			LUN:      opt.LUN,
		},
	})
	if err != nil {
		return disk, err
	}
	driverData["mappings"] = conns
	disk.DriverData = driverData
	return disk, nil
}

func (t *Array) addVolume(name, size string) (pureVolume, error) {
	if name == "" {
		return pureVolume{}, fmt.Errorf("--name is required")
	}
	if size == "" {
		return pureVolume{}, fmt.Errorf("--size is required")
	}
	sizeBytes, err := sizeconv.FromSize(size)
	if err != nil {
		return pureVolume{}, err
	}
	params := map[string]string{
		"names": name,
	}
	data := map[string]string{
		"subtype":     "regular",
		"provisioned": fmt.Sprint(sizeBytes),
	}
	req, err := t.newRequest(http.MethodPost, "/volumes", params, data)
	if err != nil {
		return pureVolume{}, err
	}
	var responseData pureResponseVolumes
	if _, err := t.Do(req, &responseData, true); err != nil {
		return pureVolume{}, err
	}
	if len(responseData.Items) == 0 {
		return pureVolume{}, fmt.Errorf("no volume item in response")
	}
	return responseData.Items[0], nil
}

func (t *Array) getHostName(hbaID string) (string, error) {
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
	m := make(map[string][]string)

	parsedMappings, err := array.ParseMappings(mappings)
	if err != nil {
		return m, err
	}

	for _, mapping := range parsedMappings {
		wwn, err := formatWWN(mapping.HBAID)
		if err != nil {
			return nil, err
		}
		hostName, err := t.getHostName(wwn)
		if err != nil {
			return nil, err
		}
		wwn, err = formatWWN(mapping.TGTID)
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
	return m, nil
}

func (t *Array) mapVolume(volumeName, hostName, hostGroupName string, lun int) (pureVolumeConnection, error) {
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
	req, err := t.newRequest(http.MethodPost, "/connections", params, nil)
	if err != nil {
		return pureVolumeConnection{}, err
	}
	var responseData pureResponseVolumeConnections
	_, err = t.Do(req, &responseData, true)
	if err != nil {
		return pureVolumeConnection{}, err
	}
	if len(responseData.Items) == 0 {
		return pureVolumeConnection{}, fmt.Errorf("no connection item in response")
	}
	return responseData.Items[0], nil
}

func (t *Array) deleteAllVolumeConnections(volumeName string) ([]pureVolumeConnection, error) {
	conns, err := t.deleteHostGroupVolumeConnections(volumeName)
	if err != nil {
		return conns, err
	}
	_, err = t.deleteHostVolumeConnections(volumeName)
	if err != nil {
		return conns, err
	}
	return conns, nil
}

func (t *Array) deleteHostGroupVolumeConnections(volumeName string) ([]pureVolumeConnection, error) {
	opt := OptGetItems{
		Filter: fmt.Sprintf("volume.name='%s'", volumeName),
	}
	conns, err := t.GetConnections(opt)
	if err != nil {
		return []pureVolumeConnection{}, nil
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
		_, err = t.Do(req, &responseData, true)
		if err != nil {
			return conns, err
		}
	}
	return conns, nil
}

func (t *Array) deleteHostVolumeConnections(volumeName string) ([]pureVolumeConnection, error) {
	opt := OptGetItems{
		Filter: fmt.Sprintf("volume.name='%s'", volumeName),
	}
	conns, err := t.GetConnections(opt)
	if err != nil {
		return []pureVolumeConnection{}, nil
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
		_, err = t.Do(req, &responseData, true)
		if err != nil {
			return conns, err
		}
	}
	return conns, nil
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
	_, err = t.Do(req, &responseData, true)
	if err != nil {
		return err
	}
	return nil
}

func (t *Array) UnmapDisk(opt OptUnmapDisk) ([]pureVolumeConnection, error) {
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
	ConnectionsDeleted := make([]pureVolumeConnection, 0)
	hostGroupsDeleted := make(map[string]any)
	switch {
	case len(mappings) > 0:
		hosts, err := t.getHostsFromMappings(opt.Mapping.Mappings)
		if err != nil {
			return nil, err
		}
		for hostName, _ := range hosts {
			queryOpt := OptGetItems{
				Filter: fmt.Sprintf("volume.name='%s' and host.name='%s'", volume.Name, hostName),
			}
			conns, err := t.GetConnections(queryOpt)
			if err != nil {
				return nil, err
			}
			if len(conns) < 1 {
				return nil, fmt.Errorf("connection not found: volume.name='%s' and host.name='%s'", volume.Name, hostName)
			} else if len(conns) > 1 {
				return nil, fmt.Errorf("too many connections found: %d matches with filter volume.name='%s' and host.name='%s'", len(conns), volume.Name, hostName)
			}
			if err := t.unmapVolume(volume.Name, hostName, ""); err != nil {
				return nil, err
			}
			ConnectionsDeleted = append(ConnectionsDeleted, conns[0])
		}
	default:
		queryOpt := OptGetItems{
			Filter: fmt.Sprintf("volume.name='%s'", volume.Name),
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
				if err := t.unmapVolume(volume.Name, opt.Mapping.HostName, ""); err != nil {
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
				} else if err := t.unmapVolume(volume.Name, "", opt.Mapping.HostGroupName); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
					hostGroupsDeleted[opt.Mapping.HostGroupName] = nil
				}
			case conn.HostGroup.Name != "":
				if _, ok := hostGroupsDeleted[conn.HostGroup.Name]; ok {
					continue
				} else if err := t.unmapVolume(volume.Name, "", conn.HostGroup.Name); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
					hostGroupsDeleted[conn.HostGroup.Name] = nil
				}
			case conn.Host.Name != "":
				if err := t.unmapVolume(volume.Name, conn.Host.Name, ""); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
				}
			}
		}
	}
	return ConnectionsDeleted, nil
}

func (t *Array) MapDisk(opt OptMapDisk) (any, error) {
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
	ConnectionsAdded := make([]pureVolumeConnection, 0)
	switch {
	case len(opt.Mapping.Mappings) > 0:
		hosts, err := t.getHostsFromMappings(opt.Mapping.Mappings)
		if err != nil {
			return nil, err
		}
		for hostName, _ := range hosts {
			if conn, err := t.mapVolume(volume.Name, hostName, "", opt.Mapping.LUN); err != nil {
				return nil, err
			} else {
				ConnectionsAdded = append(ConnectionsAdded, conn)
			}
		}
	case opt.Mapping.HostName != "":
		if conn, err := t.mapVolume(volume.Name, opt.Mapping.HostName, "", opt.Mapping.LUN); err != nil {
			return ConnectionsAdded, err
		} else {
			ConnectionsAdded = append(ConnectionsAdded, conn)
		}
	case opt.Mapping.HostGroupName != "":
		if conn, err := t.mapVolume(volume.Name, "", opt.Mapping.HostGroupName, opt.Mapping.LUN); err != nil {
			return ConnectionsAdded, err
		} else {
			ConnectionsAdded = append(ConnectionsAdded, conn)
		}
	}
	return ConnectionsAdded, nil
}

func (t *Array) getVolume(opt OptVolume) (pureVolume, error) {
	var (
		volume   pureVolume
		items    []pureVolume
		err      error
		queryOpt OptGetItems
	)
	if opt.ID != "" {
		queryOpt.Filter = fmt.Sprintf("id='%s'", opt.ID)
	} else if opt.Name != "" {
		queryOpt.Filter = fmt.Sprintf("name='%s'", opt.Name)
	} else if opt.Serial != "" {
		queryOpt.Filter = fmt.Sprintf("serial='%s'", opt.Serial)
	} else {
		return volume, fmt.Errorf("id, name and serial are empty. refuse to get all volumes")
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

	volume, err := t.getVolume(opt.Volume)
	if err != nil {
		return disk, err
	}
	disk.DiskID = volume.WWN()
	disk.DevID = volume.ID
	driverData := make(map[string]any)
	driverData["volume"] = volume
	disk.DriverData = driverData

	conns, err := t.deleteAllVolumeConnections(volume.Name)
	if err != nil {
		return disk, err
	}
	driverData["mappings"] = conns
	disk.DriverData = driverData

	volume, err = t.delVolume(opt)
	if err != nil {
		return disk, err
	}
	driverData["volume"] = volume
	disk.DriverData = driverData

	return disk, nil
}

func (t *Array) delVolume(opt OptDelDisk) (pureVolume, error) {
	if err := validateOptVolume(opt.Volume); err != nil {
		return pureVolume{}, err
	}
	volume, err := t.getVolume(opt.Volume)
	if err != nil {
		return pureVolume{}, err
	}

	var item pureVolume
	params := map[string]string{
		"ids": volume.ID,
	}
	if !volume.Destroyed {
		data := map[string]any{
			"destroyed": true,
		}
		req, err := t.newRequest(http.MethodPatch, "/volumes", params, data)
		if err != nil {
			return pureVolume{}, err
		}
		var responseData pureResponseVolumes
		_, err = t.Do(req, &responseData, true)
		if err != nil {
			return pureVolume{}, err
		}
		if len(responseData.Items) == 0 {
			return pureVolume{}, fmt.Errorf("no item in response")
		}
		item = responseData.Items[0]
	} else {
		item = volume
	}
	if opt.Now {
		req, err := t.newRequest(http.MethodDelete, "/volumes", params, nil)
		if err != nil {
			return pureVolume{}, err
		}
		var responseData pureResponseVolumes
		_, err = t.Do(req, &responseData, true)
		if err != nil {
			return item, err
		}
	}
	// TODO: del diskinfo
	return item, nil
}

func (t *Array) GetHosts(opt OptGetItems) ([]pureHost, error) {
	params := getParams(opt.Filter)
	l, err := t.doGet("GET", "/hosts", params, nil)
	if err != nil {
		return nil, err
	}
	hosts := make([]pureHost, len(l))
	for i, item := range l {
		var host pureHost
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &host)
		hosts[i] = host
	}
	return hosts, nil
}

func (t *Array) GetConnections(opt OptGetItems) ([]pureVolumeConnection, error) {
	params := getParams(opt.Filter)
	l, err := t.doGet("GET", "/connections", params, nil)
	if err != nil {
		return nil, err
	}
	conns := make([]pureVolumeConnection, len(l))
	for i, item := range l {
		var conn pureVolumeConnection
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &conn)
		conns[i] = conn
	}
	return conns, nil
}

func (t *Array) GetArrays(opt OptGetItems) ([]pureArray, error) {
	params := getParams(opt.Filter)
	l, err := t.doGet("GET", "/arrays", params, nil)
	if err != nil {
		return nil, err
	}
	arrays := make([]pureArray, len(l))
	for i, item := range l {
		var array pureArray
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &array)
		arrays[i] = array
	}
	return arrays, nil
}

func (t *Array) GetVolumes(opt OptGetItems) ([]pureVolume, error) {
	params := getParams(opt.Filter)
	l, err := t.doGet("GET", "/volumes", params, nil)
	if err != nil {
		return nil, err
	}
	volumes := make([]pureVolume, len(l))
	for i, item := range l {
		var volume pureVolume
		b, _ := json.Marshal(item)
		json.Unmarshal(b, &volume)
		volumes[i] = volume
	}
	return volumes, nil
}

func (t *Array) GetVolumeGroups(opt OptGetItems) (any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/volume-groups", params, nil)
}

func (t *Array) GetControllers(opt OptGetItems) (any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/controllers", params, nil)
}

func (t *Array) GetDrives(opt OptGetItems) (any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/drives", params, nil)
}

func (t *Array) GetPods(opt OptGetItems) (any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/pods", params, nil)
}

func (t *Array) GetPorts(opt OptGetItems) (any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/ports", params, nil)
}

func (t *Array) GetNetworkInterfaces(opt OptGetItems) ([]pureNetworkInterface, error) {
	params := getParams(opt.Filter)
	l, err := t.doGet("GET", "/network-interfaces", params, nil)
	if err != nil {
		return nil, err
	}
	items := make([]pureNetworkInterface, len(l))
	for i, e := range l {
		var item pureNetworkInterface
		b, _ := json.Marshal(e)
		json.Unmarshal(b, &item)
		items[i] = item
	}
	return items, nil
}

func (t *Array) GetHostGroups(opt OptGetItems) (any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/host-groups", params, nil)
}

func getParams(filter string) map[string]string {
	params := map[string]string{"total_item_count": "true", "limit": fmt.Sprint(ItemsPerPage)}
	if filter != "" {
		params["filter"] = filter
	}
	return params
}

func (t *Array) GetHardware(opt OptGetItems) ([]any, error) {
	params := getParams(opt.Filter)
	return t.doGet("GET", "/hardware", params, nil)
}

func (t *Array) doGet(method string, path string, params map[string]string, data interface{}) ([]any, error) {
	req, err := t.newRequest(method, path, params, data)
	if err != nil {
		return nil, err
	}
	var r pureResponse
	items := make([]any, 0)
	_, err = t.Do(req, &r, true)
	if err != nil {
		return nil, err
	}
	for len(items) < r.TotalItems {
		for _, item := range r.Items {
			//i := PureArray{}
			//s, _ := json.Marshal(item)
			//json.Unmarshal([]byte(s), &i)
			items = append(items, item)
		}

		if len(items) < r.TotalItems {
			if r.ContinuationToken != nil {
				if params == nil {
					params = map[string]string{"continuation_token": r.ContinuationToken.(string)}
				} else {
					params["continuation_token"] = r.ContinuationToken.(string)
				}
				req, err := t.newRequest(method, path, params, data)
				if err != nil {
					return nil, err
				}

				_, err = t.Do(req, r, false)
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
	req.Header.Add("Authorization", `Bearer `+token.AccessToken)

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

	bodyBytes, err := io.ReadAll(r.Body)

	if err != nil {
		return err
	}

	if len(bodyBytes) == 0 {
		return nil
	}

	return json.Unmarshal(bodyBytes, &v)
}

// validateResponse checks that the http response is within the 200 range.
// Some functionality needs to be added here to check for some specific errors,
// and probably add the equivalents to PureError and PureHTTPError from the Python
// REST client.
func validateResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	return fmt.Errorf("Response code: %d, Response body: %s", r.StatusCode, bodyString)
}
