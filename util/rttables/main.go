package rttables

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type (
	T struct {
		Index int
		Name  string
	}
	L []T
)

// List parses routing table files and returns a list of routing table entries (L). It scans predefined file paths sequentially.
//
// Extract from man ip-route:
//
//	Route tables: Linux-2.x can pack routes into several routing tables identified by a number in the range
//	from 1 to 2^32-1 or by name from /usr/share/iproute2/rt_tables or /etc/iproute2/rt_tables (has precedence
//	if exists).
//
// TODO: add support for rt_tables.d ? (also use <CONF_USR_DIR>/iproute2/rt_tables.d/X.conf files unless
// <CONF_ETC_DIR>/iproute2/rt_tables.d/X.conf exists)
func List() (L, error) {
	const (
		rtTablesFile1 = "/etc/iproute2/rt_tables"
		rtTablesFile2 = "/usr/share/iproute2/rt_tables"
	)
	var (
		scanner *bufio.Scanner
		errs    error
	)
	l := make(L, 0)
	for _, filename := range []string{rtTablesFile1, rtTablesFile2} {
		file, err := os.Open(filename)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		defer func() { _ = file.Close() }()
		scanner = bufio.NewScanner(file)
		break
	}
	if scanner == nil {
		return l, fmt.Errorf("no rt_tables file found: %w", errs)
	}

	for scanner.Scan() {
		s := scanner.Text()
		fields := strings.Fields(s)
		if len(fields) != 2 {
			continue
		}
		t := T{Name: fields[1]}
		if i, err := strconv.Atoi(fields[0]); err != nil {
			continue
		} else {
			t.Index = i
			l = append(l, t)
		}
	}
	if err := scanner.Err(); err != nil {
		return l, err
	}
	return l, nil
}

func ByName(name string) (T, error) {
	l, err := List()
	if err != nil {
		return T{}, err
	}
	return l.ByName(name)
}

func (t L) ByName(name string) (T, error) {
	for _, e := range t {
		if e.Name == name {
			return e, nil
		}
	}
	return T{}, fmt.Errorf("rt table %s not found", name)
}

func Index(name string) (int, error) {
	t, err := ByName(name)
	if err != nil {
		return 0, err
	}
	return t.Index, nil
}
