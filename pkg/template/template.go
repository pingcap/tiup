package template

type ConfigGenerator interface {
	Config() ([]byte, error)
	ConfigWithTemplate(tpl string) ([]byte, error)
	ConfigToFile(file string) error
}
