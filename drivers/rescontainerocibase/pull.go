package rescontainerocibase

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
)

// CLIImageStore is the image store of an engine reached with its command,
// as "docker" or "podman": the one of the system, which the agent reads as
// itself, and the ones of the users Credential knows.
type CLIImageStore struct {
	// Bin is the command of the engine.
	Bin string

	// ManifestIDs says how to tell whether the engine names its images
	// by the digest of their manifest, from the output of an engine
	// command: see DockerManifestIDs. It is nil for an engine naming
	// them by the digest of their configuration, always.
	ManifestIDs func(run func(a ...string) ([]byte, error)) (bool, error)

	// Credential returns who the commands of the engine run as to reach
	// the store named, which is not the one of the system: the user of
	// a rootless podman. It is nil for an engine with no such store.
	Credential func(store string) (*Credential, error)
}

var _ resource.ImageStore = CLIImageStore{}

// command returns the engine command a, run as who reaches the store of
// pull, and refuses an image the engine would read as an option.
func (t CLIImageStore) command(ctx context.Context, pull resource.Pull, a ...string) (*exec.Cmd, error) {
	if !isImageOperand(pull.Image) {
		return nil, fmt.Errorf("invalid image %q", pull.Image)
	}
	var cred *Credential
	if pull.Store != "" {
		// The store of a user is read as that user: the agent reading
		// it as itself would read the one of the system.
		if t.Credential == nil {
			return nil, fmt.Errorf("the %s store of %s is not the one of the system", t.Bin, pull.Store)
		}
		var err error
		if cred, err = t.Credential(pull.Store); err != nil {
			return nil, err
		} else if cred == nil {
			return nil, fmt.Errorf("the %s store of %s is not the one of the system", t.Bin, pull.Store)
		}
	}
	cmd := exec.CommandContext(ctx, t.Bin, a...)
	cred.Demote(cmd)
	return cmd, nil
}

// Has implements resource.ImageStore, with the image inspect of the engine,
// read as Executor.HasImage reads it.
func (t CLIImageStore) Has(ctx context.Context, pull resource.Pull) (bool, string, error) {
	if _, err := t.command(ctx, pull); err != nil {
		return false, "", err
	}
	run := func(ctx context.Context, a ...string) ([]byte, error) {
		cmd, err := t.command(ctx, pull, a...)
		if err != nil {
			return nil, err
		}
		return cmd.Output()
	}
	return hasImage(ctx, run, imageInspectArgs(pull.Image))
}

// Save implements resource.ImageStore, with the image save of the engine.
// The store is asked for the image first: a save failing says a missing
// image its own way in each engine, and what was written by then can not be
// taken back.
func (t CLIImageStore) Save(ctx context.Context, pull resource.Pull, w io.Writer) (string, error) {
	present, id, err := t.Has(ctx, pull)
	if err != nil {
		return "", err
	} else if !present {
		return "", resource.ErrImageNotFound
	}
	cmd, err := t.command(ctx, pull, imageSaveArgs(pull.Image)...)
	if err != nil {
		return "", err
	}
	return id, saveImage(ctx, cmd, w)
}

// IDKind implements resource.ImageStore.
func (t CLIImageStore) IDKind(ctx context.Context, pull resource.Pull) (string, error) {
	if t.ManifestIDs == nil {
		return resource.ImageIDConfig, nil
	}
	manifest, err := t.ManifestIDs(func(a ...string) ([]byte, error) {
		cmd, err := t.command(ctx, pull, a...)
		if err != nil {
			return nil, err
		}
		return cmd.Output()
	})
	switch {
	case err != nil:
		return "", err
	case manifest:
		return resource.ImageIDManifest, nil
	}
	return resource.ImageIDConfig, nil
}

// containerdSnapshotter is what the storage driver status of a docker
// keeping its images in containerd names: the type of its driver, as
// "io.containerd.snapshotter.v1".
const containerdSnapshotter = "io.containerd.snapshotter"

// DockerManifestIDs says whether a docker names its images by the digest of
// their manifest, which the one keeping its images in containerd does, and
// no other: it asks the daemon the status of its storage driver.
func DockerManifestIDs(run func(a ...string) ([]byte, error)) (bool, error) {
	b, err := run("info", "--format", "{{.DriverStatus}}")
	if err != nil {
		return false, fmt.Errorf("the storage driver of the engine is not known: %s", engineErrorMessage(err))
	}
	return strings.Contains(string(b), containerdSnapshotter), nil
}

// Load implements resource.ImageStore, with the image load of the engine.
func (t CLIImageStore) Load(ctx context.Context, pull resource.Pull, r io.Reader) error {
	cmd, err := t.command(ctx, pull, imageLoadArgs()...)
	if err != nil {
		return err
	}
	return loadImage(ctx, cmd, r)
}

// imageInspectArgs are the arguments of the engine command printing the id
// of an image of its store.
func imageInspectArgs(image string) []string {
	return []string{"image", "inspect", "--format", "{{.ID}}", image}
}

