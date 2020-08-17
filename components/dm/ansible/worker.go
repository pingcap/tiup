// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package ansible

// Config Copy from https://github.com/pingcap/dm/blob/21a6e6e580f2e911edbe2400241bd95de2f7ef43/dm/worker/config.go#L93
// remove some unconcern parts.
type Config struct {
	LogLevel  string `toml:"log-level" json:"log-level"`
	LogFile   string `toml:"log-file" json:"log-file"`
	LogFormat string `toml:"log-format" json:"log-format"`
	LogRotate string `toml:"log-rotate" json:"log-rotate"`

	WorkerAddr string `toml:"worker-addr" json:"worker-addr"`

	EnableGTID  bool   `toml:"enable-gtid" json:"enable-gtid"`
	AutoFixGTID bool   `toml:"auto-fix-gtid" json:"auto-fix-gtid"`
	RelayDir    string `toml:"relay-dir" json:"relay-dir"`
	MetaDir     string `toml:"meta-dir" json:"meta-dir"`
	ServerID    uint32 `toml:"server-id" json:"server-id"`
	Flavor      string `toml:"flavor" json:"flavor"`
	Charset     string `toml:"charset" json:"charset"`

	// relay synchronous starting point (if specified)
	RelayBinLogName string `toml:"relay-binlog-name" json:"relay-binlog-name"`
	RelayBinlogGTID string `toml:"relay-binlog-gtid" json:"relay-binlog-gtid"`

	SourceID string   `toml:"source-id" json:"source-id"`
	From     DBConfig `toml:"from" json:"from"`
}

// DBConfig of db.
type DBConfig struct {
	Host             string `toml:"host" json:"host" yaml:"host"`
	Port             int    `toml:"port" json:"port" yaml:"port"`
	User             string `toml:"user" json:"user" yaml:"user"`
	Password         string `toml:"password" json:"-" yaml:"password"` // omit it for privacy
	MaxAllowedPacket *int   `toml:"max-allowed-packet" json:"max-allowed-packet" yaml:"max-allowed-packet"`
}

// SourceConfig is the configuration for Worker
// ref: https://github.com/pingcap/dm/blob/3730a4e231091c5d65130d15a6c09a3b9fa3255e/dm/config/source_config.go#L51
type SourceConfig struct {
	EnableGTID  bool   `yaml:"enable-gtid" toml:"enable-gtid" json:"enable-gtid"`
	AutoFixGTID bool   `yaml:"auto-fix-gtid" toml:"auto-fix-gtid" json:"auto-fix-gtid"`
	RelayDir    string `yaml:"relay-dir" toml:"relay-dir" json:"relay-dir"`
	MetaDir     string `yaml:"meta-dir" toml:"meta-dir" json:"meta-dir"`
	Flavor      string `yaml:"flavor" toml:"flavor" json:"flavor"`
	Charset     string `yaml:"charset" toml:"charset" json:"charset"`

	EnableRelay bool `yaml:"enable-relay" toml:"enable-relay" json:"enable-relay"`
	// relay synchronous starting point (if specified)
	RelayBinLogName string `yaml:"relay-binlog-name" toml:"relay-binlog-name" json:"relay-binlog-name"`
	RelayBinlogGTID string `yaml:"relay-binlog-gtid" toml:"relay-binlog-gtid" json:"relay-binlog-gtid"`

	SourceID string   `yaml:"source-id" toml:"source-id" json:"source-id"`
	From     DBConfig `yaml:"from" toml:"from" json:"from"`
}

// ToSource generate the SourceConfig for DM 2.0
func (c *Config) ToSource() (source *SourceConfig) {
	source = &SourceConfig{
		EnableGTID:  c.EnableGTID,
		AutoFixGTID: c.AutoFixGTID,
		RelayDir:    c.RelayDir,
		MetaDir:     c.MetaDir,
		Flavor:      c.Flavor,
		// EnableRelay:
		RelayBinLogName: c.RelayBinLogName,
		RelayBinlogGTID: c.RelayBinlogGTID,
		SourceID:        c.SourceID,
		From:            c.From,
	}

	return
}
