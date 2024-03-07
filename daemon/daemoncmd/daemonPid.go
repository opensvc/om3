package daemoncmd

import (
	"os"
	"os/exec"
	"strconv"
)

func extractPidFromPidFile(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(string(data[:len(data)-1]))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func executeCmdPsPipeGrep() (string, error) {
	psCmd := exec.Command("ps", "afx")
	grepCmd := exec.Command("grep", "[o]m daemon start")

	pipe, err := psCmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	grepCmd.Stdin = pipe

	err = psCmd.Start()
	if err != nil {
		return "", err
	}

	output, err := grepCmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func killDaemonPid(pid int) error {
	killCmd := exec.Command("sudo", "kill", "-9", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		return err
	}
	return nil
}
