package template

// ConfigGenerator is used to generate configuration for component
type ConfigGenerator interface {
	Config() ([]byte, error)
	ConfigWithTemplate(tpl string) ([]byte, error)
	ConfigToFile(file string) error
}
