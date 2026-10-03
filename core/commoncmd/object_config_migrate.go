package commoncmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectConfigMigrate struct {
		OptsGlobal
		OptsLock
		DryRun bool
	}
)

// Run writes the configurations of the selected objects in the shape om reads
// them in.
//
// The changes land through the configuration update path, so a migration is
// weighed like any other write: the same validation, the same policy, the same
// claim on a pool.
func (t *CmdObjectConfigMigrate) Run(kind string) error {
	mergedSelector := MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := t.migrate(c, p, len(paths) > 1); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectConfigMigrate) migrate(c *client.T, p naming.Path, prefixed bool) error {
	prefix := ""
	if prefixed {
		prefix = p.String() + ": "
	}
	b, err := configFile(c, p)
	if err != nil {
		return err
	}
	o, err := object.New(p, object.WithConfigData(b), object.WithVolatile(true))
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	configurer, ok := o.(object.Configurer)
	if !ok {
		return fmt.Errorf("%s: a %s has no configuration to migrate", p, p.Kind)
	}
	m := object.MigrationRulesOf(p.Kind).Apply(configurer.Config())
	sets, unsets, deletes, ok := showMigration(prefix, m)
	if !ok || t.DryRun {
		return nil
	}
	backup, err := backupConfig(objectBackupName(p), b)
	if err != nil {
		// A change nothing can undo is not one to make: the configuration
		// this rewrites is the only copy of what the object was.
		return fmt.Errorf("%s: keep a copy of the configuration before changing it: %w", p, err)
	}
	fmt.Printf("%sthe configuration as it was is kept in %s\n", prefix, backup)
	params := api.PatchObjectConfigParams{}
	params.Set = &sets
	params.Unset = &unsets
	params.Delete = &deletes
	response, err := c.PatchObjectConfigWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return err
	}
	switch response.StatusCode() {
	case 200:
		if response.JSON200.IsChanged {
			fmt.Printf("%scommitted\n", prefix)
		} else {
			fmt.Printf("%sunchanged\n", prefix)
		}
	case 400:
		return fmt.Errorf("%s: %s", p, *response.JSON400)
	case 401:
		return fmt.Errorf("%s: %s", p, *response.JSON401)
	case 403:
		return fmt.Errorf("%s: %s", p, *response.JSON403)
	case 404:
		return fmt.Errorf("%s: %s", p, *response.JSON404)
	case 409:
		return fmt.Errorf("%s: %s", p, *response.JSON409)
	case 500:
		return fmt.Errorf("%s: %s", p, *response.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, response.Status())
	}
	return nil
}

// configFile is the configuration of an object, as the node holding it has
// it.
func configFile(c *client.T, p naming.Path) ([]byte, error) {
	params := api.GetObjectConfigFileParams{}
	resp, err := c.GetObjectConfigFileWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("%s: read the configuration: %s: %s", p, resp.Status(), strings.TrimSpace(string(resp.Body)))
	}
	return resp.Body, nil
}

// showMigration prints what a migration changes, and what it refuses, and
// returns the changes as the parameters of a configuration update. It
// returns false when there is nothing to change.
func showMigration(prefix string, m object.Migration) (sets, unsets, deletes []string, ok bool) {
	for _, s := range m.Refusals {
		fmt.Printf("%s%s\n", prefix, s)
	}
	if len(m.Sets) == 0 && len(m.Unsets) == 0 && len(m.Deletes) == 0 {
		fmt.Printf("%snothing to migrate\n", prefix)
		return nil, nil, nil, false
	}
	for _, s := range m.Notes {
		fmt.Printf("%s%s\n", prefix, s)
	}
	sets = make([]string, 0, len(m.Sets))
	for _, op := range m.Sets {
		sets = append(sets, fmt.Sprintf("%s=%s", op.Key, op.Value))
	}
	unsets = make([]string, 0, len(m.Unsets))
	for _, k := range m.Unsets {
		unsets = append(unsets, k.String())
	}
	deletes = append(make([]string, 0, len(m.Deletes)), m.Deletes...)
	for _, s := range sets {
		fmt.Printf("%sset %s\n", prefix, s)
	}
	for _, s := range unsets {
		fmt.Printf("%sunset %s\n", prefix, s)
	}
	for _, s := range deletes {
		fmt.Printf("%sdelete %s\n", prefix, s)
	}
	return sets, unsets, deletes, true
}

// ObjectConfigMigrateLong is the help of the config migrate command of a kind
// of object, listing the rules it applies.
func ObjectConfigMigrateLong(kind string) string {
	return configMigrateLong(object.MigrationRulesOf(naming.Kind(kind)))
}

// NodeConfigMigrateLong is the help of the node config migrate command,
// listing the rules it applies.
func NodeConfigMigrateLong() string {
	return configMigrateLong(object.NodeMigrationRules)
}

// configMigrateLong is the help of a config migrate command, listing the
// rules it applies.
func configMigrateLong(rules object.MigrationRules) string {
	return `Write the configuration in the shape om reads it in.

A configuration written for an older agent describes things this one no longer
reads that way. What it asked for is still possible, in another shape, and
this writes that shape: the configuration says the same thing afterwards, in
words om reads. What changes is printed, and what no rule can write is printed
with the reason, and kept.

The rules:

` + rules.Doc(78) + `
The changes land as a configuration update, so they are weighed like any other
write. The configuration as it was is kept under the backup directory of the
node this runs on, and where it was kept is printed. --dry-run prints the
changes and writes nothing.`
}

// objectBackupName is where the configuration of an object is kept under the
// backup directory: where it is under the configuration directory.
func objectBackupName(p naming.Path) string {
	name := p.ConfigFile()
	if rel, err := filepath.Rel(rawconfig.Paths.Etc, name); err == nil {
		return rel
	}
	return filepath.Base(name)
}

// backupConfig keeps a configuration as it is now, under the backup directory
// at name, and answers where it kept it.
//
// It is written where this command runs, which is where whoever runs it will
// look for it, and under the backup directory of the node, beside what every
// other command that replaces a configuration leaves there. The time it was
// taken ends the name: a migration asked twice keeps both.
func backupConfig(name string, b []byte) (string, error) {
	name = filepath.Join(rawconfig.Paths.Backup, name) + "." + time.Now().Format("2006-01-02T15:04:05")
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(name, b, 0600); err != nil {
		return "", err
	}
	return name, nil
}
