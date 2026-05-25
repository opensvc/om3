package arrayhp3par

// Package arrayhp3par implements the array.hp3par driver for HPE 3PAR
// storage arrays.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

const (
	// command timeouts
	DefaultTimeout = 10 * time.Second
	LongTimeout    = 300 * time.Second

	// 3PAR CLI environment variables
	EnvCSVTable  = "csvtable"
	EnvNoHdTot   = "nohdtot"
	EnvTPDPWFile = "TPDPWFILE"
	EnvTPDNoCert = "TPDNOCERTPROMPT"
)

var (
	// ErrArrayNotAccessible is returned when the array cannot be reached
	ErrArrayNotAccessible = errors.New("array not accessible")
	// ErrInvalidMethod is returned when the connection method is invalid
	ErrInvalidMethod = errors.New("invalid connection method")
	// ErrCommandFailed is returned when a 3PAR command fails
	ErrCommandFailed = errors.New("3par command failed")
)

type (
	// Array is the driver structure for HPE 3PAR arrays
	Array struct {
		*array.Array
		log *plog.Logger
	}

	// Config holds the array configuration from the cluster config
	Config struct {
		// Method is the connection method: ssh, cli, or proxy
		Method string `json:"method"`
		// Manager is the array name or IP address
		Manager string `json:"manager"`
		// Username for SSH connections
		Username string `json:"username,omitempty"`
		// Key is the SSH private key file path
		Key string `json:"key,omitempty"`
		// CLI is the 3PAR CLI binary path
		CLI string `json:"cli,omitempty"`
		// PWFile is the password file for CLI connections
		PWFile string `json:"pwf,omitempty"`
		// Path is an optional path prefix for proxy commands
		Path string `json:"path,omitempty"`
		// UUID is the client UUID for proxy authentication
		UUID string `json:"uuid,omitempty"`
	}

	// Volume represents a 3PAR virtual volume
	Volume struct {
		Name         string `json:"name"`
		VVWwn        string `json:"vv_wwn,omitempty"`
		Prov         string `json:"prov,omitempty"`
		CopyOf       string `json:"copy_of,omitempty"`
		TotRsvdMB    int64  `json:"tot_rsvd_mb,omitempty"`
		VSizeMB      int64  `json:"vsize_mb,omitempty"`
		UsrCPG       string `json:"usr_cpg,omitempty"`
		CreationTime string `json:"creation_time,omitempty"`
		RcopyGroup   string `json:"rcopy_group,omitempty"`
		RcopyStatus  string `json:"rcopy_status,omitempty"`
	}

	// System represents a 3PAR storage system
	System struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Model     string `json:"model"`
		Serial    string `json:"serial"`
		Nodes     int    `json:"nodes"`
		Master    string `json:"master"`
		TotalCap  int64  `json:"total_cap"`
		AllocCap  int64  `json:"alloc_cap"`
		FreeCap   int64  `json:"free_cap"`
		FailedCap int64  `json:"failed_cap"`
	}

	// Node represents a 3PAR controller node
	Node struct {
		AvailableCache int    `json:"available_cache"`
		ControlMem     int    `json:"control_mem"`
		DataMem        int    `json:"data_mem"`
		InCluster      string `json:"in_cluster"`
		LED            string `json:"led"`
		Master         string `json:"master"`
		Name           string `json:"name"`
		Node           int    `json:"node"`
		State          string `json:"state"`
	}

	// CPG represents a 3PAR Common Provisioning Group
	CPG struct {
		Id      string `json:"id"`
		Name    string `json:"name"`
		WarnPct int    `json:"warn%"`
		VVs     int    `json:"vvs"`
		TPVVs   int    `json:"tpvvs"`
		Usr     int    `json:"usr"`
		Snp     int    `json:"snp"`
		Total   int64  `json:"total"`
		Used    int64  `json:"used"`
	}

	// Port represents a 3PAR port
	Port struct {
		NSP           string `json:"n_s_p"`
		Mode          string `json:"mode"`
		State         string `json:"state"`
		NodeWWN       string `json:"node_wwn"`
		PortWWN       string `json:"port_wwn"`
		Type          string `json:"type"`
		Protocol      string `json:"protocol"`
		Label         string `json:"label"`
		Partner       string `json:"partner"`
		FailoverState string `json:"failover_state"`
	}

	// Version represents 3PAR system version information
	Version struct {
		Version string `json:"version"`
	}

	// RCG (Remote Copy Group) status
	RCG struct {
		Name    string      `json:"name"`
		Target  string      `json:"target"`
		Status  string      `json:"status"`
		Role    string      `json:"role"`
		Mode    string      `json:"mode"`
		Options []string    `json:"options,omitempty"`
		Volumes []RCGVolume `json:"volumes,omitempty"`
	}

	// RCGVolume represents a volume in a Remote Copy Group
	RCGVolume struct {
		LocalVV      string    `json:"local_vv"`
		ID           string    `json:"id"`
		RemoteVV     string    `json:"remote_vv"`
		RemoteID     string    `json:"remote_id"`
		SyncStatus   string    `json:"sync_status"`
		LastSyncTime time.Time `json:"last_sync_time"`
	}
)

