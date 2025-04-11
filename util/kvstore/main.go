package kvstore

import (
	"path/filepath"

	"github.com/opensvc/om3/util/file"
)

func FileToKey(path, prefix, from string) (string, error) {
	if path == "" {
		if prefix == "" && from != "" {
			if v, err := file.ExistsAndRegular(from); err != nil {
				return "", err
			} else if v {
				return filepath.Base(from), nil
			}
		}
		return prefix, nil
	}
	path = filepath.Clean(path)
	dirName := filepath.Dir(path)
	relPath, err := filepath.Rel(dirName, path)
	if prefix == "" {
		return relPath, err
	}
	return filepath.Join(prefix, relPath), nil
}
