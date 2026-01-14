package getaddr

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	ErrCacheAddr struct {
		name string
	}

	ErrManyAddr struct {
		name  string
		count int
	}
)

var (
	// netLookupIP is here to facilitate mocking in tests
	netLookupIP = net.LookupIP

	// cacheDir is where to store the successful lookup results
	cacheDir = filepath.Join(rawconfig.Paths.Var, "cache", "addrinfo")
)

func (t ErrManyAddr) Error() string {
	return fmt.Sprintf("name %s resolves to %d address", t.name, t.count)
}

func (t ErrCacheAddr) Error() string {
	return fmt.Sprintf("error caching the name %s addr", t.name)
}

func IsErrManyAddr(err error) bool {
	var e ErrManyAddr
	return errors.As(err, &e)
}

func fmtCacheFile(name string) string {
	return filepath.Join(cacheDir, name)
}

func load(name string) (net.IP, time.Duration, error) {
	cacheFilename := fmtCacheFile(name)
	stat, err := os.Stat(cacheFilename)
	if err != nil {
		return nil, 0, err
	}
	b, err := os.ReadFile(cacheFilename)
	if err != nil {
		return nil, 0, err
	}
	return net.ParseIP(string(b)), time.Since(stat.ModTime()), nil
}

func cache(name string, ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("refuse to cache invalid ip")
	}
	cacheFilename := fmtCacheFile(name)
	write := func() error {
		return os.WriteFile(cacheFilename, []byte(ip.String()), 0o0644)
	}
	if err := write(); !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return err
	}
	return write()
}

func lookup(name string) (net.IP, error) {
	var (
		l   []net.IP
		err error
	)
	l, err = netLookupIP(name)
	if err != nil {
		return nil, err
	}
	n := len(l)
	switch n {
	case 0:
		return nil, fmt.Errorf("name %s is unresolvable", name)
	case 1:
		// ok
	default:
		return l[0], ErrManyAddr{name: name, count: n}
	}
	return l[0], nil
}

func lookupAndCache(name string) (net.IP, error) {
	if !naming.IsValidFQDN(name) && !hostname.IsValid(name) {
		ip := net.ParseIP(name)
		if ip == nil {
			return nil, fmt.Errorf("unparsable ip %s", name)
		}
		return ip, nil
	}
	ip, err := lookup(name)
	if err == nil || IsErrManyAddr(err) {
		if err := cache(name, ip); err != nil {
			return ip, fmt.Errorf("%w: %w", ErrCacheAddr{name: name}, err)
		}
	}
	return ip, err
}

func Lookup(name string) (net.IP, time.Duration, error) {
	ip, err := lookupAndCache(name)
	if err == nil {
		return ip, 0, nil
	}
	return load(name)
}
