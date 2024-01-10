module github.com/opensvc/om3

go 1.20

require (
	github.com/allenai/go-swaggerui v0.1.0
	github.com/andreazorzetto/yh v0.4.0
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/antchfx/xmlquery v1.3.10
	github.com/atomicgo/cursor v0.0.1
	github.com/containerd/cgroups v1.0.1
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.9.1
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cpuguy83/go-docker v0.0.0-20230118175646-6070475a5194
	github.com/cvaroqui/ini v1.66.7-0.20220627091046-b218d4fc5c30
	github.com/danwakefield/fnmatch v0.0.0-20160403171240-cbb64ac3d964
	github.com/deepmap/oapi-codegen v1.16.2
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/digitalocean/go-smbios v0.0.0-20180907143718-390a4f403a8e
	github.com/eiannone/keyboard v0.0.0-20200508000154-caf4b762e807
	github.com/fatih/color v1.15.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/g8rswimmer/error-chain v1.0.0
	github.com/gdamore/tcell/v2 v2.3.1
	github.com/getkin/kin-openapi v0.122.0
	github.com/go-chi/jwtauth/v5 v5.0.2
	github.com/go-ping/ping v0.0.0-20210506233800-ff8be3320020
	github.com/goccy/go-json v0.10.2
	github.com/golang-collections/collections v0.0.0-20130729185459-604e922904d3
	github.com/golang/mock v1.5.0
	github.com/google/go-cmp v0.5.9
	github.com/google/nftables v0.0.0-20220129182606-a46119e5928d
	github.com/google/uuid v1.5.0
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
	github.com/prometheus/client_golang v1.15.1
	github.com/prometheus/procfs v0.9.0
	github.com/retailnext/cannula v0.0.0-20160516234737-f1c21e7f5695
	github.com/rs/zerolog v1.20.0
	github.com/shaj13/go-guardian/v2 v2.11.5
	github.com/shaj13/libcache v1.0.5
	github.com/soellman/pidfile v0.0.0-20160225184504-d482c905736b
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/ssrathi/go-attr v1.3.0
	github.com/stretchr/testify v1.8.4
	github.com/talos-systems/go-smbios v0.1.1
	github.com/vishvananda/netlink v1.1.1-0.20211118161826-650dca95af54
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	github.com/ybbus/jsonrpc v2.1.2+incompatible
	github.com/yookoala/realpath v1.0.0
	github.com/zcalusic/sysinfo v0.0.0-20210831153053-2c6e1d254246
	golang.org/x/crypto v0.17.0
	golang.org/x/net v0.19.0
	golang.org/x/sys v0.16.0
	golang.org/x/term v0.15.0
	golang.org/x/time v0.5.0
	gopkg.in/errgo.v2 v2.1.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	k8s.io/client-go v0.28.0
	sigs.k8s.io/yaml v1.3.0
	software.sslmate.com/src/go-pkcs12 v0.2.0
)

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/CloudyKit/fastprinter v0.0.0-20200109182630-33d98a066a53 // indirect
	github.com/CloudyKit/jet/v6 v6.2.0 // indirect
	github.com/Joker/jade v1.1.3 // indirect
	github.com/Microsoft/go-winio v0.4.15 // indirect
	github.com/Shopify/goreferrer v0.0.0-20220729165902-8cddb4f5de06 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/antchfx/xpath v1.2.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic v1.10.0-rc3 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20230717121745-296ad89f973d // indirect
	github.com/chenzhuoyu/iasm v0.9.0 // indirect
	github.com/cilium/ebpf v0.7.0 // indirect
	github.com/coreos/go-iptables v0.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/flosch/pongo2/v4 v4.0.2 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.9.1 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.7 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.1 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomarkdown/markdown v0.0.0-20230922112808-5421fefb8386 // indirect
	github.com/goombaio/orderedmap v0.0.0-20180924084748-ba921b7e2419 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/invopop/yaml v0.2.0 // indirect
	github.com/iris-contrib/schema v0.0.6 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v0.0.0-20200817173448-b6b71def0850 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kataras/blocks v0.0.7 // indirect
	github.com/kataras/golog v0.1.9 // indirect
	github.com/kataras/iris/v12 v12.2.6-0.20230908161203-24ba4e8933b9 // indirect
	github.com/kataras/pio v0.0.12 // indirect
	github.com/kataras/sitemap v0.0.6 // indirect
	github.com/kataras/tunnel v0.0.4 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/koneu/natend v0.0.0-20150829182554-ec0926ea948d // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.1 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx v1.2.25 // indirect
	github.com/lestrrat-go/option v1.0.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mailgun/raymond/v2 v2.0.48 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/netlink v1.4.2 // indirect
	github.com/mdlayher/socket v0.0.0-20211102153432-57e3fa563ecb // indirect
	github.com/microcosm-cc/bluemonday v1.0.25 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/opensvc/locker v1.0.3 // indirect
	github.com/pelletier/go-toml v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/safchain/ethtool v0.0.0-20200218184317-f459e2d13664 // indirect
	github.com/schollz/closestmatch v2.1.0+incompatible // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/afero v1.1.2 // indirect
	github.com/spf13/cast v1.3.0 // indirect
	github.com/spf13/jwalterweatherman v1.0.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tdewolff/minify/v2 v2.12.9 // indirect
	github.com/tdewolff/parse/v2 v2.6.8 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yosssi/ace v0.0.5 // indirect
	golang.org/x/arch v0.4.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.16.1 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.2.2 // indirect
)

replace github.com/spf13/viper => github.com/opensvc/viper v1.7.0-osvc.1
