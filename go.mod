module github.com/pingcap/tiup

go 1.16

require (
	github.com/AstroProfundis/sysinfo v0.0.0-20210201035811-eb96b87c86b3
	github.com/AstroProfundis/tabby v1.1.1-color
	github.com/BurntSushi/toml v0.3.1
	github.com/ScaleFT/sshkeys v0.0.0-20200327173127-6142f742bca5
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38
	github.com/alecthomas/chroma v0.9.1 // indirect
	github.com/alecthomas/repr v0.0.0-20181024024818-d37bc2a10ba1 // indirect
	github.com/appleboy/easyssh-proxy v1.3.9
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/blevesearch/bleve v1.0.14
	github.com/cavaliercoder/grab v1.0.1-0.20201108051000-98a5bfe305ec
	github.com/cheggaaa/pb/v3 v3.0.6
	github.com/creasty/defaults v1.5.1
	github.com/docker/go-units v0.4.0
	github.com/fatih/color v1.10.0
	github.com/gibson042/canonicaljson-go v1.0.3
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gofrs/flock v0.8.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.14.6
	github.com/jeremywohl/flatten v1.0.1
	github.com/joomcode/errorx v1.0.3
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a
	github.com/kr/text v0.2.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mattn/go-runewidth v0.0.12
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/otiai10/copy v1.5.0
	github.com/pingcap/check v0.0.0-20200212061837-5e12011dc712
	github.com/pingcap/errors v0.11.5-0.20201126102027-b0a155152ca3
	github.com/pingcap/failpoint v0.0.0-20200702092429-9f69995143ce
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/kvproto v0.0.0-20210308063835-39b884695fb8
	github.com/pingcap/log v0.0.0-20201112100606-8f1e84a3abc8 // indirect
	github.com/pingcap/tidb-insight v0.3.2
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.24.0
	github.com/prometheus/prom2json v1.3.0
	github.com/r3labs/diff/v2 v2.12.0
	github.com/relex/aini v1.2.1
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.1.0
	github.com/shirou/gopsutil v3.21.2+incompatible
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20190625010220-02440ea7a285 // indirect
	github.com/tj/go-termd v0.0.2-0.20200115111609-7f6aeb166380
	github.com/tklauser/go-sysconf v0.3.4 // indirect
	github.com/willf/bitset v1.1.11 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.0.0-20210512114948-1929aa0a36ec
	go.etcd.io/etcd/client/v3 v3.5.0-alpha.0
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.16.1-0.20210329175301-c23abee72d19
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf
	golang.org/x/mod v0.4.1
	golang.org/x/net v0.0.0-20210505214959-0714010a04ed // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210507161434-a76c4d0a0096
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210506142907-4a47615972c2
	google.golang.org/grpc v1.37.0
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	software.sslmate.com/src/go-pkcs12 v0.0.0-20201103104416-57fc603b7f52
)

replace gopkg.in/yaml.v2 => github.com/july2993/yaml v0.0.0-20200423062752-adcfa5abe2ed