var (
	cmdListVolumes = &cobra.Command{}
	cmdListSystems = &cobra.Command{}
	cmdListNodes   = &cobra.Command{}
	cmdListCPGs    = &cobra.Command{}
	cmdListPorts   = &cobra.Command{}
	cmdShowVersion = &cobra.Command{}
	cmdListRCGs    = &cobra.Command{}
	cmdShowRCG     = &cobra.Command{}
)

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "hp3par"), NewDriver)
}

// NewDriver returns a new array.hp3par driver instance
func NewDriver() array.Driver {
	return New()
}

// New returns a new array.hp3par driver instance
func New() *Array {
	return &Array{
		Array: array.New(),
	}
}

// Log returns the driver logger
func (t *Array) Log() *plog.Logger {
	if t.log == nil {
		t.log = plog.NewDefaultLogger().Attr("array", t.Name()).Attr("driver", "array.hp3par")
	}
	return t.log
}

// Run implements the driver.Driver interface
func (t *Array) Run(args []string) error {
	var err error

	parent := newParent()

	cmdListVolumes.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdListVolumes.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdListVolumes(cmd, args)
	}

	cmdListSystems.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdListSystems.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdListSystems(cmd, args)
	}

	cmdListNodes.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdListNodes.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdListNodes(cmd, args)
	}

	cmdListCPGs.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdListCPGs.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdListCPGs(cmd, args)
	}

	cmdListPorts.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdListPorts.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdListPorts(cmd, args)
	}

	cmdShowVersion.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdShowVersion.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdShowVersion(cmd, args)
	}

	cmdListRCGs.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdListRCGs.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdListRCGs(cmd, args)
	}

	cmdShowRCG.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return t.PreRunE(cmd, args)
	}
	cmdShowRCG.RunE = func(cmd *cobra.Command, args []string) error {
		return t.CmdShowRCG(cmd, args)
	}

	useFlagListVolumes()
	useFlagListSystems()
	useFlagListNodes()
	useFlagListCPGs()
	useFlagListPorts()
	useFlagShowVersion()
	useFlagListRCGs()
	useFlagShowRCG()

	parent.AddCommand(cmdListVolumes)
	parent.AddCommand(cmdListSystems)
	parent.AddCommand(cmdListNodes)
	parent.AddCommand(cmdListCPGs)
	parent.AddCommand(cmdListPorts)
	parent.AddCommand(cmdShowVersion)
	parent.AddCommand(cmdListRCGs)
	parent.AddCommand(cmdShowRCG)

	if err = parent.Execute(); err != nil {
		return err
	}
	return nil
}

func newParent() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "array hp3par",
		Short: "HPE 3PAR array driver",
	}
	return cmd
}

