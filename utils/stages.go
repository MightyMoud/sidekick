package utils

import "fmt"

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
		"sudo sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config && sudo systemctl restart ssh",
		"sudo apt-get update -y",
		"sudo apt-get upgrade -y",
		"sudo apt-get install age -y",
		"sudo apt-get install ca-certificates curl vim -y",
		"curl -LO https://github.com/getsops/sops/releases/download/v3.9.0/sops-v3.9.0.linux.amd64",
		"sudo mv sops-v3.9.0.linux.amd64 /usr/local/bin/sops",
		"sudo chmod +x /usr/local/bin/sops",
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

func GetTraefikStage(server string) CommandsStage {
	return CommandsStage{
		SpinnerSuccessMessage: "Successfully setup Traefik",
		SpinnerFailMessage:    "Something went wrong setting up Traefik on your VPS",
		Commands: []string{
			"sudo apt-get install git -y",
			"git clone https://github.com/ms-mousa/sidekick-traefik.git",
			fmt.Sprintf(`cd sidekick-traefik && sed -i.bak "s/\$HOST/%s/g; s/\$PORT/%s/g" docker-compose.traefik.yml && rm docker-compose.traefik.yml.bak`, server, "8000"),
			"sudo docker network create sidekick",
			"cd sidekick-traefik && sudo docker compose -p sidekick -f docker-compose.traefik.yml up -d",
		},
	}
}
