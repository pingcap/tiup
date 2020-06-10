package spec

// TiSparkMasterSpec is the topology specification for TiSpark master node
type TiSparkMasterSpec struct {
	Host      string `yaml:"host"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	Imported  bool   `yaml:"imported,omitempty"`
	Port      int    `yaml:"port" default:"8300"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkMasterSpec) Role() string {
	return ComponentTiSparkMaster
}

// SSH returns the host and SSH port of the instance
func (s TiSparkMasterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkMasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkMasterSpec) IsImported() bool {
	return s.Imported
}

// TiSparkSlaveSpec is the topology specification for TiSpark slave nodes
type TiSparkSlaveSpec struct {
	Host      string `yaml:"host"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	Imported  bool   `yaml:"imported,omitempty"`
	Port      int    `yaml:"port" default:"8300"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkSlaveSpec) Role() string {
	return ComponentTiSparkSlave
}

// SSH returns the host and SSH port of the instance
func (s TiSparkSlaveSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkSlaveSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkSlaveSpec) IsImported() bool {
	return s.Imported
}
