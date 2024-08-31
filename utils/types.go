package utils

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

type SidekickAppEnvConfig struct {
	File string `yaml:"file"`
	Hash string `yaml:"hash"`
}

type SidekickPreview struct {
	Name  string `yaml:"name"`
	Url   string `yaml:"url"`
	Image string `yaml:"image"`
}

type SidekickAppConfig struct {
	Name        string                     `yaml:"name"`
	Version     string                     `yaml:"version"`
	Image       string                     `yaml:"image"`
	Url         string                     `yaml:"url"`
	Port        uint64                     `yaml:"port"`
	CreatedAt   string                     `yaml:"createdAt"`
	Env         SidekickAppEnvConfig       `yaml:"env,omitempty"`
	PreviewEnvs map[string]SidekickPreview `yaml:"previewEnvs"`
}
