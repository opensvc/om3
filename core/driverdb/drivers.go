package driverdb

import (
	// Uncomment to load
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/rawconfig"
	_ "github.com/opensvc/om3/v3/drivers/arrayfreenas"
	_ "github.com/opensvc/om3/v3/drivers/arrayhds"
	_ "github.com/opensvc/om3/v3/drivers/arrayhoc"
	_ "github.com/opensvc/om3/v3/drivers/arrayhp3par"
	_ "github.com/opensvc/om3/v3/drivers/arraypure"
	_ "github.com/opensvc/om3/v3/drivers/arraysymmetrix"
	_ "github.com/opensvc/om3/v3/drivers/arrayxtremio"
	_ "github.com/opensvc/om3/v3/drivers/chkbtrfs"
	_ "github.com/opensvc/om3/v3/drivers/chketh"
	_ "github.com/opensvc/om3/v3/drivers/chkfsidf"
	_ "github.com/opensvc/om3/v3/drivers/chkfsudf"
	_ "github.com/opensvc/om3/v3/drivers/chkfszfs"
	_ "github.com/opensvc/om3/v3/drivers/chkjstat"
	_ "github.com/opensvc/om3/v3/drivers/chklag"
	_ "github.com/opensvc/om3/v3/drivers/chkmcelog"
	_ "github.com/opensvc/om3/v3/drivers/chkmpath"
	_ "github.com/opensvc/om3/v3/drivers/chknuma"
	_ "github.com/opensvc/om3/v3/drivers/chkomreport"
	_ "github.com/opensvc/om3/v3/drivers/chkpowerpath"
	_ "github.com/opensvc/om3/v3/drivers/chkraid"
	_ "github.com/opensvc/om3/v3/drivers/chkvg"
	_ "github.com/opensvc/om3/v3/drivers/chkzpool"
	_ "github.com/opensvc/om3/v3/drivers/claimcpu"
	_ "github.com/opensvc/om3/v3/drivers/claimmemory"
	_ "github.com/opensvc/om3/v3/drivers/claimnetwork"
	_ "github.com/opensvc/om3/v3/drivers/claimpool"
	_ "github.com/opensvc/om3/v3/drivers/pooldirectory"
	_ "github.com/opensvc/om3/v3/drivers/poolfreenas"
	_ "github.com/opensvc/om3/v3/drivers/poolhoc"
	_ "github.com/opensvc/om3/v3/drivers/poolpure"
	_ "github.com/opensvc/om3/v3/drivers/poolshm"

	_ "github.com/opensvc/om3/v3/drivers/poolsymmetrix"
	_ "github.com/opensvc/om3/v3/drivers/poolvirtual"
	_ "github.com/opensvc/om3/v3/drivers/poolzpool"
	_ "github.com/opensvc/om3/v3/drivers/resappforking"
	_ "github.com/opensvc/om3/v3/drivers/resappsimple"
	_ "github.com/opensvc/om3/v3/drivers/resdiskdisk"
	_ "github.com/opensvc/om3/v3/drivers/resdiskhp3par"
	_ "github.com/opensvc/om3/v3/drivers/resdiskloop"
	_ "github.com/opensvc/om3/v3/drivers/resdisklv"
	_ "github.com/opensvc/om3/v3/drivers/resdiskmd"
	_ "github.com/opensvc/om3/v3/drivers/resdiskraw"
	_ "github.com/opensvc/om3/v3/drivers/resdisksgcp_nfs_cg"
	_ "github.com/opensvc/om3/v3/drivers/resdiskvg"
	_ "github.com/opensvc/om3/v3/drivers/resdiskxp8"
	_ "github.com/opensvc/om3/v3/drivers/resfsdir"
	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
	_ "github.com/opensvc/om3/v3/drivers/resfshost"
	_ "github.com/opensvc/om3/v3/drivers/resfssgcp_nfs"
	_ "github.com/opensvc/om3/v3/drivers/resfszfs"
	_ "github.com/opensvc/om3/v3/drivers/resiphost"
	_ "github.com/opensvc/om3/v3/drivers/resiproute"
	_ "github.com/opensvc/om3/v3/drivers/resiprule"
	_ "github.com/opensvc/om3/v3/drivers/resipsgcp_dnsalias"
	_ "github.com/opensvc/om3/v3/drivers/ressharenfs"
	_ "github.com/opensvc/om3/v3/drivers/ressyncbtrfs"
	_ "github.com/opensvc/om3/v3/drivers/ressyncbtrfssnap"
	_ "github.com/opensvc/om3/v3/drivers/ressyncrsync"
	_ "github.com/opensvc/om3/v3/drivers/ressyncsymsnapvx"
	_ "github.com/opensvc/om3/v3/drivers/ressyncsymsrdfs"
	_ "github.com/opensvc/om3/v3/drivers/ressynczfs"
	_ "github.com/opensvc/om3/v3/drivers/ressynczfssnap"
	_ "github.com/opensvc/om3/v3/drivers/restaskacme"
	_ "github.com/opensvc/om3/v3/drivers/restaskhost"
	_ "github.com/opensvc/om3/v3/drivers/resvol"
	_ "github.com/opensvc/om3/v3/drivers/switchbrocade"
)

func init() {
	filepath.WalkDir(rawconfig.Paths.Drivers, func(path string, e os.DirEntry, err error) error {
		if e == nil {
			return nil
		}
		if e.IsDir() && path != rawconfig.Paths.Drivers {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, ".so") {
			return nil
		}
		if err := driver.LoadBundle(path); err != nil {
			fmt.Fprintf(os.Stderr, "loading bundle %s: %s\n", path, err)
		}
		return nil
	})
}
