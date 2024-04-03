package arraypure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devans10/pugo/pure1"
	"github.com/golang-jwt/jwt"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/sizeconv"
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

	Array struct {
		*array.Array
		token *pureToken
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
				if data, err := t.resizeDisk(id, name, serial, size, truncate); err != nil {
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
				if data, err := t.unmapDisk(id, name, serial, mappings, host, hostGroup); err != nil {
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
				if data, err := t.mapDisk(id, name, serial, mappings, host, hostGroup, lun); err != nil {
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
				if data, err := t.delDisk(id, name, serial, now); err != nil {
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
			Short: "add a zvol-type dataset and map",
			RunE: func(cmd *cobra.Command, _ []string) error {
				if data, err := t.addDisk(name, size, mappings); err != nil {
					return err
				} else {
					return dump(data)
				}
			},
		}
		useFlagName(cmd)
		useFlagSize(cmd)
		useFlagMapping(cmd)
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
				data, err := t.getHosts(filter)
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
				data, err := t.getConnections(filter)
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
				data, err := t.getVolumes(filter)
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
				data, err := t.getControllers(filter)
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
				data, err := t.getDrives(filter)
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
				data, err := t.getPods(filter)
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
				data, err := t.getPorts(filter)
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
				data, err := t.getNetworkInterfaces(filter)
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
				data, err := t.getVolumeGroups(filter)
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
				data, err := t.getHostGroups(filter)
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
				data, err := t.getArrays(filter)
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
				data, err := t.getHardware(filter)
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
	return object.NewSec(s, object.WithVolatile(true))
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

	responseBody, err := ioutil.ReadAll(resp.Body)
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

func validateHostIdentifiers(mappings []string, host, hostGroup string) error {
	if len(mappings) == 0 && host == "" && hostGroup == "" {
		return fmt.Errorf("--mapping, --host or --hostgroup is required")
	}
	if len(mappings) > 0 && host != "" {
		return fmt.Errorf("--mapping and --host are mutually exclusive")
	}
	if len(mappings) > 0 && hostGroup != "" {
		return fmt.Errorf("--mapping and --hostgroup are mutually exclusive")
	}
	if host != "" && hostGroup != "" {
		return fmt.Errorf("--host and --hostgroup are mutually exclusive")
	}
	return nil
}

func validateVolumeIdentifiers(id, name, serial string) error {
	if name == "" && id == "" && serial == "" {
		return fmt.Errorf("--name, --id or --serial is required")
	}
	if name != "" && id != "" {
		return fmt.Errorf("--name and --id are mutually exclusive")
	}
	if name != "" && serial != "" {
		return fmt.Errorf("--name and --serial are mutually exclusive")
	}
	if id != "" && serial != "" {
		return fmt.Errorf("--serial and --id are mutually exclusive")
	}
	return nil
}

func (t *Array) resizeDisk(id, name, serial, size string, truncate bool) (*pureVolume, error) {
	if err := validateVolumeIdentifiers(id, name, serial); err != nil {
		return nil, err
	}
	if size == "" {
		return nil, fmt.Errorf("--size is required")
	}
	var method resizeMethod
	if len(size) > 1 {
		switch size[0] {
		case '+':
			size = size[1:]
			method = ResizeUp
		case '-':
			size = size[1:]
			method = ResizeDown
		}
	}
	sizeBytes, err := sizeconv.FromSize(size)
	if err != nil {
		return nil, err
	}
	volume, err := t.getVolume(id, name, serial)
	if err != nil {
		return nil, err
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
	if truncate {
		params["truncate"] = "true"
	}
	data := map[string]string{
		"provisioned": fmt.Sprint(sizeBytes),
	}
	req, err := t.newRequest(http.MethodPatch, "/volumes", params, data)
	if err != nil {
		return nil, err
	}
	var responseData pureResponseVolumes
	if _, err := t.Do(req, &responseData, true); err != nil {
		return nil, err
	}
	if len(responseData.Items) == 0 {
		return nil, fmt.Errorf("no item in response")
	}
	return &responseData.Items[0], nil
}

func (t *Array) addDisk(name, size string, mappings []string) (*pureVolume, error) {
	if name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if size == "" {
		return nil, fmt.Errorf("--size is required")
	}
	sizeBytes, err := sizeconv.FromSize(size)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	var responseData pureResponseVolumes
	if _, err := t.Do(req, &responseData, true); err != nil {
		return nil, err
	}
	if len(responseData.Items) == 0 {
		return nil, fmt.Errorf("no item in response")
	}
	return &responseData.Items[0], nil
}

func (t *Array) getHostGroupName(hbaID string) (string, error) {
	filter := fmt.Sprintf("wwns='%s'", hbaID)
	hosts, err := t.getHosts(filter)
	if err != nil {
		return "", err
	}
	if len(hosts) == 0 {
		return "", fmt.Errorf("no host found for hba id %s", hbaID)
	}
	if len(hosts) > 1 {
		return "", fmt.Errorf("too many hosts found for hba id %s", hbaID)
	}
	for _, host := range hosts {
		if !host.IsLocal {
			continue
		}
		if host.HostGroup.Name == "" {
			continue
		}
		return host.HostGroup.Name, nil
	}
	return "", fmt.Errorf("hba id %s not found in any hostgroup", hbaID)
}

func (t *Array) getHostName(hbaID string) (string, error) {
	filter := fmt.Sprintf("wwns='%s'", hbaID)
	hosts, err := t.getHosts(filter)
	if err != nil {
		return "", err
	}
	if len(hosts) == 0 {
		return "", fmt.Errorf("no host found for hba id %s", hbaID)
	}
	if len(hosts) > 1 {
		return "", fmt.Errorf("too many hosts found for hba id %s", hbaID)
	}
	return hosts[0].Name, nil
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
			filter := fmt.Sprintf("fc.wwn='%s' and services='scsi-fc' and enabled='true'", wwn)
			networkInterfaces, err := t.getNetworkInterfaces(filter)
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
}

func (t *Array) getHostGroupsFromMappings(mappings []string) (map[string][]string, error) {
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
		hostGroupName, err := t.getHostGroupName(wwn)
		if err != nil {
			return nil, err
		}
		for _, target := range targets {
			wwn, err := formatWWN(target)
			if err != nil {
				return nil, err
			}
			filter := fmt.Sprintf("fc.wwn='%s' and services='scsi-fc' and enabled='true'", wwn)
			networkInterfaces, err := t.getNetworkInterfaces(filter)
			if err != nil {
				return nil, err
			}
			if len(networkInterfaces) == 0 {
				continue
			}
			if v, ok := m[hostGroupName]; ok {
				m[hostGroupName] = append(v, wwn)
			} else {
				m[hostGroupName] = []string{wwn}
			}
		}
	}
	return m, nil
}

func (t *Array) unmap(volumeName, hostName, hostGroupName string) error {
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

func (t *Array) unmapDisk(id, name, serial string, mappings []string, hostName, hostGroupName string) ([]pureVolumeConnection, error) {
	if err := validateVolumeIdentifiers(id, name, serial); err != nil {
		return nil, err
	}
	if err := validateHostIdentifiers(mappings, hostName, hostGroupName); err != nil {
		return nil, err
	}
	volume, err := t.getVolume(id, name, serial)
	if err != nil {
		return nil, err
	}
	ConnectionsDeleted := make([]pureVolumeConnection, 0)
	hostGroupsDeleted := make(map[string]any)
	switch {
	case len(mappings) > 0:
		hosts, err := t.getHostsFromMappings(mappings)
		if err != nil {
			return nil, err
		}
		for hostName, _ := range hosts {
			filter := fmt.Sprintf("volume.name='%s' and host.name='%s'", volume.Name, hostName)
			conns, err := t.getConnections(filter)
			if err != nil {
				return nil, err
			}
			if len(conns) < 1 {
				return nil, fmt.Errorf("connection not found: volume.name='%s' and host.name='%s'", volume.Name, hostName)
			} else if len(conns) > 1 {
				return nil, fmt.Errorf("too many connections found: %d matches with filter volume.name='%s' and host.name='%s'", len(conns), volume.Name, hostName)
			}
			if err := t.unmap(volume.Name, hostName, ""); err != nil {
				return nil, err
			}
			ConnectionsDeleted = append(ConnectionsDeleted, conns[0])
		}
	default:
		filter := fmt.Sprintf("volume.name='%s'", volume.Name)
		conns, err := t.getConnections(filter)
		if err != nil {
			return nil, err
		}
		for _, conn := range conns {
			switch {
			case hostName != "":
				if conn.Host.Name != hostName {
					continue
				}
				if err := t.unmap(volume.Name, hostName, ""); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
				}
			case hostGroupName != "":
				if conn.HostGroup.Name != hostGroupName {
					continue
				}
				if _, ok := hostGroupsDeleted[hostGroupName]; ok {
					continue
				} else if err := t.unmap(volume.Name, "", hostGroupName); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
					hostGroupsDeleted[hostGroupName] = nil
				}
			case conn.HostGroup.Name != "":
				if _, ok := hostGroupsDeleted[conn.HostGroup.Name]; ok {
					continue
				} else if err := t.unmap(volume.Name, "", conn.HostGroup.Name); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
					hostGroupsDeleted[hostGroupName] = nil
				}
			case conn.Host.Name != "":
				if err := t.unmap(volume.Name, conn.Host.Name, ""); err != nil {
					return ConnectionsDeleted, err
				} else {
					ConnectionsDeleted = append(ConnectionsDeleted, conn)
				}
			}
		}
	}
	return ConnectionsDeleted, nil
}

func (t *Array) mapDisk(id, name, serial string, mappings []string, host, hostGroup string, lun int) (any, error) {
	return nil, nil
}

func (t *Array) getVolume(id, name, serial string) (pureVolume, error) {
	var (
		volume pureVolume
		items  []pureVolume
		err    error
		filter string
	)
	if id != "" {
		filter = fmt.Sprintf("id='%s'", id)
	} else if name != "" {
		filter = fmt.Sprintf("name='%s'", name)
	} else if serial != "" {
		filter = fmt.Sprintf("serial='%s'", serial)
	} else {
		return volume, fmt.Errorf("id, name and serial are empty. refuse to get all volumes")
	}
	items, err = t.getVolumes(filter)
	if err != nil {
		return volume, err
	}
	if len(items) > 1 {
		return volume, fmt.Errorf("multiple volumes found with name %s", name)
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
	return volume, fmt.Errorf("no volume found with %s", filter)
}

func (t *Array) delDisk(id, name, serial string, now bool) (*pureVolume, error) {
	if err := validateVolumeIdentifiers(id, name, serial); err != nil {
		return nil, err
	}
	volume, err := t.getVolume(id, name, serial)
	if err != nil {
		return nil, err
	}

	//diskID := strings.ToLower(volume.Serial)
	// TODO: delMap

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
			return nil, err
		}
		var responseData pureResponseVolumes
		_, err = t.Do(req, &responseData, true)
		if err != nil {
			return nil, err
		}
		if len(responseData.Items) == 0 {
			return nil, fmt.Errorf("no item in response")
		}
		item = responseData.Items[0]
	} else {
		item = volume
	}
	if now {
		req, err := t.newRequest(http.MethodDelete, "/volumes", params, nil)
		if err != nil {
			return nil, err
		}
		var responseData pureResponseVolumes
		_, err = t.Do(req, &responseData, true)
		if err != nil {
			return &item, err
		}
	}
	// TODO: del diskinfo
	return &item, nil
}

func (t *Array) getHosts(filter string) ([]pureHost, error) {
	params := getParams(filter)
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

func (t *Array) getConnections(filter string) ([]pureVolumeConnection, error) {
	params := getParams(filter)
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

func (t *Array) getVolumes(filter string) ([]pureVolume, error) {
	params := getParams(filter)
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

func (t *Array) getVolumeGroups(filter string) (any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/volume-groups", params, nil)
}

func (t *Array) getControllers(filter string) (any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/controllers", params, nil)
}

func (t *Array) getDrives(filter string) (any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/drives", params, nil)
}

func (t *Array) getPods(filter string) (any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/pods", params, nil)
}

func (t *Array) getPorts(filter string) (any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/ports", params, nil)
}

func (t *Array) getNetworkInterfaces(filter string) ([]any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/network-interfaces", params, nil)
}

func (t *Array) getHostGroups(filter string) (any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/host-groups", params, nil)
}

func getParams(filter string) map[string]string {
	params := map[string]string{"total_item_count": "true", "limit": fmt.Sprint(ItemsPerPage)}
	if filter != "" {
		params["filter"] = filter
	}
	return params
}

func (t *Array) getHardware(filter string) ([]any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/hardware", params, nil)
}

func (t *Array) getArrays(filter string) ([]any, error) {
	params := getParams(filter)
	return t.doGet("GET", "/arrays", params, nil)
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
// and probably add the equivlents to PureError and PureHTTPError from the Python
// REST client.
func validateResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	return fmt.Errorf("Response code: %d, Response body: %s", r.StatusCode, bodyString)
}
