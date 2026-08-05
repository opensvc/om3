package ressyncplakar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/resourceparser"
	"github.com/rs/zerolog"
	"sigs.k8s.io/yaml"
)

type (
	T struct {
		ressync.T
		StoreConfig  string
		Passphrase   string
		Src          []string
		PolicyConfig string
		PolicyName   string
		Name         string
		DstConfig    string

		lastBackup      time.Time
		lastBackupCount int
	}

	policiesCfg struct {
		Policies map[string]any `yaml:"policies"`
	}

	backupSource struct {
		Path string
		Rid  string
		Src  string
	}

	backupList struct {
		SnapshotId string
		Timestamp  time.Time
	}

	header interface {
		Head() string
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
	core interface {
		FQDN() string
		Path() naming.Path
	}
)

const (
	plakar = "plakar"
)

var (
	lockName = "sync"

	policiesFile = "policies.yml"
	storesFile   = "stores.yml"
	dstFile      = "destinations.yml"
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Status(context.Context) status.T {
	return t.StatusLastSync([]string{hostname.Hostname()})
}

func (t *T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action: "sync_update",
		Option: "schedule",
		Base:   "",
	}
}

func (t *T) Restore(ctx context.Context, to, src string) error {
	if to == "" {
		return nil
	}
	var latestBackup backupList
	err := t.execList(src, func(line string) {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return
		}
		timestamp, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			t.Log().Warnf("failed to parse timestamp from line '%s': %v", line, err)
			return
		}
		if latestBackup == (backupList{}) || timestamp.After(latestBackup.Timestamp) {
			latestBackup = backupList{
				SnapshotId: parts[1],
				Timestamp:  timestamp,
			}
		}
	})
	if err != nil {
		return err
	}
	if latestBackup == (backupList{}) {
		return fmt.Errorf("no backup found")
	}
	if err := t.execRestore(to, latestBackup.SnapshotId); err != nil {
		return err
	}
	return nil
}

func (t *T) Label(context.Context) string {
	if t.lastBackup.IsZero() {
		return "never backed up"
	}
	return fmt.Sprintf("last backup: %s (%d dirs)", t.lastBackup.Format(time.RFC822), t.lastBackupCount)
}

func (t *T) Update(ctx context.Context) error {
	if err := t.backup(ctx); err != nil {
		return err
	}
	if err := t.WriteLastSync(hostname.Hostname()); err != nil {
		return err
	}
	return nil
}

