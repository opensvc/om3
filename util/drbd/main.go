package drbd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

type (
	T struct {
		res string
		log *plog.Logger
	}

	Config struct {
		XMLName   xml.Name         `xml:"config"`
		File      string           `xml:"config,attr"`
		Common    ConfigCommon     `xml:"common"`
		Resources []ConfigResource `xml:"resource"`
	}
	ConfigCommon struct {
		XMLName xml.Name `xml:"common"`
	}
	ConfigResource struct {
		XMLName    xml.Name         `xml:"resource"`
		Name       string           `xml:"name,attr"`
		FileLine   string           `xml:"conf-file-line"`
		Hosts      []ConfigHost     `xml:"host"`
		Connection ConfigConnection `xml:"connection"`
	}
	ConfigConnection struct {
		XMLName xml.Name               `xml:"connection"`
		Hosts   []ConfigConnectionHost `xml:"host"`
	}
	ConfigConnectionHost struct {
		XMLName xml.Name      `xml:"host"`
		Name    string        `xml:"name,attr"`
		Address ConfigAddress `xml:"address"`
	}
	ConfigAddress struct {
		XMLName xml.Name `xml:"address"`
		Family  string   `xml:"family,attr"`
		Port    string   `xml:"port,attr"`
		IP      string   `xml:",chardata"`
	}
	ConfigVolume struct {
		Name     string       `xml:"vnr,attr"`
		Device   ConfigDevice `xml:"device"`
		Disk     string       `xml:"disk"`
		MetaDisk string       `xml:"meta-disk"`
	}
	ConfigDevice struct {
		Path  string `xml:",chardata"`
		Minor string `xml:"minor,attr"`
	}
	ConfigHost struct {
		Name    string         `xml:"name,attr"`
		Volumes []ConfigVolume `xml:"volume"`
		Address ConfigAddress  `xml:"address"`
	}
	Digest struct {
		Ports  map[string]any
		Minors map[string]any
	}
)

var (
	KeyResource       = "resource "
	KeyConnectionMesh = "connection-mesh "
	KeyHosts          = "hosts "
	KeyOn             = "on "
	KeyNodeID         = "node-id "
	KeyDevice         = "device "
	KeyDisk           = "disk "
	KeyMetaDisk       = "meta-disk "
	KeyAddress        = "address "
	KeyVolume         = "volume "

	KeyResourceLen       = len(KeyResource)
	KeyConnectionMeshLen = len(KeyConnectionMesh)
	KeyHostsLen          = len(KeyHosts)
	KeyOnLen             = len(KeyOn)
	KeyNodeIDLen         = len(KeyNodeID)
	KeyDeviceLen         = len(KeyDevice)
	KeyDiskLen           = len(KeyDisk)
	KeyMetaDiskLen       = len(KeyMetaDisk)
	KeyAddressLen        = len(KeyAddress)
	KeyVolumeLen         = len(KeyVolume)

	RetryDelay   = time.Second * 1
	RetryTimeout = time.Second * 10

	ExitCodeDeviceInUse = 11

	isModProbed = false

	MaxDRBD = 512
	MinPort = 7289
	MaxPort = 7489
)

