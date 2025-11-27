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

import (
	"fmt"
	"strings"
)

var UsersetupStage = CommandsStage{
	SpinnerSuccessMessage: "New user created successfully",
	SpinnerFailMessage:    "Error creating a new user for the machine",
	Commands: []string{
		"sudo useradd -m -s /bin/bash -G sudo sidekick",
		`echo "sidekick ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers.d/sidekick`,
		"mkdir -p /home/sidekick/.ssh/",
		"sudo cat /root/.ssh/authorized_keys | sudo tee -a /home/sidekick/.ssh/authorized_keys",
		"sudo chown sidekick:sidekick /home/sidekick/.ssh/authorized_keys",
		"sudo chmod 600 /home/sidekick/.ssh/authorized_keys",
	},
}

var SetupStage = CommandsStage{
	SpinnerSuccessMessage: "VPS updated and setup successfully",
	SpinnerFailMessage:    "Error happened running basic setup commands",
	Commands: []string{
		fmt.Sprintf("echo '%s' > ./setup.sh", SetupStageScript),
		"chmod +x ./setup.sh",
		"sudo ./setup.sh",
		"rm ./setup.sh",
	},
}

var DockerStage = CommandsStage{
	SpinnerSuccessMessage: "Docker setup successfully",
	SpinnerFailMessage:    "Error happened during setting up docker",
	Commands: []string{
		"sudo apt-get update -y",
		"sudo install -m 0755 -d /etc/apt/keyrings",
		"sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc",
		"sudo chmod a+r /etc/apt/keyrings/docker.asc",
		`echo \
		"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
		$(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
		sudo tee /etc/apt/sources.list.d/docker.list > /dev/null`,
		"sudo apt-get update -y",
		"sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin -y",
		"sudo usermod -aG docker sidekick",
	},
}

func GetTraefikStage(email string) CommandsStage {
	return CommandsStage{
		SpinnerSuccessMessage: "Successfully setup Traefik",
		SpinnerFailMessage:    "Something went wrong setting up Traefik on your VPS",
		Commands: []string{
			"mkdir traefik",
			fmt.Sprintf("echo '%s' > ./traefik/docker-compose.yml", strings.Replace(TraefikDockerComposeFile, "$EMAIL", email, 1)),
			"mkdir -p ./traefik/ssl-certs/",
			"touch ./traefik/ssl-certs/acme.json",
			"chmod 600 ./traefik/ssl-certs/acme.json",
			"sudo docker network create sidekick",
			"cd traefik && sudo docker compose -p sidekick up -d",
		},
	}
}