// isImageOperand says whether image can be the operand of an engine command:
// one starting with a '-' would be read as an option.
func isImageOperand(image string) bool {
	return image != "" && !strings.HasPrefix(image, "-")
}

// PullOf returns the Pull of a container resource of an engine, read from
// its configuration: the image its start would run, in the store named, with
// the credentials of its registry_creds. The drivers implement
// resource.PullDescriber with it.
func PullOf(engine, store string, src resource.HoldSource) (resource.Pull, bool) {
	image, ok := src.Get("image")
	if !ok || !isImageOperand(image) {
		return resource.Pull{}, false
	}
	creds, _ := src.Get("registry_creds")
	return newPull(engine, store, image, creds, src.Path()), true
}

// newPull is the one maker of the Pull of a resource, from its configuration
// as from its configured driver: the daemon prefetches the key the first
// says, and the status reads the outcome of the key the second says.
func newPull(engine, store, image, registryCreds string, p naming.Path) resource.Pull {
	pull := resource.Pull{
		Engine: engine,
		Store:  store,
		Image:  normalizeImage(engine, image),
	}
	if registryCreds != "" {
		// The secret is one of the namespace of the object: the same
		// name in another namespace is another secret.
		pull.Creds = naming.Path{Namespace: p.Namespace, Kind: naming.KindSec, Name: registryCreds}.String()
	}
	return pull
}

// enginePodman is podman, as resource.Pull names the engines.
const enginePodman = "podman"

// normalizeImage returns the image of a resource as the key of its pull
// says it, which is as its engine reads it. Docker reads a name with no
// registry as one of its hub. Podman resolves it with its registries.conf,
// maybe to another registry: its name is kept as written, for the store to
// be asked the image a start would run, and not one of the hub it does not
// hold, which the daemon would see missing at every check.
func normalizeImage(engine, image string) string {
	if engine == enginePodman {
		return resource.NormalizeQualifiedImage(image)
	}
	return resource.NormalizeImage(image)
}

// SetImageStore names the engine of the container and the store its image
// is pulled to, as resource.Pull says them. The drivers set them when
// configured.
func (t *BT) SetImageStore(engine, store string) {
	t.imageEngine = engine
	t.imageStore = store
}

// pullKey is the Pull of the resource, as PullOf says it from the
// configuration.
func (t *BT) pullKey() resource.Pull {
	return newPull(t.imageEngine, t.imageStore, t.Image, t.RegistryCreds, t.Path)
}

// Prefetch implements resource.Prefetcher: it pulls the image when the store
// of the engine does not hold it, and never pulls again one it holds,
// whatever image_pull_policy says of the starts.
//
// The outcome is recorded on the node, by the agent and not by the user of a
// rootless container. An engine that does not answer records none, and is
// the error returned: nothing is known of the image.
func (t *BT) Prefetch(ctx context.Context) (resource.PrefetchReport, error) {
	report := resource.PrefetchReport{RID: t.RID()}
	if t.executer == nil {
		return report, fmt.Errorf("prefetch: undefined executer")
	}
	if t.imageEngine == "" || !isImageOperand(t.Image) {
		return report, fmt.Errorf("prefetch: no image to pull")
	}
	report.Key = t.pullKey()
	has, id, err := t.executer.HasImage(ctx)
	switch {
	case err != nil:
		return report, fmt.Errorf("unable to detect if image %s exists locally: %w", t.Image, err)
	case has:
		report.State = resource.PrefetchPresent
		report.ImageID = id
	default:
		if err := t.pull(ctx); err != nil {
			report.State = resource.PrefetchFailed
			report.Error = err.Error()
		} else {
			report.State = resource.PrefetchPulled
			if _, id, err := t.executer.HasImage(ctx); err == nil {
				report.ImageID = id
			}
		}
	}
	report.UpdatedAt = time.Now()
	if err := report.PrefetchOutcome.Write(); err != nil {
		return report, fmt.Errorf("prefetch: record the outcome: %w", err)
	}
	return report, nil
}

// warnPrefetchFailed warns, in the status of a resource with no container,
// or with a stopped one its start removes, of an image the store of the
// engine does not hold after a prefetch that failed: a failover to this
// node would pull it first.
//
// The outcome of the prefetch drives the warning, not the missing image
// alone: a prefetch not done yet, as after a deploy, is no fault to report,
// and its image is not even looked for. One this resource disables is not
// read, and one its node disables has no outcome left: the worker removes
// the outcome of a key no resource pulls, and the status then no longer
// warns. An image missing is told from an engine that does not answer,
// which says nothing of the image.
func (t *BT) warnPrefetchFailed(ctx context.Context) {
	if t.ImagePrefetch == "false" || t.imageEngine == "" || t.executer == nil {
		return
	}
	outcome, ok := resource.ReadPrefetchOutcome(t.pullKey())
	if !ok || outcome.State != resource.PrefetchFailed {
		return
	}
	if has, _, err := t.executer.HasImage(ctx); err != nil || has {
		return
	}
	t.StatusLog().Warn("image %s is not on this node and its last prefetch failed (%s): a failover here pulls it first, slow or failing if the registry is unreachable", t.Image, outcome.Error)
}