func (t *Array) PreRunE(cmd *cobra.Command, args []string) error {
	if err := t.Array.PreRunE(cmd, args); err != nil {
		return err
	}
	return t.loadConfig()
}

// loadConfig loads the array configuration from the cluster config
func (t *Array) loadConfig() error {
	if t.Config() == nil {
		return nil
	}

	// Get configuration from the array section in cluster config
	// The section name is derived from the array name (e.g., "array#myarray")
	config := &Config{}

	// Try to get method
	if v, err := t.Config().Get("method"); err == nil {
		config.Method = v
	}

	// Try to get manager (array name/IP)
	if v, err := t.Config().Get("manager"); err == nil {
		config.Manager = v
	} else {
		// If manager is not set, use the array name from the section
		name := t.Name()
		if strings.HasPrefix(name, "array#") {
			config.Manager = strings.TrimPrefix(name, "array#")
		} else {
			config.Manager = name
		}
	}

	// Get SSH credentials
	if v, err := t.Config().Get("username"); err == nil {
		config.Username = v
	}
	if v, err := t.Config().Get("key"); err == nil {
		config.Key = v
	}

	// Get CLI credentials
	if v, err := t.Config().Get("cli"); err == nil {
		config.CLI = v
	}
	if v, err := t.Config().Get("pwf"); err == nil {
		config.PWFile = v
	}

	// Get optional settings
	if v, err := t.Config().Get("path"); err == nil {
		config.Path = v
	}
	if v, err := t.Config().Get("uuid"); err == nil {
		config.UUID = v
	}

	// Validate configuration
	if config.Method == "" {
		return fmt.Errorf("method is required in array configuration")
	}

	if config.Method == "ssh" {
		if config.Username == "" || config.Key == "" {
			return fmt.Errorf("username and key are required for ssh method")
		}
	} else if config.Method == "cli" {
		if config.CLI == "" {
			config.CLI = "cli" // default CLI binary name
		}
		if config.PWFile == "" {
			return fmt.Errorf("pwf is required for cli method")
		}
	} else if config.Method != "proxy" {
		return fmt.Errorf("unsupported method: %s (supported: ssh, cli, proxy)", config.Method)
	}

	t.log.Debug().Interface("config", config).Msg("loaded array configuration")

	return nil
}

// getConfig returns the loaded configuration
func (t *Array) getConfig() (*Config, error) {
	// In a real implementation, this would be stored during loadConfig
	// For now, we'll reload it
	config := &Config{}

	name := t.Name()
	if strings.HasPrefix(name, "array#") {
		config.Manager = strings.TrimPrefix(name, "array#")
	} else {
		config.Manager = name
	}

	if t.Config() != nil {
		if v, err := t.Config().Get("method"); err == nil {
			config.Method = v
		}
		if v, err := t.Config().Get("manager"); err == nil {
			config.Manager = v
		}
		if v, err := t.Config().Get("username"); err == nil {
			config.Username = v
		}
		if v, err := t.Config().Get("key"); err == nil {
			config.Key = v
		}
		if v, err := t.Config().Get("cli"); err == nil {
			config.CLI = v
		} else {
			config.CLI = "cli"
		}
		if v, err := t.Config().Get("pwf"); err == nil {
			config.PWFile = v
		}
		if v, err := t.Config().Get("path"); err == nil {
			config.Path = v
		}
		if v, err := t.Config().Get("uuid"); err == nil {
			config.UUID = v
		}
	}

	return config, nil
}

// runCommand executes a 3PAR command using the configured method
func (t *Array) runCommand(ctx context.Context, cmd string) (string, error) {
	config, err := t.getConfig()
	if err != nil {
		return "", err
	}

	switch config.Method {
	case "ssh":
		return t.runSSHCommand(ctx, cmd, config)
	case "cli":
		return t.runCLICommand(ctx, cmd, config)
	case "proxy":
		return t.runProxyCommand(ctx, cmd, config)
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidMethod, config.Method)
	}
}

