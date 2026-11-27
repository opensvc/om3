package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/distribution/reference"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	// Pull is the image a resource pulls on a node before it runs its
	// container, and where to: the key of a prefetch. Two resources
	// saying the same Pull on a node need one pull, whatever their
	// objects.
	//
	// It names things and holds none: the daemon reads it to decide what
	// to prefetch, and the pull itself is run by the command of an
	// object, the only one reading its credentials.
	Pull struct {
		// Engine is the engine holding the image, as "docker" or
		// "podman".
		Engine string `json:"engine"`

		// Store tells the stores of an engine apart, as the driver
		// names them, and is empty for the store of the system. For
		// podman, it is the user of a rootless container, whose
		// images are in a store of their own.
		Store string `json:"store,omitempty"`

		// Image is the image reference, normalized as docker reads it:
		// "docker.io/library/nginx:latest" for "nginx". See
		// NormalizeImage.
		//
		// An engine resolving the names with no registry its own way,
		// as podman, keeps such a name as written, with its tag:
		// "nginx:latest" for "nginx". See NormalizeQualifiedImage.
		Image string `json:"image"`

		// Creds is the identity of the credentials the pull logs in
		// the registry with, as the path of a secret, and is empty
		// for the credentials of the node. It is not the credentials:
		// two resources pulling the same image with different ones
		// are two pulls, one of which may be refused.
		Creds string `json:"creds,omitempty"`
	}

	// PullDescriber is implemented by the drivers whose resources pull an
	// image. It is called on a driver allocated and not configured, as
	// Holder is: the Pull comes from the source alone, and nothing runs.
	// It returns false for a resource whose image does not evaluate.
	PullDescriber interface {
		PullOf(HoldSource) (Pull, bool)
	}

	// Prefetcher is implemented by the drivers able to pull the image of
	// their resource ahead of its start, when the store of the engine
	// does not hold it. The outcome is in the report, and recorded on
	// the node for the status of the resource and for the daemon: an
	// error is returned only for what prevented the try.
	Prefetcher interface {
		Prefetch(context.Context) (PrefetchReport, error)
	}

	// PrefetchReport is what a prefetch of the image of a resource did.
	PrefetchReport struct {
		RID string `json:"rid"`
		PrefetchOutcome
	}

	// PrefetchOutcome is the outcome of the last prefetch of a Pull on
	// the node, as recorded in its file.
	PrefetchOutcome struct {
		Key Pull `json:"key"`

		// State is PrefetchPresent, PrefetchPulled or PrefetchFailed.
		State string `json:"state"`

		// ImageID is the id of the image in the store, when there.
		ImageID string `json:"image_id,omitempty"`

		// Error is why the prefetch failed.
		Error string `json:"error,omitempty"`

		// UpdatedAt is when the outcome was recorded, by the clock of
		// the node. It is read by humans, and compared to the one of
		// the outcome read before a prefetch only, by the daemon, to
		// tell the outcome this prefetch recorded from the former one
		// left in the file: never to another clock, nor to now.
		UpdatedAt time.Time `json:"updated_at"`
	}

	// ImageStore answers for the image stores of an engine, without the
	// configuration of a resource: the daemon asks it whether a prefetch
	// has anything to pull before running the command of an object.
	//
	// It reads the stores, and nothing else: it needs no credentials, and
	// changes nothing.
	ImageStore interface {
		// Has says whether the store the Pull names holds its image,
		// and the id of the image. An engine that does not answer is
		// an error, not a missing image.
		Has(context.Context, Pull) (present bool, id string, err error)
	}
)

// The states of a prefetch outcome.
const (
	// PrefetchPresent is the outcome of a prefetch finding the image in
	// the store.
	PrefetchPresent = "present"

	// PrefetchPulled is the outcome of a prefetch that pulled the image.
	PrefetchPulled = "pulled"

	// PrefetchFailed is the outcome of a prefetch that could not pull the
	// image.
	PrefetchFailed = "failed"
)

var (
	imageStoresMu sync.RWMutex
	imageStores   = make(map[string]ImageStore)
)

// RegisterImageStore sets the image store of an engine, as the drivers of
// the engine do when registered.
func RegisterImageStore(engine string, store ImageStore) {
	imageStoresMu.Lock()
	defer imageStoresMu.Unlock()
	imageStores[engine] = store
}

// ImageStoreOf returns the image store of an engine, nil when no driver
// registered one.
func ImageStoreOf(engine string) ImageStore {
	imageStoresMu.RLock()
	defer imageStoresMu.RUnlock()
	return imageStores[engine]
}

// NormalizeImage returns the image reference s as docker reads it, with the
// registry, the namespace and the tag it defaults to: "nginx" is
// "docker.io/library/nginx:latest". A reference with a digest loses its tag,
// which the digest overrides.
//
// A string that is no docker reference, as one naming a transport of podman,
// is returned as it is: the engine has the last word on what it pulls.
func NormalizeImage(s string) string {
	s = strings.TrimSpace(s)
	named, err := reference.ParseDockerRef(s)
	if err != nil {
		return s
	}
	return named.String()
}