func (t *T) Provisioned(context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Running() (resource.RunningInfoList, error) {
	return t.RunningFromLock(lockName)
}

func (t *T) backup(ctx context.Context) error {
	return t.backupWithRetries(ctx, 0)
}

func (t *T) backupWithRetries(ctx context.Context, retries int) error {
	const maxRetries = 2

	if retries >= maxRetries {
		return fmt.Errorf("max retries exceeded on backup")
	}

	paths, err := t.parseSrc(ctx)
	if err != nil {
		return err
	}
	exist, err := t.configDirExists()
	if err != nil {
		return err
	}
	if !exist {
		if err = t.importConfig(); err != nil {
			return err
		}
	}
	success := 0
	for _, path := range paths {
		stderr, err := t.execBackup(path)
		if err == nil {
			if cleanErr := t.clean(t.buildFlags(path.Src)); cleanErr != nil {
				return cleanErr
			}
			success++
			continue
		}
		if stderr != "" && t.isConfigError(stderr) {
			t.Log().Infof("Import configuration and retry backup (attempt %d/%d)", retries+1, maxRetries)
			if importErr := t.importConfig(); importErr != nil {
				return importErr
			}
			return t.backupWithRetries(ctx, retries+1)
		}
		return err
	}
	t.lastBackup = time.Now()
	t.lastBackupCount = success
	return nil
}

func (t *T) isConfigError(stderr string) bool {
	configErrors := []string{
		"could not resolve repository",
		"could not load configuration",
	}
	for _, errMsg := range configErrors {
		if strings.Contains(stderr, errMsg) {
			return true
		}
	}
	return false
}

func (t *T) importKey(key, output string) (string, error) {
	km, err := datarecv.ParseKeyMetaRelObj(key, t.GetObject())
	if err != nil {
		return "", err
	}
	keyring := t.getConfigPath(output)
	if err = km.CacheFileAt(keyring); err != nil {
		return "", err
	}
	return keyring, nil
}

func (t *T) configDirExists() (bool, error) {
	_, err := os.Stat(t.getConfigDir())
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (t *T) importConfig() error {
	exists, err := t.configDirExists()
	if err != nil {
		return err
	}
	if _, err := t.importKey(t.StoreConfig, storesFile); err != nil {
		return err
	}
	if t.DstConfig != "" {
		if _, err = t.importKey(t.DstConfig, dstFile); err != nil {
			return err
		}
	}
	if err := t.execCreate(!exists); err != nil {
		return err
	}
	return nil
}

func (t *T) checkPolicy(path string) (string, error) {
	if t.PolicyName != "" {
		return t.PolicyName, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var policies policiesCfg
	if err = yaml.Unmarshal(data, &policies); err != nil {
		return "", err
	}
	if len(policies.Policies) == 0 {
		return "", fmt.Errorf("no policy found in policies.yml")
	}
	if len(policies.Policies) == 1 {
		for name := range policies.Policies {
			return name, nil
		}
	}
	return "", fmt.Errorf("multiple policies found in policies.yml, specify one with the policy keyword")
}

func (t *T) clean(tag string) error {
	if t.PolicyConfig == "" {
		t.Log().Infof("no policy_config configuration, skipping prune")
		return nil
	}
	keyring, err := t.importKey(t.PolicyConfig, policiesFile)
	if err != nil {
		return err
	}
	policy := t.PolicyName
	if policy == "" {
		policy, err = t.checkPolicy(keyring)
		if err != nil {
			return err
		}
	}
	return t.execPrune(policy, tag)
}

func (t *T) fqdn() string {
	return t.GetObject().(core).FQDN()
}

func (t *T) fqn() string {
	path := t.GetObject().(core).Path()
	return path.FQN()
}

func (t *T) getContent(key string) ([]byte, error) {
	km, err := datarecv.ParseKeyMetaRelObj(key, t.GetObject())
	if err != nil {
		return nil, err
	}
	content, err := km.Decode()
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (t *T) parseSrc(ctx context.Context) ([]backupSource, error) {
	drivers, err := t.getHeaderResources(ctx)
	if err != nil {
		return nil, err
	}
	if len(t.Src) == 0 {
		return t.buildSourcesFromDrivers(drivers), nil
	}
	return t.buildSourcesFromConfig(drivers)
}

func (t *T) buildSourcesFromDrivers(drivers []resource.Driver) []backupSource {
	sources := make([]backupSource, 0, len(drivers))
	for _, driver := range drivers {
		if h, ok := driver.(header); ok {
			sources = append(sources, backupSource{
				Path: h.Head() + "/",
				Rid:  driver.RID(),
				Src:  driver.RID(),
			})
		}
	}
	return sources
}

func (t *T) buildSourcesFromConfig(drivers []resource.Driver) ([]backupSource, error) {
	driverMap := t.buildDriverMap(drivers)
	sources := make([]backupSource, 0, len(driverMap))

	for _, src := range t.Src {
		parsed := resourceparser.Parse(strings.TrimSpace(src))
		if parsed.Schema == "file" {
			sources = append(sources, backupSource{
				Path: parsed.Target,
				Rid:  "file",
				Src:  src,
			})
			continue
		}
		if driver, exists := driverMap[parsed.Schema]; exists {
			if h, ok := driver.(header); ok {
				sources = append(sources, backupSource{
					Path: filepath.Join(h.Head(), parsed.Target),
					Rid:  driver.RID(),
					Src:  src,
				})
			}
		}
	}
	return sources, nil
}

func (t *T) buildDriverMap(drivers []resource.Driver) map[string]resource.Driver {
	driverMap := make(map[string]resource.Driver, len(drivers))
	for _, driver := range drivers {
		driverMap[driver.RID()] = driver
	}
	return driverMap
}

func (t *T) getHeaderResources(ctx context.Context) ([]resource.Driver, error) {
	rl, ok := t.GetObject().(resourceLister)
	if !ok {
		return []resource.Driver{}, fmt.Errorf("object does not implement resourceLister")
	}
	sb := statusbus.FromContext(ctx)
	drivers := make([]resource.Driver, 0)
	for _, r := range rl.Resources() {
		if _, ok := r.(header); !ok {
			continue
		}
		rStatus := sb.Get(r.RID())
		if rStatus != status.Up {
			continue
		}
		drivers = append(drivers, r)
	}
	return drivers, nil
}

func (t *T) getConfigFlag() string {
	if capabilities.Has(capsConfigdir) {
		return "-configdir"
	}
	return "-config"
}

func (t *T) getConfigPath(name string) string {
	return filepath.Join(rawconfig.Paths.Run, t.fqn(), plakar, name)
}

func (t *T) getConfigDir() string {
	return filepath.Join(rawconfig.Paths.Run, t.fqn(), plakar)
}

func (t *T) buildFlags(src string) string {
	flags := []string{"src=" + src, "node=" + hostname.Hostname(), "path=" + t.GetObject().(core).Path().String(), "rid=" + t.RID()}
	if t.Name != "" {
		flags = append(flags, "name="+t.Name)
	}
	return strings.Join(flags, ",")
}

func (t *T) buildCommandWithPassphrase(args ...string) (*command.T, error) {
	passphrase, err := t.getContent(t.Passphrase)
	if err != nil {
		return nil, err
	}
	return command.New(
		command.WithName(capabilities.GetPath(plakar)),
		command.WithVarArgs(args...),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
		command.WithVarEnv("PLAKAR_PASSPHRASE="+string(passphrase)),
	), nil
}

func (t *T) execBackup(src backupSource) (string, error) {
	cmd, err := t.buildCommandWithPassphrase(t.getConfigFlag(), t.getConfigDir(), "at", "@"+t.fqdn(), "backup", "-tag", t.buildFlags(src.Src), src.Path)
	if err != nil {
		return "", err
	}
	err = cmd.Run()
	return string(cmd.Stderr()), err
}

func (t *T) execCreate(overwritten bool) error {
	cmd, err := t.buildCommandWithPassphrase(t.getConfigFlag(), t.getConfigDir(), "at", "@"+t.fqdn(), "create")
	if err != nil {
		return err
	}
	err = cmd.Run()
	if err != nil && overwritten {
		if _, err := t.importKey(t.StoreConfig, storesFile); err != nil {
			return err
		}
		return t.execCreate(false)
	}
	return err
}

func (t *T) execPrune(policy, tag string) error {
	if len(t.PolicyConfig) <= 0 {
		t.Log().Infof("no policy_config configuration, skipping prune")
		return nil
	}
	cmd, err := t.buildCommandWithPassphrase(t.getConfigFlag(), t.getConfigDir(), "at", "@"+t.fqdn(), "prune", "-tag", tag, "-policy", policy, "-apply")
	if err != nil {
		return err
	}
	return cmd.Run()
}

func (t *T) execRestore(path, snapshotId string) error {
	args := []string{t.getConfigFlag(), t.getConfigDir(), "at", "@" + t.fqdn(), "restore", "-to", path, snapshotId}
	cmd, err := t.buildCommandWithPassphrase(args...)
	if err != nil {
		return err
	}
	return cmd.Run()
}

func (t *T) execList(src string, onLine func(string)) error {
	args := []string{t.getConfigFlag(), t.getConfigDir(), "at", "@" + t.fqdn(), "ls"}
	if src != "" {
		args = append(args, "-tag", t.buildFlags(src))
	}

	passphrase, err := t.getContent(t.Passphrase)
	if err != nil {
		return err
	}
	cmdOpt := []funcopt.O{
		command.WithName(capabilities.GetPath(plakar)),
		command.WithVarArgs(args...),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
		command.WithVarEnv("PLAKAR_PASSPHRASE=" + string(passphrase)),
	}
	if onLine == nil {
		cmdOpt = append(cmdOpt, command.WithStdoutLogLevel(zerolog.InfoLevel))
	} else {
		cmdOpt = append(cmdOpt, command.WithOnStdoutLine(onLine))
	}
	cmd := command.New(cmdOpt...)

	return cmd.Run()
}
