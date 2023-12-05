package sysproc

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	GetInodeFromTcpFileContent = fGetInodeFromTcpFileContent

	getParentPid   = fGetParentPid
	GetPidFromPort = fGetPidFromPort

	osReadDir  = os.ReadDir
	osReadFile = os.ReadFile
	osReadLink = os.Readlink
)

func fGetPidFromPort(port int) (int, error) {
	socketMap, err := getSocketsMap()
	if err != nil {
		return -1, err
	}
	inode, err := getInodeListeningOnPort(port)
	if err != nil {
		return -1, err
	}
	return getParentPid(socketMap[inode])
}

func getInodeListeningOnPort(port int) (int, error) {
	procDir, err := osReadDir("/proc")
	if err != nil {
		return -1, err
	}
	for _, proc := range procDir {
		if proc.IsDir() {
			tcpFileContent, err := osReadFile(filepath.Join("/proc", proc.Name(), "net", "tcp"))
			if err != nil {
				// silently ignore error, things are changing...
				continue
			}
			tcp6FileContent, err := osReadFile(filepath.Join("/proc", proc.Name(), "net", "tcp6"))
			if err != nil {
				// silently ignore error, things are changing...
				continue
			}

			inode, err := GetInodeFromTcpFileContent(port, tcpFileContent)
			if err != nil {
				return -1, err
			}
			if inode != -1 {
				return inode, nil
			}

			inode, err = GetInodeFromTcpFileContent(port, tcp6FileContent)
			if err != nil {
				return -1, err
			}
			if inode != -1 {
				return inode, nil
			}
		}
	}
	return -1, fmt.Errorf("there is no process listening on port %d", port)
}

func fGetParentPid(pid int) (int, error) {
	strPid := strconv.Itoa(pid)
	statContent, err := osReadFile(filepath.Join("/proc", strPid, "stat"))
	if err != nil {
		return -1, err
	}
	splitLine := strings.Fields(string(statContent))
	if len(splitLine) < 4 {
		return -1, fmt.Errorf("the stat file of the pid %s, is in the wrong format", strPid)
	}
	strPpid := splitLine[3]
	pidExeTarget, err := osReadLink(filepath.Join("/proc", strPid, "exe"))
	if err != nil {
		return -1, err
	}
	ppidExeTarget, err := osReadLink(filepath.Join("/proc", strPpid, "exe"))
	if err != nil {
		return -1, err
	}
	if pidExeTarget == ppidExeTarget {
		ppid, err := strconv.Atoi(strPpid)
		if err != nil {
			return -1, err
		}
		return fGetParentPid(ppid)
	}
	return pid, nil
}

func getSocketsMap() (map[int]int, error) {
	socketsMap := map[int]int{}
	procDir, err := osReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, proc := range procDir {
		pid, _ := strconv.Atoi(proc.Name())
		if pid == 1 {
			continue
		}
		if proc.IsDir() {
			fdsName := filepath.Join("/proc", proc.Name(), "fd")
			fds, err := osReadDir(fdsName)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("getSocketsMap readir %s: %w", fdsName, err)
			}
			for _, fd := range fds {
				fdName := filepath.Join(fdsName, fd.Name())
				link, err := osReadLink(fdName)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, err
				}
				splitLink := strings.Split(link, "[")
				if splitLink[0] == "socket:" && len(splitLink) == 2 {
					if len(splitLink[1]) > 1 {
						inode, err := strconv.Atoi(splitLink[1][:len(splitLink[1])-1])
						if err != nil {
							return nil, fmt.Errorf("getSocketsMap can't parse port from %s: %w", fdName, err)
						}
						socketsMap[inode] = pid
					}
				}
			}
		}
	}
	return socketsMap, nil
}

func fGetInodeFromTcpFileContent(port int, content []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		splitLine := strings.Fields(scanner.Text())
		if len(splitLine) < 10 {
			continue
		}
		var splitAddress = strings.Split(splitLine[1], ":")
		if len(splitAddress) != 2 {
			continue
		}
		portUsed, err := strconv.ParseInt(splitAddress[1], 16, 64)
		if err != nil {
			return -1, err
		}
		if int(portUsed) == port {
			inode, err := strconv.Atoi(splitLine[9])
			if err != nil {
				return -1, err
			}
			return inode, nil
		}
	}
	return -1, nil
}
