package zfs

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/rs/zerolog"
)

type (
	DatasetType      int32
	ListDatasetsOpts struct {
		Types []DatasetType
		Log   *zerolog.Logger
	}
)

const (
	// DatasetTypeFilesystem - file system dataset
	DatasetTypeFilesystem DatasetType = (1 << 0)
	// DatasetTypeSnapshot - snapshot of dataset
	DatasetTypeSnapshot = (1 << 1)
	// DatasetTypeVolume - volume (virtual block device) dataset
	DatasetTypeVolume = (1 << 2)
	// DatasetTypePool - pool dataset
	DatasetTypePool = (1 << 3)
	// DatasetTypeBookmark - bookmark dataset
	DatasetTypeBookmark = (1 << 4)
)

var (
	datasetTypeStrMap = map[DatasetType]string{
		DatasetTypeFilesystem: "filesystem",
		DatasetTypeSnapshot:   "snapshot",
		DatasetTypeVolume:     "volume",
		DatasetTypePool:       "pool",
		DatasetTypeBookmark:   "bookmark",
	}
)

func (t DatasetType) String() string {
	return datasetTypeStrMap[t]
}

func ListDatasetsWithLogger(l *zerolog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*ListDatasetsOpts)
		t.Log = l
		return nil
	})
}

func parseVolume(b []byte) Vols {
	data := make(Vols, 0)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		words := strings.Split(line, "\t")
		zvol := Vol{}
		n := len(words)
		if n != 3 {
			continue
		}
		zvol.Name = words[0]
		if i, err := strconv.ParseUint(words[1], 10, 64); err == nil {
			zvol.Size = i
		}
		if i, err := strconv.ParseUint(words[2], 10, 64); err == nil {
			zvol.BlockSize = i
		}
		data = append(data, zvol)
	}
	return data
}

func ListVolumes(fopts ...funcopt.O) (Vols, error) {
	opts := &ListDatasetsOpts{}
	funcopt.Apply(opts, fopts...)
	cmd := command.New(
		command.WithName("zfs"),
		command.WithVarArgs("list", "-t", "volume", "-Hp", "-o", "name,volsize,volblocksize"),
		command.WithBufferedStdout(),
		command.WithLogger(opts.Log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseVolume(b), nil
}
