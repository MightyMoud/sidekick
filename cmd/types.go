package cmd

type DockerService struct {
	Image       string   `yaml:"image"`
	Command     string   `yaml:"command,omitempty"`
	Ports       []string `yaml:"ports,omitempty"`
	Volumes     []string `yaml:"volumes,omitempty"`
	Labels      []string `yaml:"labels,omitempty"`
	Networks    []string `yaml:"networks,omitempty"`
	Environment []string `yaml:"environment,omitempty"`
}

type DockerNetwork struct {
	External bool `yaml:"external"`
}

type DockerComposeFile struct {
	Services map[string]DockerService `yaml:"services"`
	Networks map[string]DockerNetwork `yaml:"networks"`
}

type SidekickAppConfig struct {
	Image          string `yaml:"image"`
	Url            string `yaml:"url"`
	CreatedAt      string `yaml:"createdAt"`
	LastDeployedAt string `yaml:"lastDeployedAt"`
}
