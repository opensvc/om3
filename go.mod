module github.com/opensvc/om3

go 1.21.0

toolchain go1.22.0

require (
	github.com/allenai/go-swaggerui v0.1.0
	github.com/andreazorzetto/yh v0.4.0
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/antchfx/xmlquery v1.3.10
	github.com/atomicgo/cursor v0.0.1
	github.com/containerd/cgroups v1.0.1
	github.com/containerd/cgroups/v3 v3.0.3
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.9.1
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cvaroqui/ini v1.66.7-0.20220627091046-b218d4fc5c30
	github.com/danwakefield/fnmatch v0.0.0-20160403171240-cbb64ac3d964
	github.com/deepmap/oapi-codegen v1.16.3
	github.com/devans10/pugo/pure1 v0.0.0-20230602184138-1a5d930c950e
	github.com/digitalocean/go-smbios v0.0.0-20180907143718-390a4f403a8e
	github.com/eiannone/keyboard v0.0.0-20200508000154-caf4b762e807
	github.com/fatih/color v1.16.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/g8rswimmer/error-chain v1.0.0
	github.com/gdamore/tcell/v2 v2.7.1
	github.com/getkin/kin-openapi v0.127.0
	github.com/go-chi/jwtauth/v5 v5.0.2
	github.com/go-ping/ping v0.0.0-20210506233800-ff8be3320020
	github.com/goccy/go-json v0.10.2
	github.com/golang-collections/collections v0.0.0-20130729185459-604e922904d3
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/golang/mock v1.5.0
	github.com/google/go-cmp v0.6.0
	github.com/google/nftables v0.0.0-20220129182606-a46119e5928d
	github.com/google/uuid v1.6.0
	github.com/goombaio/orderedset v0.0.0-20180925151225-8e67b20a9b77
	github.com/hashicorp/go-version v1.4.0
	github.com/hexops/gotextdiff v1.0.3
	github.com/iancoleman/orderedmap v0.2.0
	github.com/inancgumus/screen v0.0.0-20190314163918-06e984b86ed3
	github.com/j-keck/arping v0.0.0-20160618110441-2cf9dc699c56
	github.com/jaypipes/pcidb v0.6.0
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/labstack/echo-contrib v0.15.0
	github.com/labstack/echo/v4 v4.11.4
	github.com/labstack/gommon v0.4.2
	github.com/mattn/go-isatty v0.0.20
	github.com/mattn/go-runewidth v0.0.15
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mlafeldt/sysrq v0.0.0-20171106101645-38dd78d6e663
	github.com/msoap/byline v1.1.1
	github.com/ncw/directio v1.0.5
	github.com/oapi-codegen/runtime v1.1.1
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/opensvc/fcache v1.0.3
	github.com/opensvc/fcntllock v1.0.3
	github.com/opensvc/flock v1.0.3
	github.com/opensvc/testhelper v1.0.0
	github.com/pbar1/pkill-go v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.15.1
	github.com/prometheus/procfs v0.9.0
	github.com/retailnext/cannula v0.0.0-20160516234737-f1c21e7f5695
	github.com/rivo/tview v0.0.0-20240921122403-a64fc48d7654
	github.com/rs/zerolog v1.20.0
	github.com/shaj13/go-guardian/v2 v2.11.5
	github.com/shaj13/libcache v1.0.5
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/ssrathi/go-attr v1.3.0
	github.com/stretchr/testify v1.9.0
	github.com/subosito/gotenv v1.2.0
	github.com/talos-systems/go-smbios v0.1.1
	github.com/vishvananda/netlink v1.1.1-0.20211118161826-650dca95af54
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	github.com/ybbus/jsonrpc v2.1.2+incompatible
	github.com/yookoala/realpath v1.0.0
	github.com/zcalusic/sysinfo v0.0.0-20210831153053-2c6e1d254246
	golang.org/x/crypto v0.31.0
	golang.org/x/exp v0.0.0-20230725093048-515e97ebf090
	golang.org/x/net v0.33.0
	golang.org/x/sys v0.28.0
	golang.org/x/term v0.27.0
	golang.org/x/time v0.5.0
	gopkg.in/errgo.v2 v2.1.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	k8s.io/client-go v0.28.0
	sigs.k8s.io/yaml v1.3.0
	software.sslmate.com/src/go-pkcs12 v0.2.0
)

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/antchfx/xpath v1.2.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cilium/ebpf v0.11.0 // indirect
	github.com/coreos/go-iptables v0.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/go-chi/chi/v5 v5.1.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/goombaio/orderedmap v0.0.0-20180924084748-ba921b7e2419 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/invopop/yaml v0.3.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v0.0.0-20200817173448-b6b71def0850 // indirect
	github.com/koneu/natend v0.0.0-20150829182554-ec0926ea948d // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx v1.2.29 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/netlink v1.4.2 // indirect
	github.com/mdlayher/socket v0.0.0-20211102153432-57e3fa563ecb // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/opensvc/locker v1.0.3 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/safchain/ethtool v0.0.0-20200218184317-f459e2d13664 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.2.2 // indirect
)

replace github.com/spf13/viper => github.com/opensvc/viper v1.7.0-osvc.1
