module github.com/pingcap/tiup

go 1.16

require (
	github.com/AstroProfundis/sysinfo v0.0.0-20210201035811-eb96b87c86b3
	github.com/AstroProfundis/tabby v1.1.1-color
	github.com/BurntSushi/toml v0.3.1
	github.com/ScaleFT/sshkeys v0.0.0-20200327173127-6142f742bca5
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38
	github.com/appleboy/easyssh-proxy v1.3.9
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/blevesearch/bleve v1.0.14
	github.com/cavaliercoder/grab v1.0.1-0.20201108051000-98a5bfe305ec
	github.com/cheggaaa/pb/v3 v3.0.6
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/creasty/defaults v1.5.1
	github.com/docker/go-units v0.4.0
	github.com/fatih/color v1.10.0
	github.com/gibson042/canonicaljson-go v1.0.3
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gofrs/flock v0.8.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.3
	github.com/jeremywohl/flatten v1.0.1
	github.com/joomcode/errorx v1.0.3
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/magiconair/properties v1.8.4
	github.com/mattn/go-runewidth v0.0.10
	github.com/otiai10/copy v1.5.0
	github.com/pingcap/check v0.0.0-20200212061837-5e12011dc712
	github.com/pingcap/errors v0.11.5-0.20201126102027-b0a155152ca3
	github.com/pingcap/failpoint v0.0.0-20200702092429-9f69995143ce
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/go-tpc v1.0.5
	github.com/pingcap/go-ycsb v0.0.0-20210129115622-04d8656123e4
	github.com/pingcap/goleveldb v0.0.0-20191226122134-f82aafb29989 // indirect
	github.com/pingcap/kvproto v0.0.0-20210308063835-39b884695fb8
	github.com/pingcap/log v0.0.0-20201112100606-8f1e84a3abc8 // indirect
	github.com/pingcap/tidb-insight v0.3.2
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.3.0
	github.com/r3labs/diff/v2 v2.12.0
	github.com/relex/aini v1.2.1
	github.com/sergi/go-diff v1.1.0
	github.com/shirou/gopsutil v3.21.2+incompatible
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20190625010220-02440ea7a285 // indirect
	github.com/tidwall/pretty v1.0.2 // indirect
	github.com/tj/go-termd v0.0.2-0.20200115111609-7f6aeb166380
	github.com/tklauser/go-sysconf v0.3.4 // indirect
	github.com/unrolled/render v1.0.1 // indirect
	github.com/xo/usql v0.7.8
	go.etcd.io/etcd v0.5.0-alpha.5.0.20210226220824-aa7126864d82
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/mod v0.4.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210309074719-68d13333faf2
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	google.golang.org/genproto v0.0.0-20200108215221-bd8f9a0ef82f
	google.golang.org/grpc v1.26.0
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	software.sslmate.com/src/go-pkcs12 v0.0.0-20201103104416-57fc603b7f52
)

replace gopkg.in/yaml.v2 => github.com/july2993/yaml v0.0.0-20200423062752-adcfa5abe2ed
