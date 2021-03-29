module opensvc.com/opensvc

go 1.13

require (
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.9.1
	github.com/danwakefield/fnmatch v0.0.0-20160403171240-cbb64ac3d964
	github.com/fatih/color v1.10.0
	github.com/gofrs/flock v0.8.0
	github.com/golang-collections/collections v0.0.0-20130729185459-604e922904d3
	github.com/golang/mock v1.5.0
	github.com/google/uuid v1.2.0
	github.com/guregu/null v4.0.0+incompatible
	github.com/inancgumus/screen v0.0.0-20190314163918-06e984b86ed3
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a
	github.com/kopoli/go-terminal-size v0.0.0-20170219200355-5c97524c8b54
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mattn/go-isatty v0.0.12
	github.com/mitchellh/go-homedir v1.1.0
	github.com/msoap/byline v1.1.1
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.20.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.3.0
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

replace github.com/spf13/viper => github.com/opensvc/viper v1.7.0-osvc.1