// runSSHCommand executes a command via SSH
func (t *Array) runSSHCommand(ctx context.Context, cmd string, config *Config) (string, error) {
	// Build SSH command: ssh -i <key> <username>@<manager> <command>
	sshCmd := []string{"ssh", "-i", config.Key, fmt.Sprintf("%s@%s", config.Username, config.Manager)}

	// Prefix 3PAR commands with environment settings
	fullCmd := fmt.Sprintf("setclienv %s 1; setclienv %s 1; %s; exit", EnvCSVTable, EnvNoHdTot, cmd)

	t.log.Debug().Str("command", fullCmd).Msg("executing via ssh")

	cmdExec := exec.CommandContext(ctx, sshCmd[0], sshCmd[1:]...)
	cmdExec.Stdin = strings.NewReader(fullCmd)

	var stdout, stderr bytes.Buffer
	cmdExec.Stdout = &stdout
	cmdExec.Stderr = &stderr

	err := cmdExec.Run()
	out := stdout.String()
	errOut := stderr.String()

	if err != nil {
		if len(errOut) > 0 {
			t.log.Error().Str("stderr", errOut).Msg("ssh command failed")
		}
		return cleanOutput(out), fmt.Errorf("%w: %s", ErrCommandFailed, err)
	}

	return cleanOutput(out), nil
}

// runCLICommand executes a command via the 3PAR CLI binary
func (t *Array) runCLICommand(ctx context.Context, cmd string, config *Config) (string, error) {
	// Set environment variables for CLI
	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=1", EnvCSVTable))
	env = append(env, fmt.Sprintf("%s=1", EnvNoHdTot))
	env = append(env, fmt.Sprintf("%s=%s", EnvTPDPWFile, config.PWFile))
	env = append(env, fmt.Sprintf("%s=1", EnvTPDNoCert))

	// Build CLI command: cli -sys <array> -nohdtot -csvtable <command>
	args := []string{"-sys", config.Manager, "-nohdtot", "-csvtable"}
	args = append(args, strings.Fields(cmd)...)

	// Use the configured CLI binary or default to "cli"
	cliBin := config.CLI
	if cliBin == "" {
		cliBin = "cli"
	}

	// Check if CLI binary exists
	if _, err := exec.LookPath(cliBin); err != nil {
		return "", fmt.Errorf("3par cli binary not found: %s", cliBin)
	}

	t.log.Debug().Str("binary", cliBin).Strs("args", args).Msg("executing cli command")

	cmdExec := exec.CommandContext(ctx, cliBin, args...)
	cmdExec.Env = env

	var stdout, stderr bytes.Buffer
	cmdExec.Stdout = &stdout
	cmdExec.Stderr = &stderr

	err := cmdExec.Run()
	out := stdout.String()
	errOut := stderr.String()

	if err != nil {
		if len(errOut) > 0 {
			t.log.Error().Str("stderr", errOut).Msg("cli command failed")
		}
		// Check for specific error messages
		if strings.Contains(errOut, "authenticity of the storage system cannot be established") {
			return "", fmt.Errorf("3par connection error: array ssl cert is not trusted. open interactive session to trust it")
		}
		return cleanOutput(out), fmt.Errorf("%w: %s", ErrCommandFailed, err)
	}

	return cleanOutput(out), nil
}

// runProxyCommand executes a command via the proxy API
func (t *Array) runProxyCommand(ctx context.Context, cmd string, config *Config) (string, error) {
	// Proxy method uses HTTP API
	// URL: https://<manager>/api/cmd/
	url := fmt.Sprintf("https://%s/api/cmd/", config.Manager)

	// Get UUID if not set
	uuid := config.UUID
	if uuid == "" {
		// Try to get from environment or generate
		uuid = os.Getenv("OM3_NODE_UUID")
		if uuid == "" {
			// In a real implementation, we'd get this from node config
			uuid = "unknown"
		}
	}

	// Build form data
	formData := map[string]string{
		"array": config.Manager,
		"cmd":   cmd,
		"path":  config.Path,
		"uuid":  uuid,
	}

	t.log.Debug().Str("url", url).Interface("form", formData).Msg("executing proxy command")

	// In a real implementation, we'd use http client
	// For now, return an error as proxy is not fully implemented
	return "", fmt.Errorf("proxy method not yet implemented")
}

