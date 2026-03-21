package templates

// Input defines interactive user fields typically consumed via CLI dialog.
type Input struct {
	Description string `yaml:"description"`
	Default     any    `yaml:"default,omitempty"`
	Value       any
}

// ProjectTemplate delineates the configuration manifest defining how a template generates.
type ProjectTemplate struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Renderer    string           `yaml:"renderer,omitempty"`
	Inputs      map[string]Input `yaml:"inputs"`
	Env         map[string]any   `yaml:"env"`
}
