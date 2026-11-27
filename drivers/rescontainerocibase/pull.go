package rescontainerocibase

import (
	"context"
	"errors"
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

// Prefetch implements resource.Prefetcher: it gets the image when the store
// of the engine does not hold it, and never gets again one it holds,
// whatever image_pull_policy says of the starts.
//
// The image is copied from the first of the sources holding it, and pulled
// from its registry when none does, or when none is given: the nodes of a
// cluster with no registry hold the images loaded on one of them by hand.
// The outcome tells the source, and a failure what both said.
//
// The outcome is recorded on the node, by the agent and not by the user of a
// rootless container. An engine that does not answer records none, and is
// the error returned: nothing is known of the image.
func (t *BT) Prefetch(ctx context.Context, sources []resource.ImageSource) (resource.PrefetchReport, error) {
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
		node, id, peersErr := t.copyImage(ctx, report.Key, sources)
		if node != "" {
			report.State = resource.PrefetchPulled
			report.Source = resource.PrefetchSourcePeer(node)
			report.ImageID = id
			break
		}
		if err := ctx.Err(); err != nil {
			// The copies were cut with the command: a pull would be
			// too, and its failure say nothing of the registry.
			return report, fmt.Errorf("prefetch: %w", err)
		}
		if err := t.pull(ctx); err != nil {
			report.State = resource.PrefetchFailed
			report.Error = prefetchError(peersErr, err)
		} else {
			report.State = resource.PrefetchPulled
			report.Source = resource.PrefetchSourceRegistry
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

// prefetchError is the error of a prefetch whose pull failed with pullErr,
// after the nodes it asked for a copy, when it asked some, failed with
// peersErr.
func prefetchError(peersErr, pullErr error) string {
	if peersErr == nil {
		return pullErr.Error()
	}
	return fmt.Sprintf("peers: %s; registry: %s", peersErr, pullErr)
}

// nodePlatform is the platform of this node, replaced by the tests.
var nodePlatform = resource.NodePlatform

// copyImage copies the image of key from the first of the sources holding
// it, and returns the node it came from, with the id of the image. With no
// node to return, the error says why of each source asked, and is nil when
// none was.
//
// The sources are asked in their order for the image in the store of the
// engine of the key, then for the one in the store of the system of the
// other engines, which save an archive this one loads: a node holding the
// image the same way is preferred, having it as a start here would. A node
// of another platform is asked once: its images are built for it.
//
// The other engines are asked only for an image naming its registry, which
// they all read the same. A name with no registry, as podman keeps it in
// its key, is resolved by each engine its own way: the docker of a node
// holds under "nginx:latest" the image of its hub, where the podman of
// this one may resolve it to another registry, and the ids of the two
// stores would agree on the wrong image. Such an image is copied from the
// nodes of its engine, then pulled from the registry its engine resolves.
//
// No credentials are used: the nodes hand the image they hold to the
// agent of a node of their cluster.
func (t *BT) copyImage(ctx context.Context, key resource.Pull, sources []resource.ImageSource) (string, string, error) {
	if len(sources) == 0 {
		return "", "", nil
	}
	engines := []string{key.Engine}
	if resource.ImageNamesRegistry(key.Image) {
		for _, engine := range resource.ImageStoreEngines() {
			if engine != key.Engine {
				engines = append(engines, engine)
			}
		}
	}
	reasons := make(map[string]error)
	foreign := make(map[string]bool)
	for _, engine := range engines {
		ask := resource.Pull{Engine: engine, Image: key.Image}
		if engine == key.Engine {
			ask.Store = key.Store
		}
		for _, source := range sources {
			node := source.Node()
			if foreign[node] {
				continue
			}
			id, err := t.copyImageFrom(ctx, source, ask, key)
			if err == nil {
				t.Log().Infof("image %s copied from the %s store of %s", t.Image, engine, node)
				return node, id, nil
			}
			if ctx.Err() != nil {
				return "", "", ctx.Err()
			}
			var platformErr *platformError
			if errors.As(err, &platformErr) {
				foreign[node] = true
			}
			// What a node says of the store of the engine of the
			// key is kept, unless it only said it does not hold
			// the image, and says more of another store.
			if former, ok := reasons[node]; !ok || (errors.Is(former, resource.ErrImageNotFound) && !errors.Is(err, resource.ErrImageNotFound)) {
				reasons[node] = err
			}
			t.Log().Debugf("image %s not copied from the %s store of %s: %s", t.Image, engine, node, err)
		}
	}
	l := make([]string, 0, len(sources))
	for _, source := range sources {
		if err, ok := reasons[source.Node()]; ok {
			l = append(l, fmt.Sprintf("%s: %s", source.Node(), err))
			delete(reasons, source.Node())
		}
	}
	return "", "", errors.New(strings.Join(l, ", "))
}

// platformError is the error of a copy from a node of another platform.
type platformError struct {
	platform string
}

func (t *platformError) Error() string {
	return fmt.Sprintf("a %s node, this one is %s", t.platform, nodePlatform())
}

// imageIDKind says what the ids of the store of key are the digest of, and
// nothing when the engine does not tell. It is replaced by the tests.
var imageIDKind = func(ctx context.Context, key resource.Pull) string {
	store := resource.ImageStoreOf(key.Engine)
	if store == nil {
		return ""
	}
	kind, err := store.IDKind(ctx, key)
	if err != nil {
		return ""
	}
	return kind
}

// copyImageFrom loads the archive of the image ask names, as source sends
// it, and returns the id of the image. The archive goes from the node to
// the engine as it comes, and is kept nowhere.
//
// The store is then asked for the image of the resource, which is what a
// start would run: the archive loaded has to have brought it, or the copy
// failed. Its id has to be the one the node announced, when the two stores
// name an image the same: the archive of another image, as of a reference
// the node gave another image while it saved, is not what was asked for.
// The name the load gave that image is then removed, for no start to run
// it, and the error says so. An archive cut short is refused by the engine.
//
// Two stores not naming an image the same, as a docker keeping its images
// in containerd and one that does not, or one of them not telling, have no
// ids to compare: the image under its name is the copy.
//
// The copy is bound by the pull_timeout, as a pull is: a node that stalls
// leaves their time to the others, and to the registry.
func (t *BT) copyImageFrom(ctx context.Context, source resource.ImageSource, ask, key resource.Pull) (string, error) {
	if t.PullTimeout != nil && *t.PullTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.PullTimeout)
		defer cancel()
	}
	archive, err := source.Save(ctx, ask)
	if err != nil {
		return "", err
	}
	defer func() { _ = archive.Body.Close() }()
	if archive.Platform != nodePlatform() {
		return "", &platformError{platform: archive.Platform}
	}
	if err := t.executer.LoadImage(ctx, archive.Body); err != nil {
		return "", fmt.Errorf("load: %w", err)
	}
	has, id, err := t.executer.HasImage(ctx)
	switch {
	case err != nil:
		return "", err
	case !has:
		return "", fmt.Errorf("the archive loaded did not bring image %s", t.Image)
	}
	if kind := imageIDKind(ctx, key); kind == "" || kind != archive.IDKind {
		return id, nil
	}
	if sameImageID(id, archive.ID) {
		return id, nil
	}
	if err := t.executer.RemoveImage(ctx); err != nil {
		return "", fmt.Errorf("the image loaded is %s, not the %s announced, and its removal failed: %w", id, archive.ID, err)
	}
	return "", fmt.Errorf("the image loaded is %s, not the %s announced: removed", id, archive.ID)
}

// sameImageID says whether two ids, as two engines print them, are the id
// of one image: docker prefixes the digest with its algorithm, and podman
// does not.
func sameImageID(a, b string) bool {
	const prefix = "sha256:"
	a, b = strings.TrimPrefix(a, prefix), strings.TrimPrefix(b, prefix)
	return a != "" && a == b
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