// NormalizeQualifiedImage returns the image reference s as NormalizeImage
// does when it names its registry, and with its name as written when it
// names none: "ghcr.io/opensvc/pause" is "ghcr.io/opensvc/pause:latest",
// and "nginx" is "nginx:latest".
//
// It is the normalization of the engines that do not read a name with no
// registry as one of the docker hub. Podman resolves it with the aliases
// and the search registries of its registries.conf, where "nginx" may be
// the image of another registry: a key saying "docker.io/library/nginx"
// would name an image the store does not hold, and a start does not run.
// The name as written is resolved by the engine, as a start resolves it.
//
// The tag and the digest are the ones the engines read: the "latest" tag
// for a reference with none, and the digest alone for a reference with
// both.
func NormalizeQualifiedImage(s string) string {
	s = strings.TrimSpace(s)
	if ImageNamesRegistry(s) {
		return NormalizeImage(s)
	}
	named, err := reference.ParseDockerRef(s)
	if err != nil {
		return s
	}
	name := s
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	if i := strings.LastIndexByte(name, ':'); i > strings.LastIndexByte(name, '/') {
		name = name[:i]
	}
	switch ref := named.(type) {
	case reference.Digested:
		return name + "@" + ref.Digest().String()
	case reference.Tagged:
		return name + ":" + ref.Tag()
	}
	return s
}

// ImageNamesRegistry says whether the image reference s starts with a
// registry, as the engines tell one from the first component of a path: it
// has a '.' or a ':', is "localhost", or has an upper case letter, which a
// repository can not have.
func ImageNamesRegistry(s string) bool {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return false
	}
	host := s[:i]
	return strings.ContainsAny(host, ".:") || host == "localhost" || strings.ToLower(host) != host
}

// IsZero says whether the Pull names no image.
func (t Pull) IsZero() bool {
	return t.Image == ""
}

// String describes the Pull for a log line.
func (t Pull) String() string {
	s := t.Engine + " image " + t.Image
	if t.Store != "" {
		s += " in the store of " + t.Store
	}
	if t.Creds != "" {
		s += " with the credentials of " + t.Creds
	}
	return s
}

// Hash returns a name for the Pull, the same on every node and safe in a
// file name.
func (t Pull) Hash() string {
	h := sha256.New()
	for _, s := range []string{t.Engine, t.Store, t.Image, t.Creds} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// PrefetchDir is where the node records the outcomes of its prefetches.
func PrefetchDir() string {
	return filepath.Join(rawconfig.NodeVarDir(), "prefetch")
}

// prefetchOutcomeFile is the file of the outcome of the last prefetch of
// pull. The engine is a word of the configuration: its base name alone is
// used.
func prefetchOutcomeFile(pull Pull) string {
	engine := filepath.Base(filepath.Clean("/" + pull.Engine))
	return filepath.Join(PrefetchDir(), engine, pull.Hash()+".json")
}

// ReadPrefetchOutcome returns the outcome of the last prefetch of pull on
// the node, and false when none is recorded.
func ReadPrefetchOutcome(pull Pull) (PrefetchOutcome, bool) {
	outcome, err := readPrefetchOutcomeFile(prefetchOutcomeFile(pull))
	if err != nil || outcome.Key != pull {
		return PrefetchOutcome{}, false
	}
	return outcome, true
}

func readPrefetchOutcomeFile(path string) (PrefetchOutcome, error) {
	var outcome PrefetchOutcome
	b, err := os.ReadFile(path)
	if err != nil {
		return outcome, err
	}
	err = json.Unmarshal(b, &outcome)
	return outcome, err
}

// Write records the outcome on the node, in the place of the former outcome
// of its Pull. The file is written whole then renamed: its readers, the
// status of the resources and the daemon, never read half of it.
func (t PrefetchOutcome) Write() error {
	path := prefetchOutcomeFile(t.Key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// RemovePrefetchOutcome forgets the outcome of the last prefetch of pull.
func RemovePrefetchOutcome(pull Pull) error {
	err := os.Remove(prefetchOutcomeFile(pull))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// PrefetchOutcomes returns the outcomes recorded on the node. A file that
// does not read as an outcome is reported with the zero Pull, for its
// removal by RemovePrefetchOutcomeFile.
func PrefetchOutcomes() map[string]PrefetchOutcome {
	m := make(map[string]PrefetchOutcome)
	paths, _ := filepath.Glob(filepath.Join(PrefetchDir(), "*", "*"))
	for _, path := range paths {
		if strings.HasPrefix(filepath.Base(path), ".") {
			// An outcome being written, see Write.
			continue
		}
		outcome, err := readPrefetchOutcomeFile(path)
		if err != nil || path != prefetchOutcomeFile(outcome.Key) {
			outcome = PrefetchOutcome{}
		}
		m[path] = outcome
	}
	return m
}

// RemovePrefetchOutcomeFile removes a file PrefetchOutcomes reported.
func RemovePrefetchOutcomeFile(path string) error {
	if filepath.Dir(filepath.Dir(path)) != PrefetchDir() {
		return errors.New("not a prefetch outcome file: " + path)
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