// cleanOutput removes prompt characters and other artifacts from 3PAR output
func cleanOutput(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string

	for _, line := range lines {
		// Remove prompt character (%)
		if idx := strings.Index(line, "%"); idx != -1 {
			line = line[idx+1:]
			if strings.HasPrefix(line, " ") {
				line = strings.TrimPrefix(line, " ")
			}
		}
		// Remove empty lines
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

// parseCSV parses 3PAR CSV output into a slice of maps
func (t *Array) parseCSV(output string, cols []string) []map[string]string {
	var results []map[string]string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		values := t.splitCSVLine(line)
		if len(values) == 0 {
			continue
		}

		item := make(map[string]string)
		for i, col := range cols {
			if i < len(values) {
				item[col] = values[i]
			} else {
				item[col] = ""
			}
		}
		results = append(results, item)
	}

	return results
}

// splitCSVLine splits a CSV line respecting quoted strings
func (t *Array) splitCSVLine(line string) []string {
	var values []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				values = append(values, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	// Add the last value
	values = append(values, strings.TrimSpace(current.String()))

	return values
}

// CmdListVolumes lists all virtual volumes on the array
func (t *Array) CmdListVolumes(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	// Check if we need to show remote copy info
	showRCopy := false
	if t.Config() != nil {
		if _, err := t.Config().Get("remotecopy"); err == nil {
			showRCopy = true
		}
	}

	cols := []string{"Name", "VV_WWN", "Prov", "CopyOf", "Tot_Rsvd_MB", "VSize_MB", "UsrCPG", "CreationTime"}
	if showRCopy {
		cols = append(cols, "RcopyGroup", "RcopyStatus")
	}

	cmdStr := fmt.Sprintf("showvv -showcols %s", strings.Join(cols, ","))
	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	rows := t.parseCSV(out, cols)
	var volumes []Volume
	for _, row := range rows {
		vol := Volume{
			Name:         row["Name"],
			VVWwn:        row["VV_WWN"],
			Prov:         row["Prov"],
			CopyOf:       row["CopyOf"],
			UsrCPG:       row["UsrCPG"],
			CreationTime: row["CreationTime"],
		}
		if v, ok := row["Tot_Rsvd_MB"]; ok {
			vol.TotRsvdMB = t.parseInt64(v)
		}
		if v, ok := row["VSize_MB"]; ok {
			vol.VSizeMB = t.parseInt64(v)
		}
		if showRCopy {
			vol.RcopyGroup = row["RcopyGroup"]
			vol.RcopyStatus = row["RcopyStatus"]
		}
		volumes = append(volumes, vol)
	}

	return datarecv.Emit(volumes)
}

// CmdListSystems lists storage system information
func (t *Array) CmdListSystems(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cols := []string{"ID", "Name", "Model", "Serial", "Nodes", "Master", "TotalCap", "AllocCap", "FreeCap", "FailedCap"}
	cmdStr := "showsys"

	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	rows := t.parseCSV(out, cols)
	var systems []System
	for _, row := range rows {
		sys := System{
			ID:     row["ID"],
			Name:   row["Name"],
			Model:  row["Model"],
			Serial: row["Serial"],
			Master: row["Master"],
		}
		sys.Nodes = int(t.parseInt64(row["Nodes"]))
		sys.TotalCap = t.parseInt64(row["TotalCap"])
		sys.AllocCap = t.parseInt64(row["AllocCap"])
		sys.FreeCap = t.parseInt64(row["FreeCap"])
		sys.FailedCap = t.parseInt64(row["FailedCap"])
		systems = append(systems, sys)
	}

	return datarecv.Emit(systems)
}

// CmdListNodes lists controller nodes
func (t *Array) CmdListNodes(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cols := []string{"Available_Cache", "Control_Mem", "Data_Mem", "InCluster", "LED", "Master", "Name", "Node", "State"}
	cmdStr := fmt.Sprintf("shownode -showcols %s", strings.Join(cols, ","))

	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	rows := t.parseCSV(out, cols)
	var nodes []Node
	for _, row := range rows {
		node := Node{
			Name:      row["Name"],
			InCluster: row["InCluster"],
			LED:       row["LED"],
			Master:    row["Master"],
			State:     row["State"],
		}
		node.AvailableCache = int(t.parseInt64(row["Available_Cache"]))
		node.ControlMem = int(t.parseInt64(row["Control_Mem"]))
		node.DataMem = int(t.parseInt64(row["Data_Mem"]))
		node.Node = int(t.parseInt64(row["Node"]))
		nodes = append(nodes, node)
	}

	return datarecv.Emit(nodes)
}

// CmdListCPGs lists Common Provisioning Groups
func (t *Array) CmdListCPGs(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cols := []string{"Id", "Name", "Warn%", "VVs", "TPVVs", "Usr", "Snp", "Total", "Used"}
	cmdStr := "showcpg"

	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	rows := t.parseCSV(out, cols)
	var cpgs []CPG
	for _, row := range rows {
		cpg := CPG{
			Id:   row["Id"],
			Name: row["Name"],
		}
		cpg.WarnPct = int(t.parseInt64(row["Warn%"]))
		cpg.VVs = int(t.parseInt64(row["VVs"]))
		cpg.TPVVs = int(t.parseInt64(row["TPVVs"]))
		cpg.Usr = int(t.parseInt64(row["Usr"]))
		cpg.Snp = int(t.parseInt64(row["Snp"]))
		cpg.Total = t.parseInt64(row["Total"])
		cpg.Used = t.parseInt64(row["Used"])
		cpgs = append(cpgs, cpg)
	}

	return datarecv.Emit(cpgs)
}

// CmdListPorts lists array ports
func (t *Array) CmdListPorts(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cols := []string{"N:S:P", "Mode", "State", "Node_WWN", "Port_WWN", "Type", "Protocol", "Label", "Partner", "FailoverState"}
	cmdStr := "showport"

	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	rows := t.parseCSV(out, cols)
	var ports []Port
	for _, row := range rows {
		port := Port{
			NSP:           row["N:S:P"],
			Mode:          row["Mode"],
			State:         row["State"],
			NodeWWN:       row["Node_WWN"],
			PortWWN:       row["Port_WWN"],
			Type:          row["Type"],
			Protocol:      row["Protocol"],
			Label:         row["Label"],
			Partner:       row["Partner"],
			FailoverState: row["FailoverState"],
		}
		ports = append(ports, port)
	}

	return datarecv.Emit(ports)
}

// CmdShowVersion shows the array version
func (t *Array) CmdShowVersion(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmdStr := "showversion -s"
	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	version := Version{Version: strings.TrimSpace(out)}
	return datarecv.Emit(version)
}

// CmdListRCGs lists Remote Copy Groups
func (t *Array) CmdListRCGs(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmdStr := "showrcopy groups"
	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	// Parse RCG output - this is a complex format with nested data
	// For now, return raw output
	return datarecv.Emit(map[string]string{"output": out})
}

// CmdShowRCG shows details of a specific Remote Copy Group
func (t *Array) CmdShowRCG(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	if len(args) == 0 {
		return fmt.Errorf("rcg name is required")
	}

	rcgName := args[0]
	cmdStr := fmt.Sprintf("showrcopy -group %s", rcgName)
	out, err := t.runCommand(ctx, cmdStr)
	if err != nil {
		return err
	}

	// Parse RCG status output
	rcg, err := t.parseRCGStatus(out, rcgName)
	if err != nil {
		return err
	}

	return datarecv.Emit(rcg)
}

// parseRCGStatus parses the showrcopy output for a specific RCG
func (t *Array) parseRCGStatus(out string, rcgName string) (*RCG, error) {
	// The output format is:
	// Name,Target,Status,Role,Mode,"Options"
	//  ,LocalVV,ID,RemoteVV,ID,SyncStatus,LastSyncTime
	//  ,vol1,1,vol2,2,Synced,2024-01-01 12:00:00

	lines := strings.Split(out, "\n")
	rcg := &RCG{Name: rcgName}
	var inRCGBlock bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is our RCG line
		if strings.HasPrefix(line, rcgName+",") || strings.HasPrefix(line, rcgName) {
			inRCGBlock = true
			if err := t.parseRCGHeader(line, rcg); err != nil {
				return nil, err
			}
			continue
		}

		if !inRCGBlock {
			continue
		}

		// End of RCG block (line doesn't start with space or comma)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, ",") {
			break
		}

		// Parse volume line
		if strings.HasPrefix(line, ",") || strings.HasPrefix(line, " ") {
			vv, err := t.parseRCGVolumeLine(line)
			if err != nil {
				t.log.Warn().Str("line", line).Err(err).Msg("failed to parse volume line")
				continue
			}
			if vv != nil {
				rcg.Volumes = append(rcg.Volumes, *vv)
			}
		}
	}

	return rcg, nil
}

