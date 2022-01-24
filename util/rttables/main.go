package rttables

import (
	"bufio"
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

const rtTablesFile = "/etc/iproute2/rt_tables"

func List() (L, error) {
	l := make(L, 0)
	file, err := os.Open(rtTablesFile)
	if err != nil {
		return l, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
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
