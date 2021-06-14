// +build linux

package lvm2

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
)

type (
	LVData struct {
		Report LVReport `json:"report"`
	}
	LVReport struct {
		LV []LVInfo `json:"lv"`
	}
	LVInfo struct {
		LVName          string `json:"lv_name"`
		VGName          string `json:"vg_name"`
		LVAttr          string `json:"lv_attr"`
		LVSize          string `json:"lv_name"`
		Origin          string `json:"origin"`
		DataPercent     string `json:"data_percent"`
		CopyPercent     string `json:"copy_percent"`
		MetadataPercent string `json:"metadata_percent"`
		MovePV          string `json:"move_pv"`
		ConvertPV       string `json:"convert_pv"`
		MirrorLog       string `json:"mirror_log"`
	}
)

func FullName(vg string, lv string) string {
	return fmt.Sprintf("%s/%s", vg, lv)
}

func LVActivate(vg string, lv string, log *zerolog.Logger) error {
	return lvChange(vg, lv, log, []string{"-ay"})
}

func LVDeactivate(vg string, lv string, log *zerolog.Logger) error {
	return lvChange(vg, lv, log, []string{"-an"})
}

func lvChange(vg string, lv string, log *zerolog.Logger, args []string) error {
	fullname := FullName(vg, lv)
	cmd := command.New(
		command.WithName("lvchange"),
		command.WithArgs(append(args, fullname)),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
		//return fmt.Errorf(cmd.GetStderr())
	}
	return nil
}

func LVAttr(vg string, lv string, log *zerolog.Logger) (*LVInfo, error) {
	data := LVData{}
	fullname := FullName(vg, lv)
	cmd := command.New(
		command.WithName("lvs"),
		command.WithVarArgs(fullname),
		command.WithLogger(log),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	cmd.Run()
	b, err := cmd.Cmd().Output()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	if len(data.Report.LV) == 1 {
		return &data.Report.LV[0], nil
	}
	return nil, fmt.Errorf("lv %s not found", fullname)
}
