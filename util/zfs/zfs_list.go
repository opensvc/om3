package zfs

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	Zvol struct {
		Name      ZfsName
		Size      uint64
		BlockSize uint64
	}
	Zvols []Zvol
)

func (t Zvols) Paths() []string {
	l := make([]string, 0)
	for _, zvol := range t {
		l = append(l, "/dev/zvol/"+string(zvol.Name))
	}
	return l
}

func parseZfsListVolumes(b []byte) Zvols {
	data := make(Zvols, 0)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		words := strings.Split(line, "\t")
		zvol := Zvol{}
		n := len(words)
		if n != 3 {
			continue
		}
		zvol.Name = ZfsName(words[0])
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

func (t *Pool) ZfsListVolumes(fopts ...funcopt.O) (Zvols, error) {
	opts := &poolStatusOpts{}
	funcopt.Apply(opts, fopts...)
	cmd := command.New(
		command.WithName("zfs"),
		command.WithVarArgs("list", "-t", "volume", "-Hp", "-o", "name,volsize,volblocksize"),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseZfsListVolumes(b), nil
}
