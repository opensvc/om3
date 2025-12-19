package datarecv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	KeyMeta struct {
		Key  string
		Path naming.Path
		From string
	}

	pather interface {
		Path() naming.Path
	}
)

func ParseKeyMetaRelObj(line string, i any) (KeyMeta, error) {
	o, ok := i.(pather)
	if !ok {
		return KeyMeta{}, fmt.Errorf("%s is not a correct type accessing a datastore key", i)
	}
	return ParseKeyMetaRel(line, o.Path().Namespace)
}

func ParseKeyMetaRel(line, namespace string) (KeyMeta, error) {
	var (
		km   KeyMeta
		word string
	)
	km.From = namespace
	if km.From == "" {
		return km, fmt.Errorf("empty key reader namespace")
	}
	words := strings.Fields(line)
	for {
		word, words = pop(words)
		if word == "" {
			break
		}
		switch word {
		case "from":
			word, words = pop(words)
			path, err := naming.ParsePathRel(word, namespace)
			if err != nil {
				return km, err
			}
			km.Path = path
		case "key":
			word, words = pop(words)
			km.Key = word
		}
	}
	if km.Key == "" {
		return km, fmt.Errorf("key name not found in key reference")
	}
	if km.Path.IsZero() {
		return km, fmt.Errorf("datastore path not found in key reference")
	}
	return km, nil
}

func (t *KeyMeta) RootDecode() ([]byte, error) {
	ds, err := object.NewDataStore(t.Path, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	return ds.DecodeKey(t.Key)
}

func (t *KeyMeta) Decode() ([]byte, error) {
	ds, err := object.NewDataStore(t.Path, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	if !ds.Allow(t.From) {
		return nil, fmt.Errorf("the %s namespace is not allowed to access %s keys", t.From, t.Path)
	}
	return ds.DecodeKey(t.Key)
}

func (t *KeyMeta) CacheFile() (string, error) {
	ds, err := object.NewDataStore(t.Path, object.WithVolatile(true))
	if !ds.Allow(t.From) {
		return "", fmt.Errorf("the %s namespace is not allowed to access %s keys", t.From, t.Path)
	}
	filename := filepath.Join(rawconfig.Paths.Run, t.Path.FQN(), "key", t.Key)
	dsModTime, err := t.Path.ModTime()
	if err != nil {
		return "", err
	}
	kvInstall := object.KVInstall{
		Required:    true,
		ToPath:      filename,
		FromPattern: t.Key,
		FromStore:   t.Path,
		AccessControl: object.KVInstallAccessControl{
			User:         "root",
			Group:        "root",
			Perm:         &defaultSecPerm,
			MakedirUser:  "root",
			MakedirGroup: "root",
			MakedirPerm:  &defaultDirPerm,
		},
	}
	fileinfo, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		// cache file does not exist... install
		if err := ds.InstallKeyTo(kvInstall); err != nil {
			return "", err
		}
		return filename, nil
	} else if err != nil {
		return "", err
	}

	if fileinfo.ModTime() == dsModTime {
		// cache file is up to date... serve as is
		return filename, nil
	}

	// cache file is outdated... reinstall
	if err := ds.InstallKeyTo(kvInstall); err != nil {
		return "", err
	}
	return filename, nil

}
