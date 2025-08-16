/*
Copyright © 2024 Mahmoud Mousa <m.mousa@hey.com>

Licensed under the GNU GPL License, Version 3.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.gnu.org/licenses/gpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

type DependsOn struct {
	Condition string `yaml:"condition"`
}

type Healthcheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}

type DockerBuildContext struct {
	Context    string                 `yaml:"context,omitempty"`
	Dockerfile string                 `yaml:"dockerfile,omitempty"`
	Args       map[string]interface{} `yaml:"args,omitempty"`
}

type DockerService struct {
	Image       string                 `yaml:"image,omitempty"`
	Build       *DockerBuildContext    `yaml:"build,omitempty"`
	Command     string                 `yaml:"command,omitempty"`
	Restart     string                 `yaml:"restart,omitempty"`
	Ports       []string               `yaml:"ports,omitempty"`
	Volumes     []string               `yaml:"volumes,omitempty"`
	Labels      []string               `yaml:"labels,omitempty"`
	Networks    []string               `yaml:"networks,omitempty"`
	Environment interface{}            `yaml:"environment,omitempty"` // Can be []string or map[string]string
	EnvFile     interface{}            `yaml:"env_file,omitempty"`     // Can be string or []string
	DependsOn   map[string]DependsOn   `yaml:"depends_on,omitempty"`
	HealthCheck Healthcheck            `yaml:"healthcheck,omitempty"`
	EntryPoint  []string               `yaml:"entrypoint,omitempty"`
}

type DockerNetwork struct {
	External bool `yaml:"external"`
}

type DockerComposeFile struct {
	Services map[string]DockerService `yaml:"services"`
	Networks map[string]DockerNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]DockerVolume  `yaml:"volumes,omitempty"`
}

type DockerVolume struct {
	Driver string `yaml:"driver,omitempty"`
}
type SidekickAppEnvConfig struct {
	File string `yaml:"file"`
	Hash string `yaml:"hash"`
}

type SidekickPreview struct {
	Url       string `yaml:"url"`
	Image     string `yaml:"image"`
	CreatedAt string `yaml:"createdAt"`
}

type SidekickAppDatabaseBackupConfig struct {
	Target       string `yaml:"target"`
	BucketName   string `yaml:"bucketName"`
	BucketPath   string `yaml:"path"`
	BucketRegion string `yaml:"region"`
	S3Endpoint   string `yaml:"s3Endpoint"`
}

type SidekickAppDatabaseConfig struct {
	Type   string                          `yaml:"type"`
	DbName string                          `yaml:"databaseName"`
	Backup SidekickAppDatabaseBackupConfig `yaml:"backup,omitempty"`
}

type SidekickAppConfig struct {
	Name           string                     `yaml:"name"`
	Version        string                     `yaml:"version"`
	Image          string                     `yaml:"image,omitempty"`
	Url            string                     `yaml:"url"`
	Port           uint64                     `yaml:"port"`
	CreatedAt      string                     `yaml:"createdAt"`
	DeploymentType string                     `yaml:"deploymentType,omitempty"` // "dockerfile" or "compose"
	ComposeFile    string                     `yaml:"composeFile,omitempty"`
	MainService    string                     `yaml:"mainService,omitempty"`
	Env            SidekickAppEnvConfig       `yaml:"env,omitempty"`
	DatabaseConfig SidekickAppDatabaseConfig  `yaml:"database,omitempty"`
	PreviewEnvs    map[string]SidekickPreview `yaml:"previewEnvs,omitempty"`
}
