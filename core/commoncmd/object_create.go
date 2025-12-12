package commoncmd

import (
	"fmt"
	"io"
	"os"

	"github.com/opensvc/om3/v3/util/uri"
)

func DataFromConfigURI(u uri.T) ([]byte, error) {
	fpath, err := u.Fetch()
	if err != nil {
		return nil, nil
	}
	defer os.Remove(fpath)
	return DataFromConfigFile(fpath)
}

func DataFromConfigFile(fpath string) ([]byte, error) {
	b, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	return b, err
}

func DataFromStdin() ([]byte, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return b, err
}

func DataFromTemplate(template string) ([]byte, error) {
	return nil, fmt.Errorf("todo: collector requester")
}
