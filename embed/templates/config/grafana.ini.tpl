##################### Grafana Configuration Example #####################
#
# Everything has defaults so you only need to uncomment things you want to
# change

# possible values : production, development
; app_mode = production

# instance name, defaults to HOSTNAME environment variable value or hostname if HOSTNAME var is empty
; instance_name = ${HOSTNAME}

#################################### Paths ####################################
[paths]
# Path to where grafana can store temp files, sessions, and the sqlite3 db (if that is used)
#
data = {{.DeployDir}}/data
#
# Directory where grafana can store logs
#
logs = {{.DeployDir}}/log
#
# Directory where grafana will automatically scan and look for plugins
#
plugins = {{.DeployDir}}/plugins
#
# folder that contains provisioning config files that grafana will apply on startup and while running.
provisioning = {{.DeployDir}}/provisioning

#
#################################### Server ####################################
[server]
# Protocol (http or https)
;protocol = http

# The ip address to bind to, empty will bind to all interfaces
;http_addr =

# The http port  to use
http_port = {{.Port}}

# The public facing domain name used to access grafana from a browser
{{- if .Domain}}
domain = {{.Domain}}
{{- else}}
domain = {{.IP}}
{{- end}}

# Redirect to correct domain if host header does not match domain
# Prevents DNS rebinding attacks
;enforce_domain = false

# The full public facing url
{{- if .RootURL}}
root_url = {{.RootURL}}
server_from_sub_path = true
{{- end}}

# Log web requests
;router_logging = false

# the path relative working path
;static_root_path = public

# enable gzip
;enable_gzip = false

# https certs & key file
;cert_file =
;cert_key =

#################################### Database ####################################
[database]
# Either "mysql", "postgres" or "sqlite3", it's your choice
;type = sqlite3
;host = 127.0.0.1:3306
;name = grafana
;user = root
;password =

# For "postgres" only, either "disable", "require" or "verify-full"
;ssl_mode = disable

# For "sqlite3" only, path relative to data_path setting
;path = grafana.db

#################################### Session ####################################
[session]
# Either "memory", "file", "redis", "mysql", "postgres", default is "file"
;provider = file

# Provider config options
# memory: not have any config yet
# file: session dir path, is relative to grafana data_path
# redis: config like redis server e.g. `addr=127.0.0.1:6379,pool_size=100,db=grafana`
# mysql: go-sql-driver/mysql dsn config string, e.g. `user:password@tcp(127.0.0.1:3306)/database_name`
# postgres: user=a password=b host=localhost port=5432 dbname=c sslmode=disable
;provider_config = sessions

# Session cookie name
;cookie_name = grafana_sess

# If you use session in https only, default is false
;cookie_secure = false

# Session life time, default is 86400
;session_life_time = 86400

#################################### Analytics ####################################
[analytics]
# Server reporting, sends usage counters to stats.grafana.org every 24 hours.
# No ip addresses are being tracked, only simple counters to track
# running instances, dashboard and error counts. It is very helpful to us.
# Change this option to false to disable reporting.
;reporting_enabled = true

# Set to false to disable all checks to https://grafana.net
# for new versions (grafana itself and plugins), check is used
# in some UI views to notify that grafana or plugin update exists
# This option does not cause any auto updates, nor send any information
# only a GET request to http://grafana.net to get latest versions
check_for_updates = true

# Google Analytics universal tracking code, only enabled if you specify an id here
;google_analytics_ua_id =

#################################### Security ####################################
[security]
# default admin user, created on startup
admin_user = {{.Username}}

# default admin password, can be changed before first start of grafana,  or in profile settings
admin_password = `{{.Password}}`

# used for signing
;secret_key = SW2YcwTIb9zpOOhoPsMm

# Auto-login remember days
;login_remember_days = 7
;cookie_username = grafana_user
;cookie_remember_name = grafana_remember

# disable gravatar profile images
;disable_gravatar = false

