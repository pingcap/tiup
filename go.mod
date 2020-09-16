module github.com/pingcap/tiup

go 1.13

require (
	github.com/AstroProfundis/sysinfo v0.0.0-20200423033635-f6f7687215fd
	github.com/AstroProfundis/tabby v1.1.0-color
	github.com/BurntSushi/toml v0.3.1
	github.com/ScaleFT/sshkeys v0.0.0-20181112160850-82451a803681
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38
	github.com/appleboy/easyssh-proxy v1.3.2
	github.com/asaskevich/EventBus v0.0.0-20180315140547-d46933a94f05
	github.com/blevesearch/bleve v1.0.8-0.20200520165604-f0ded112bb1b
	github.com/cavaliercoder/grab v2.0.1-0.20200331080741-9f014744ee41+incompatible
	github.com/cheggaaa/pb v2.0.7+incompatible
	github.com/cheynewallace/tabby v1.1.0
	github.com/creasty/defaults v1.3.0
	github.com/cznic/b v0.0.0-20181122101859-a26611c4d92d // indirect
	github.com/cznic/strutil v0.0.0-20181122101858-275e90344537 // indirect
	github.com/facebookgo/ensure v0.0.0-20200202191622-63f1cf65ac4c // indirect
	github.com/facebookgo/subset v0.0.0-20200203212716-c811ad88dec4 // indirect
	github.com/fatih/color v1.9.0
	github.com/gibson042/canonicaljson-go v1.0.3
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.3.4
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.4
	github.com/jeremywohl/flatten v1.0.1
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/joomcode/errorx v1.0.1
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a // indirect
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/magiconair/properties v1.8.0
	github.com/markbates/pkger v0.16.0
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-runewidth v0.0.7
	github.com/otiai10/copy v1.2.0
	github.com/pingcap/check v0.0.0-20200212061837-5e12011dc712
	github.com/pingcap/dm v1.1.0-alpha.0.20200521025928-83063141c5fd
	github.com/pingcap/errors v0.11.5-0.20200820035142-66eb5bf1d1cd
	github.com/pingcap/failpoint v0.0.0-20200702092429-9f69995143ce
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/go-tpc v1.0.4-0.20200525052430-dc963cdeef62
	github.com/pingcap/go-ycsb v0.0.0-20200226103513-00ca633a87d8
	github.com/pingcap/kvproto v0.0.0-20200810113304-6157337686b1
	github.com/pingcap/tidb-insight v0.3.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.3.0
	github.com/r3labs/diff v0.0.0-20200627101315-aecd9dd05dd2
	github.com/relex/aini v1.2.0
	github.com/sergi/go-diff v1.0.1-0.20180205163309-da645544ed44
	github.com/shirou/gopsutil v2.20.3+incompatible
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/tikv/pd v1.1.0-beta.0.20200824114021-f8c45ae287fd
	github.com/tj/assert v0.0.0-20190920132354-ee03d75cd160
	github.com/tj/go-termd v0.0.1
	github.com/xo/usql v0.7.8
	go.etcd.io/etcd v0.5.0-alpha.5.0.20191023171146-3cf2f69b5738
	go.uber.org/atomic v1.6.0
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200204104054-c9f3fb736b72
	golang.org/x/mod v0.2.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f
	gopkg.in/VividCortex/ewma.v1 v1.1.1 // indirect
	gopkg.in/cheggaaa/pb.v2 v2.0.7 // indirect
	gopkg.in/fatih/color.v1 v1.7.0 // indirect
	gopkg.in/ini.v1 v1.55.0
	gopkg.in/mattn/go-colorable.v0 v0.1.0 // indirect
	gopkg.in/mattn/go-isatty.v0 v0.0.4 // indirect
	gopkg.in/mattn/go-runewidth.v0 v0.0.4 // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	gopkg.in/yaml.v2 v2.2.8
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.0.0-20200619203921-c9ed90bd32dc
)

replace gopkg.in/yaml.v2 => github.com/july2993/yaml v0.0.0-20200423062752-adcfa5abe2ed
