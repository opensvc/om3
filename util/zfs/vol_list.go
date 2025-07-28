package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
	"github.com/rs/zerolog"
)

type (
	DatasetType      int32
	DatasetTypes     []DatasetType
	ListDatasetsOpts struct {
		Names        []string
		Types        DatasetTypes
		Log          *plog.Logger
		OrderBy      []string
		OrderReverse bool
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

func (t DatasetTypes) String() string {
	l := make([]string, 0)
	for _, e := range t {
		if s := e.String(); s != "" {
			l = append(l, e.String())
		}
	}
	return strings.Join(l, ",")
}

func (t DatasetType) String() string {
	return datasetTypeStrMap[t]
}

func ListWithOrderReverse() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*ListDatasetsOpts)
		t.OrderReverse = true
		return nil
	})
}

func ListWithTypes(l ...DatasetType) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*ListDatasetsOpts)
		t.Types = l
		return nil
	})
}

func ListWithNames(l ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*ListDatasetsOpts)
		t.Names = l
		return nil
	})
}

func ListWithOrderBy(l ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*ListDatasetsOpts)
		t.OrderBy = l
		return nil
	})
}

func ListWithLogger(l *plog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*ListDatasetsOpts)
		t.Log = l
		return nil
	})
}

func parseFilesystem(b []byte, opts *ListDatasetsOpts) Filesystems {
	data := make(Filesystems, 0)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		words := strings.Split(line, "\t")
		fs := Filesystem{}
		n := len(words)
		if n != 1 {
			continue
		}
		fs.Name = words[0]
		fs.Log = opts.Log
		data = append(data, fs)
	}
	return data
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

func ListFilesystems(fopts ...funcopt.O) (Filesystems, error) {
	opts := &ListDatasetsOpts{}
	funcopt.Apply(opts, fopts...)
	args := []string{"list", "-Hp", "-o", "name"}
	if opts.Types != nil {
		args = append(args, "-t", opts.Types.String())
	}
	if opts.OrderBy != nil {
		s := strings.Join(opts.OrderBy, ",")
		if opts.OrderReverse {
			args = append(args, "-S", s)
		} else {
			args = append(args, "-s", s)
		}
	}
	if opts.Names != nil {
		args = append(args, opts.Names...)
	}
	cmd := command.New(
		command.WithName("zfs"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithLogger(opts.Log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithIgnoredExitCodes(1),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	if bytes.Contains(cmd.Stderr(), []byte("does not exist")) {
		return Filesystems{}, os.ErrNotExist
	}
	if cmd.ExitCode() != 0 {
		return Filesystems{}, fmt.Errorf("exit status %d", cmd.ExitCode())
	}
	return parseFilesystem(cmd.Stdout(), opts), nil
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
