module github.com/pingcap/tiup

go 1.16

replace (
	github.com/appleboy/easyssh-proxy => github.com/AstroProfundis/easyssh-proxy v1.3.10-0.20210615044136-d52fc631316d
	gopkg.in/yaml.v2 => github.com/july2993/yaml v0.0.0-20200423062752-adcfa5abe2ed
)

require (
	github.com/AstroProfundis/sysinfo v0.0.0-20210901042104-765f00aa1304
	github.com/AstroProfundis/tabby v1.1.1-color
	github.com/BurntSushi/toml v0.4.1
	github.com/ScaleFT/sshkeys v0.0.0-20200327173127-6142f742bca5
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38
	github.com/appleboy/easyssh-proxy v1.3.9
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/blevesearch/bleve v1.0.14
	github.com/cavaliercoder/grab v1.0.1-0.20201108051000-98a5bfe305ec
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/creasty/defaults v1.5.2
	github.com/docker/go-units v0.4.0
	github.com/fatih/color v1.12.0
	github.com/gibson042/canonicaljson-go v1.0.3
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gofrs/flock v0.8.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/jeremywohl/flatten v1.0.1
	github.com/joomcode/errorx v1.0.3
	github.com/juju/ansiterm v0.0.0-20210706145210-9283cdf370b5
	github.com/kr/text v0.2.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mattn/go-runewidth v0.0.13
	github.com/otiai10/copy v1.6.0
	github.com/pingcap/check v0.0.0-20200212061837-5e12011dc712
	github.com/pingcap/errors v0.11.4
	github.com/pingcap/failpoint v0.0.0-20210806123219-9af598712e22
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/kvproto v0.0.0-20210830034942-555d7b3265ae
	github.com/pingcap/tidb-insight/collector v0.0.0-20210901075740-c60ea2cc41e4
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.30.0
	github.com/prometheus/prom2json v1.3.0
	github.com/r3labs/diff/v2 v2.13.6
	github.com/relex/aini v1.5.0
	github.com/sergi/go-diff v1.2.0
	github.com/shirou/gopsutil v3.21.7+incompatible
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tj/go-termd v0.0.1
	go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	go.uber.org/atomic v1.9.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/mod v0.5.0
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210831042530-f4d43177bf5e
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
	google.golang.org/genproto v0.0.0-20210831024726-fe130286e0e2
	google.golang.org/grpc v1.40.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	software.sslmate.com/src/go-pkcs12 v0.0.0-20210415151418-c5206de65a78
)