func New(res string, opts ...funcopt.O) *T {
	t := T{
		res: res,
	}
	_ = funcopt.Apply(&t, opts...)
	return &t
}
func WithLogger(log *plog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func ResConfigFile(res string) string {
	return fmt.Sprintf("/etc/drbd.d/%s.res", res)
}

func Dump() ([]byte, error) {
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithVarArgs("dump-xml"),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	return cmd.Output()
}

func ParseConfig(b []byte) (*Config, error) {
	data := new(Config)
	err := xml.Unmarshal(b, data)
	return data, err
}

func GetConfig() (*Config, error) {
	if b, err := Dump(); err != nil {
		return nil, err
	} else {
		return ParseConfig(b)
	}
}

func (t Config) GetResource(name string) (ConfigResource, bool) {
	for _, resource := range t.Resources {
		if resource.Name == name {
			return resource, true
		}
	}
	return ConfigResource{}, false
}

func (t ConfigResource) GetHost(name string) (ConfigHost, bool) {
	for _, host := range t.Hosts {
		if host.Name == name {
			return host, true
		}
	}
	return ConfigHost{}, false
}

func (t ConfigHost) GetVolume(name string) (ConfigVolume, bool) {
	for _, volume := range t.Volumes {
		if volume.Name == name {
			return volume, true
		}
	}
	return ConfigVolume{}, false
}

func (t Config) GetMinors() map[string]any {
	m := make(map[string]any)
	for _, resource := range t.Resources {
		for _, host := range resource.Hosts {
			for _, volume := range host.Volumes {
				m[volume.Device.Minor] = nil
			}
		}
	}
	return m
}

func (t Config) GetPorts() map[string]any {
	m := make(map[string]any)
	for _, resource := range t.Resources {
		for _, host := range resource.Hosts {
			m[host.Address.Port] = nil
		}
	}
	return m
}

func ParseDigest(b []byte) (Digest, error) {
	digest := Digest{}
	config, err := ParseConfig(b)
	if err != nil {
		return digest, err
	}
	digest.Ports = config.GetPorts()
	digest.Minors = config.GetMinors()
	return digest, nil
}

func GetDigest() (Digest, error) {
	if b, err := Dump(); err != nil {
		return Digest{}, err
	} else {
		return ParseDigest(b)
	}
}

func (t T) Primary() error {
	args := []string{"primary", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) PrimaryForce() error {
	args := []string{"primary", t.res, "--force"}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Secondary() error {
	args := []string{"secondary", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Adjust() error {
	args := []string{"adjust", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Connect() error {
	args := []string{"connect", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Disconnect() error {
	args := []string{"disconnect", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Attach() error {
	args := []string{"attach", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) DetachForce() error {
	args := []string{"detach", t.res, "--force"}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Down() error {
	args := []string{"down", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) Up() error {
	args := []string{"up", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return retry(cmd)
}

func (t T) CreateMD(maxPeers int) error {
	args := []string{"create-md", "--force", "--max-peers", fmt.Sprint(maxPeers), t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t T) HasMD() (bool, error) {
	hasMeta := true
	args := []string{"--", "--force", "dump-md", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithOnStderrLine(func(s string) {
			if strings.Contains(s, "No valid meta data found") {
				hasMeta = false
			}
		}),
	)
	err := cmd.Run()
	if !hasMeta {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (t T) Role() (string, error) {
	args := []string{"role", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithBufferedStdout(),
	)
	if b, err := cmd.Output(); err != nil {
		return "", err
	} else {
		s := strings.TrimSpace(string(b))
		switch s {
		case "Primary", "Secondary":
			// drbd9
			return s, nil
		default:
			// drbd8
			l := strings.Split(s, "/")
			if len(l) != 2 {
				return s, fmt.Errorf("unexpected role: %s", s)
			}
			// the second element was the remote role.
			return l[0], nil
		}
	}
}

func (t T) ConnState() (string, error) {
	isAttached := true
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithVarArgs("cstate", t.res),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(func(s string) {
			if strings.Contains(s, "Device minor not allocated") {
				isAttached = false
			}
		}),
	)
	if err := cmd.Run(); err != nil {
		if !isAttached || cmd.ExitCode() == 10 {
			return "Unattached", nil
		} else {
			return "", err
		}
	}
	b := bytes.Split(cmd.Stdout(), []byte("\n"))[0]
	return string(b), nil
}

func (t T) DiskStates() ([]string, error) {
	args := []string{"dstate", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		return []string{}, err
	}
	s := strings.TrimSpace(string(cmd.Stdout()))
	return strings.Split(s, "/"), nil
}

func (t T) Remove() error {
	return nil
}

func (t T) IsUp() (bool, string, error) {
	return false, "", fmt.Errorf("todo")
}

func (t T) IsDefined() (bool, error) {
	isDefined := true
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithVarArgs("status", t.res),
		command.WithOnStderrLine(func(s string) {
			if strings.Contains(s, "no resources defined") {
				isDefined = false
			}
			if strings.Contains(s, "not defined") {
				isDefined = false
			}
		}),
	)
	err := cmd.Run()
	if !isDefined {
		return false, nil
	}
	if cmd.ExitCode() == 10 {
		// no such resource
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// WipeMD executes the `wipe-md` drbd command.
//
// Ignore the exit code `20`:
//
//	Returned when the sub dev is not found.
//	No need to fail, as the sub dev is surely flagged for unprovision too,
//	which will wipe metadata.
//	This situation happens on unprovision on a stopped instance, when drbd
//	is stacked over another (stopped) disk resource.
func (t T) WipeMD() error {
	args := []string{"--", "--force", "wipe-md", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithOnStderrLine(func(s string) {
			if s == "" {
				return
			}
			if strings.HasPrefix(s, "Do you ") {
				return
			}
			if strings.HasPrefix(s, "***") {
				return
			}
			t.log.Errorf(s)
		}),
		command.WithIgnoredExitCodes(0, 20),
	)
	cmd.Cmd().Stdin = strings.NewReader("yes\n")
	return cmd.Run()
}

func (t T) validateName() error {
	if t.res == "" {
		return fmt.Errorf("name is required")
	}
	if len(t.res) > 32 {
		return fmt.Errorf("device drbd res name is too long, 32 chars max (res name is %s)", t.res)
	}
	return nil
}

func (t T) devpathFromName() string {
	return "/dev/drbd/by-res/" + t.res + "/0"
}

func (t *T) Create(disk string, addr string, port int) error {
	return fmt.Errorf("todo")
}

func retry(cmd *command.T) error {
	limit := time.Now().Add(RetryTimeout)
	for {
		err := cmd.Run()
		if err == nil {
			return nil
		}
		if cmd.ExitCode() != ExitCodeDeviceInUse {
			return err
		}
		if time.Now().Add(RetryDelay).After(limit) {
			return err
		}
		time.Sleep(RetryDelay)
	}
}

func (t T) ModProbe() error {
	if isModProbed {
		return nil
	}
	if file.Exists("/proc/drbd") {
		isModProbed = true
		return nil
	}
	cmd := command.New(
		command.WithName("modprobe"),
		command.WithArgs([]string{"drbd"}),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	err := cmd.Run()
	if err != nil {
		return err
	}
	isModProbed = true
	return nil
}

func intsContains(l []int, i int) bool {
	for _, v := range l {
		if v == i {
			return true
		}
	}
	return false
}

func (t Digest) FreeMinor(exclude []int) (int, error) {
	if exclude == nil {
		exclude = []int{}
	}
	for i := 0; i < MaxDRBD; i++ {
		s := fmt.Sprint(i)
		if _, ok := t.Minors[s]; ok {
			continue
		} else if intsContains(exclude, i) {
			continue
		} else {
			return i, nil
		}
	}
	return 0, fmt.Errorf("no free minor")
}

func (t Digest) FreePort(exclude []int) (int, error) {
	if exclude == nil {
		exclude = []int{}
	}
	for i := MinPort; i < MaxPort; i++ {
		s := fmt.Sprint(i)
		if _, ok := t.Ports[s]; ok {
			continue
		} else if intsContains(exclude, i) {
			continue
		} else {
			return i, nil
		}
	}
	return 0, fmt.Errorf("no free port")
}