# data source proxy whitelist (ip_or_domain:port separated by spaces)
;data_source_proxy_whitelist =

[snapshots]
# snapshot sharing options
;external_enabled = true
;external_snapshot_url = https://snapshots-origin.raintank.io
;external_snapshot_name = Publish to snapshot.raintank.io

#################################### Users ####################################
[users]
# disable user signup / registration
;allow_sign_up = true

# Allow non admin users to create organizations
;allow_org_create = true

# Set to true to automatically assign new users to the default organization (id 1)
;auto_assign_org = true

# Default role new users will be automatically assigned (if disabled above is set to true)
;auto_assign_org_role = Viewer

# Background text for the user field on the login page
;login_hint = email or username

# Default UI theme ("dark" or "light")
{{- if .DefaultTheme}}
default_theme = {{.DefaultTheme}}
{{- else}}
;default_theme = dark
{{- end}}

############### Set Cookie Name for Multiple Instances #######################
[auth]
login_cookie_name = grafana_session_{{ if .Domain }}{{.Domain}}_{{ end }}{{.Port}}

#################################### Anonymous Auth ##########################
[auth.anonymous]
{{- if .AnonymousEnable}}
enabled = true
{{- end}}

# specify organization name that should be used for unauthenticated users
{{- if .OrgName}}
org_name = {{.OrgName}}
{{- else}}
;org_name = Main Org.
{{- end}}

# specify role for unauthenticated users
{{- if .OrgRole}}
org_role = {{.OrgRole}}
{{- else}}
;org_role = Viewer
{{- end}}

#################################### Basic Auth ##########################
[auth.basic]
;enabled = true

#################################### Auth LDAP ##########################
[auth.ldap]
;enabled = false
;config_file = /etc/grafana/ldap.toml

#################################### SMTP / Emailing ##########################
[smtp]
;enabled = false
;host = localhost:25
;user =
;password =
;cert_file =
;key_file =
;skip_verify = false
;from_address = admin@grafana.localhost

[emails]
;welcome_email_on_sign_up = false

#################################### Logging ##########################
[log]
# Either "console", "file", "syslog". Default is console and  file
# Use space to separate multiple modes, e.g. "console file"
mode = file

# Either "trace", "debug", "info", "warn", "error", "critical", default is "info"
;level = info

# For "console" mode only
[log.console]
;level =

# log line format, valid options are text, console and json
;format = console

# For "file" mode only
[log.file]
level = info

# log line format, valid options are text, console and json
format = text

# This enables automated log rotate(switch of following options), default is true
;log_rotate = true

# Max line number of single file, default is 1000000
;max_lines = 1000000

# Max size shift of single file, default is 28 means 1 << 28, 256MB
;max_size_shift = 28

# Segment log daily, default is true
;daily_rotate = true

# Expired days of log file(delete after max days), default is 7
;max_days = 7

[log.syslog]
;level =

# log line format, valid options are text, console and json
;format = text

# Syslog network type and address. This can be udp, tcp, or unix. If left blank, the default unix endpoints will be used.
;network =
;address =

# Syslog facility. user, daemon and local0 through local7 are valid.
;facility =

# Syslog tag. By default, the process' argv[0] is used.
;tag =


#################################### AMQP Event Publisher ##########################
[event_publisher]
;enabled = false
;rabbitmq_url = amqp://localhost/
;exchange = grafana_events

;#################################### Dashboard JSON files ##########################
[dashboards.json]
enabled = false
path = {{.DeployDir}}/dashboards

#################################### Internal Grafana Metrics ##########################
# Metrics available at HTTP API Url /api/metrics
[metrics]
# Disable / Enable internal metrics
;enabled           = true

# Publish interval
;interval_seconds  = 10

# Send internal metrics to Graphite
; [metrics.graphite]
; address = localhost:2003
; prefix = prod.grafana.%(instance_name)s.

#################################### Internal Grafana Metrics ##########################
# Url used to to import dashboards directly from Grafana.net
[grafana_net]
url = https://grafana.net
