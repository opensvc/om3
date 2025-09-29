package drbd

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/lock"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/waitfor"
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

const (
	ConnStateStandAlone        = "StandAlone"
	ConnStateDisconnecting     = "Disconnecting"
	ConnStateUnconnected       = "Unconnected"
	ConnStateTimeout           = "Timeout"
	ConnStateBrokenPipe        = "BrokenPipe"
	ConnStateNetworkFailure    = "NetworkFailure"
	ConnStateProtocolError     = "ProtocolError"
	ConnStateTearDow           = "TearDown"
	ConnStateConnecting        = "Connecting"
	ConnStateConnected         = "Connected"
	ConnStateLegacycConnecting = "WFConnection"
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

	// waitConnectionStateDelay defines the periodic delay used when polling for
	// connection state changes.
	waitConnectionStateDelay = time.Second * 1

	// waitConnectingOrConnectedTimeout defines the maximum duration to wait for
	// a connection state change to connecting or connected before timing out.
	waitConnectingOrConnectedTimeout = time.Second * 20
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

func (t *T) Primary(ctx context.Context) error {
	args := []string{"primary", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) PrimaryForce(ctx context.Context) error {
	args := []string{"primary", t.res, "--force"}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Secondary(ctx context.Context) error {
	args := []string{"secondary", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Adjust(ctx context.Context) error {
	args := []string{"adjust", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Connect(ctx context.Context) error {
	return t.withLock(ctx, t.connect, "drbdadm connect", time.Second)
}

func (t *T) connect(ctx context.Context) error {
	args := []string{"connect", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Disconnect(ctx context.Context) error {
	args := []string{"disconnect", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Attach(ctx context.Context) error {
	args := []string{"attach", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) DetachForce(ctx context.Context) error {
	args := []string{"detach", t.res, "--force"}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Down(ctx context.Context) error {
	args := []string{"down", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) Up(ctx context.Context) error {
	return t.withLock(ctx, t.up, "drbdadm up", time.Second)
}

func (t *T) up(ctx context.Context) error {
	args := []string{"up", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithContext(ctx),
	)
	return retry(cmd)
}

func (t *T) CreateMD(ctx context.Context, maxPeers int) error {
	args := []string{"create-md", "--force", "--max-peers", fmt.Sprint(maxPeers), t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.InfoLevel),
		command.WithContext(ctx),
	)
	return cmd.Run()
}

func (t *T) HasMD(ctx context.Context) (bool, error) {
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
		command.WithContext(ctx),
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

func (t *T) Role(ctx context.Context) (string, error) {
	args := []string{"role", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithContext(ctx),
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

func (t *T) ConnState(ctx context.Context) (string, error) {
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
		command.WithContext(ctx),
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

func (t *T) DiskStates(ctx context.Context) ([]string, error) {
	args := []string{"dstate", t.res}
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithContext(ctx),
	)
	if err := cmd.Run(); err != nil {
		return []string{}, err
	}
	s := strings.TrimSpace(string(cmd.Stdout()))
	return strings.Split(s, "/"), nil
}

func (t *T) Remove() error {
	return nil
}

func (t *T) IsUp() (bool, string, error) {
	return false, "", fmt.Errorf("todo")
}

func (t *T) IsDefined(ctx context.Context) (bool, error) {
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
		command.WithContext(ctx),
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
func (t *T) WipeMD(ctx context.Context) error {
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
		command.WithContext(ctx),
	)
	cmd.Cmd().Stdin = strings.NewReader("yes\n")
	return cmd.Run()
}

func (t *T) validateName() error {
	if t.res == "" {
		return fmt.Errorf("name is required")
	}
	if len(t.res) > 32 {
		return fmt.Errorf("device drbd res name is too long, 32 chars max (res name is %s)", t.res)
	}
	return nil
}

func (t *T) devpathFromName() string {
	return "/dev/drbd/by-res/" + t.res + "/0"
}

func (t *T) Create(disk string, addr string, port int) error {
	return fmt.Errorf("todo")
}

// startConnection establishes a connection for the DRBD resource, managing its state transitions as needed.
// It returns an error if the connection process fails.
func (t *T) StartConnection(ctx context.Context) error {
	state, err := t.ConnState(ctx)
	if err != nil {
		return err
	}

	t.log.Infof("drbd resource %s cstate %s", t.res, state)
	switch state {
	case ConnStateConnecting, ConnStateConnected, ConnStateLegacycConnecting:
		// the expected state is reached
		return nil
	case ConnStateUnconnected, ConnStateTimeout, ConnStateBrokenPipe, ConnStateNetworkFailure, ConnStateProtocolError, ConnStateTearDow:
		// Temporary state from C_CONNECTED to C_UNCONNECTED
		_, err = t.WaitConnectingOrConnected(ctx)
		return err
	case ConnStateDisconnecting:
		// Temporary state to StandAlone
		t.log.Infof("drbd resource %s: wants cstate StandAlone before restart connection", t.res)
		if _, err := t.WaitCState(ctx, 5*time.Second, ConnStateStandAlone); err != nil {
			return fmt.Errorf("drbd resource %s: waiting for cstate StandAlone: %w", t.res, err)
		}
		_, err = t.ConnectAndWaitConnectingOrConnected(ctx)
		return err
	case ConnStateStandAlone:
		_, err = t.ConnectAndWaitConnectingOrConnected(ctx)
		return err
	default:
		return fmt.Errorf("drbd resource %s cstate %s unexpected while waiting for %s or %s",
			t.res, state, ConnStateConnecting, ConnStateConnected)
	}
}

func (t *T) TryStartConnection(ctx context.Context) error {
	if ok, err := t.IsDefined(ctx); err != nil {
		return err
	} else if !ok {
		t.log.Infof("drbd resource %s is not defined, skipping connection", t.res)
		return nil
	}
	return t.StartConnection(ctx)
}

func (t *T) WaitCState(ctx context.Context, timeout time.Duration, candidates ...string) (string, error) {
	t.log.Infof("wait %s for cstate in (%s)", t.res, strings.Join(candidates, ","))
	var state, lastState string
	ok, err := waitfor.TrueNoErrorCtx(ctx, timeout, waitConnectionStateDelay, func() (bool, error) {
		var err error
		state, err = t.ConnState(ctx)

		if err != nil {
			return false, err
		}

		if slices.Contains(candidates, state) {
			return true, nil
		} else {
			if state != lastState {
				t.log.Infof("wait %s cstate in (%s), found current cstate %s", t.res, strings.Join(candidates, ","), state)
				lastState = state
			}
			return false, nil
		}
	})
	if err != nil {
		return state, fmt.Errorf("wait for %s cstate in (%s): %w",
			t.res, strings.Join(candidates, ","), err)
	} else if !ok {
		return state, fmt.Errorf("wait for %s cstate in (%s): timeout, last state was: %s",
			t.res, strings.Join(candidates, ","), state)
	}
	t.log.Infof("wait for %s cstate in (%s): succeed found %s",
		t.res, strings.Join(candidates, ","), state)
	return state, nil
}

func (t *T) WaitConnectingOrConnected(ctx context.Context) (string, error) {
	return t.WaitCState(ctx, waitConnectingOrConnectedTimeout, ConnStateConnecting, ConnStateConnected)
}

func (t *T) ConnectAndWaitConnectingOrConnected(ctx context.Context) (string, error) {
	if err := t.Connect(ctx); err != nil {
		return "", fmt.Errorf("drbd resource %s: connect: %w", t.res, err)
	}
	return t.WaitConnectingOrConnected(ctx)
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

func (t *T) ModProbe(ctx context.Context) error {
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
		command.WithContext(ctx),
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

func (t *T) withLock(ctx context.Context, f func(context.Context) error, intent string, timeout time.Duration) error {
	return lock.Func(t.lockFile(), timeout, intent, func() error { return f(ctx) })
}

func (t *T) lockFile() string {
	return filepath.Join(rawconfig.Paths.Lock, "drbd-"+t.res+".lock")
}
