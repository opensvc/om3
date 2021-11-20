module opensvc.com/opensvc

go 1.13

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/atomicgo/cursor v0.0.1
	github.com/containerd/cgroups v1.0.1
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.9.1
	github.com/cpuguy83/go-docker v0.0.0-20201116220134-debea1262389
	github.com/danwakefield/fnmatch v0.0.0-20160403171240-cbb64ac3d964
	github.com/eiannone/keyboard v0.0.0-20200508000154-caf4b762e807
	github.com/eidolon/wordwrap v0.0.0-20161011182207-e0f54129b8bb
	github.com/fatih/color v1.10.0
	github.com/go-ping/ping v0.0.0-20210506233800-ff8be3320020
	github.com/gofrs/flock v0.8.1
	github.com/golang-collections/collections v0.0.0-20130729185459-604e922904d3
	github.com/golang/mock v1.5.0
	github.com/google/uuid v1.2.0
	github.com/guregu/null v4.0.0+incompatible
	github.com/hexops/gotextdiff v1.0.3
	github.com/iancoleman/orderedmap v0.2.0
	github.com/inancgumus/screen v0.0.0-20190314163918-06e984b86ed3
	github.com/j-keck/arping v0.0.0-20160618110441-2cf9dc699c56
	github.com/jaypipes/pcidb v0.6.0
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mattn/go-isatty v0.0.12
	github.com/mitchellh/go-homedir v1.1.0
	github.com/msoap/byline v1.1.1
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/opensvc/fcache v1.0.3
	github.com/opensvc/fcntllock v1.0.2
	github.com/opensvc/flock v1.0.3
	github.com/opensvc/testhelper v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.20.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/ssrathi/go-attr v1.3.0
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	github.com/yookoala/realpath v1.0.0
	github.com/zcalusic/sysinfo v0.0.0-20210831153053-2c6e1d254246
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1
	gopkg.in/errgo.v2 v2.1.0
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

replace github.com/spf13/viper => github.com/opensvc/viper v1.7.0-osvc.1

replace github.com/cpuguy83/go-docker => github.com/opensvc/go-docker v0.0.0-20211017135555-65a1ec774c95
