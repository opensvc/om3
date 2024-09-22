package keystore

import "path/filepath"

func FileToKey(path, prefix string) (string, error) {
	if path == "" {
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