// parseRCGHeader parses the RCG header line
func (t *Array) parseRCGHeader(line string, rcg *RCG) error {
	// Remove leading comma if present
	line = strings.TrimPrefix(line, ",")

	// Split by comma, respecting quotes
	parts := t.splitCSVLine(line)
	if len(parts) < 5 {
		return fmt.Errorf("invalid rcg header line: %s", line)
	}

	rcg.Target = parts[0]
	rcg.Status = parts[1]
	rcg.Role = parts[2]
	rcg.Mode = parts[3]

	// The rest is options in a quoted string
	if len(parts) > 5 {
		optionsStr := strings.Join(parts[4:], ",")
		// Remove quotes
		optionsStr = strings.Trim(optionsStr, `"`)
		// Split options by comma
		for _, opt := range strings.Split(optionsStr, ",") {
			opt = strings.TrimSpace(opt)
			if opt != "" {
				rcg.Options = append(rcg.Options, opt)
			}
		}
	}

	return nil
}

// parseRCGVolumeLine parses a volume line from RCG output
func (t *Array) parseRCGVolumeLine(line string) (*RCGVolume, error) {
	// Remove leading comma and space
	line = strings.TrimPrefix(line, ",")
	line = strings.TrimPrefix(line, " ")

	parts := t.splitCSVLine(line)
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid volume line: %s", line)
	}

	vv := &RCGVolume{
		LocalVV:    parts[0],
		ID:         parts[1],
		RemoteVV:   parts[2],
		RemoteID:   parts[3],
		SyncStatus: parts[4],
	}

	// Parse LastSyncTime
	timeStr := strings.TrimSpace(parts[5])
	if timeStr != "" {
		// Try to parse the time in various formats
		formats := []string{
			"2006-01-02 15:04:05 MST",
			"2006-01-02 15:04:05",
			"02-Jan-2006 15:04:05 MST",
		}
		for _, format := range formats {
			parsed, err := time.Parse(format, timeStr)
			if err == nil {
				vv.LastSyncTime = parsed.UTC()
				break
			}
		}
	}

	return vv, nil
}

// parseInt64 parses a string to int64, returning 0 on error
func (t *Array) parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.log.Warn().Str("value", s).Err(err).Msg("failed to parse int64")
		return 0
	}
	return val
}

// Key implements the key.Provider interface for use with the xconfig package
func (t *Array) Key(s string) key.T {
	return key.T{Section: t.Name(), Option: s}
}
