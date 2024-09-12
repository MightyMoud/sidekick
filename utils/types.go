/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

Licensed under the GNU AGPL License, Version 3.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.gnu.org/licenses/agpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
