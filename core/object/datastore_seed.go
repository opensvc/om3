package object

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"syscall"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/random"
	"github.com/opensvc/om3/v3/util/uri"
)

var (
	// ErrSeedOtherNamespace is the refusal to seed a key of a datastore of
	// another namespace. A datastore sharing its keys with a namespace lets
	// it read them: writing one is not the reader's to do.
	ErrSeedOtherNamespace = errors.New("a key of a datastore of another namespace is not seeded")

	// ErrSourceNotRegular is the refusal to read a source that is no
	// regular file: a device as /dev/urandom never ends, a fifo blocks
	// until a writer comes, and a directory is no content.
	ErrSourceNotRegular = errors.New("not a regular file")
)

// ErrRandomKeyMissing is a key generated, "source random", missing where it
// is used. The provision of the object generates it once, on its leader
// node: a key made anywhere else would differ from the one each other node
// reads, so it is an error, and never a password made on the spot.
type ErrRandomKeyMissing struct {
	Path naming.Path
	Key  string
}

func (t ErrRandomKeyMissing) Error() string {
	return fmt.Sprintf("the key %s of %s, generated from source random by the provision of the object on its leader node, does not exist: provision the object, or add the key", t.Key, t.Path)
}

// HasDataStoreKey says the datastore p exists and holds the key.
func HasDataStoreKey(p naming.Path, keyName string) bool {
	if !p.Exists() {
		return false
	}
	ds, err := NewDataStore(p, WithVolatile(true))
	if err != nil {
		return false
	}
	return ds.HasKey(keyName)
}

// SeedKey writes the key of the datastore from the source, a key the
// datastore does not hold, for the object p, whose resource rid seeds it:
// an empty rid is the object, seeding for all its resources.
//
// The source is an http or https url, fetched as the node.allowed_urls and
// SSRF policies allow, "random", a password generated, or the path of a
// regular file of the node. A template is seeded as it is written, and
// rendered on each node at each install.
//
// The key is written for the object p, and the refresh of the consumers of
// the datastore skips the resources of p seeding it, which install it next.
// The datastore is to be of the namespace of p.
func SeedKey(ds DataStore, keyName, source string, log *plog.Logger, p naming.Path, rid string) error {
	if ds.Path().Namespace != p.Namespace {
		return fmt.Errorf("%s key %s: %w", ds.Path(), keyName, ErrSeedOtherNamespace)
	}
	if source == keywords.SourceRandom {
		log.Infof("generating key %s of %s", keyName, ds.Path())
	} else {
		log.Infof("seeding key %s of %s from source %s", keyName, ds.Path(), source)
	}
	data, err := readSource(source)
	if err != nil {
		return err
	}

	// The key written, the datastore refreshes the other services
	// installing its keys: their failures are theirs, warned of, and not
	// the one of this seeding.
	warnings, err := ConsumersInstallWarnings(ds.AddKeySeeded(keyName, data, p, rid))
	if err != nil {
		return fmt.Errorf("failed to add key %s to datastore: %w", keyName, err)
	}
	for _, w := range warnings {
		log.Warnf("seeded key %s, the install it refreshed failed in %s", keyName, w)
	}
	return nil
}

// readSource returns the content of a source: a password generated, the
// body of an http or https url, or the content of a regular file of the
// node.
func readSource(source string) ([]byte, error) {
	switch {
	case source == keywords.SourceRandom:
		return random.Password(32, nil), nil
	case uri.IsValidHttp(source):
		return fetchSource(source)
	default:
		return readSourceFile(source)
	}
}

func fetchSource(source string) ([]byte, error) {
	client, err := uri.SafeHttpClient(source)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client for %s: %w", source, err)
	}
	resp, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", source, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch from %s: HTTP %d", source, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read from %s: %w", source, err)
	}
	return data, nil
}

// readSourceFile reads a source file, a regular one only. The file is
// opened non-blocking, as a fifo would otherwise hold the open until a
// writer comes, and its type is told by the file opened, so a file
// replaced after a check is not the one read.
//
// /dev/urandom is such a file, which "key add --from /dev/urandom" took
// for a generator: here it is a path, and "source random" is the
// generator.
func readSourceFile(source string) ([]byte, error) {
	f, err := os.OpenFile(source, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", source, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", source, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("failed to read file %s: %w: use 'source random' for a generated key", source, ErrSourceNotRegular)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", source, err)
	}
	return data, nil
}

// EnsureDataStore returns the datastore p, which it creates when it does
// not exist and is of the namespace given, and says whether it created it.
// A datastore of another namespace is not created: it is the one of that
// namespace to make.
func EnsureDataStore(p naming.Path, namespace string) (DataStore, bool, error) {
	if p.Exists() {
		ds, err := NewDataStore(p)
		if err != nil {
			return nil, false, fmt.Errorf("failed to init existing datastore %s: %w", p, err)
		}
		return ds, false, nil
	}
	if p.Namespace != namespace {
		return nil, false, fmt.Errorf("datastore %s does not exist and namespace %s doesn't match service namespace %s",
			p, p.Namespace, namespace)
	}
	var (
		ds  DataStore
		err error
	)
	switch p.Kind {
	case naming.KindCfg:
		ds, err = NewCfg(p)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create cfg datastore %s: %w", p, err)
		}
	case naming.KindSec:
		ds, err = NewSec(p)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create sec datastore %s: %w", p, err)
		}
	default:
		return nil, false, fmt.Errorf("unsupported datastore kind %s for %s", p.Kind, p)
	}
	if err := ds.Config().Commit(); err != nil {
		return nil, false, fmt.Errorf("failed to save new datastore %s: %w", p, err)
	}
	newDs, err := NewDataStore(p)
	if err != nil {
		return nil, false, fmt.Errorf("failed to reinit datastore %s after creation: %w", p, err)
	}
	return newDs, true, nil
}
