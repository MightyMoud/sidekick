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

var sshKeyScript = `
		publicKey=$1

		echo "$publicKey" | ssh-keygen -lvf /dev/stdin 
	`

var EnvEncryptionScript = `
	PUBKEY=$1
	ENVFILE=$2

	sops encrypt --output-type dotenv --age $PUBKEY $ENVFILE > encrypted.env
	`

var DeployAppWithEnvScript = `
	export SOPS_AGE_KEY=$age_secret_key && \
	cd $service_name && \
	old_container_id=$(docker ps -f name=$service_name -q | tail -n1) && \
	sops exec-env encrypted.env 'docker compose -p sidekick up -d --no-deps --scale $service_name=2 --no-recreate $service_name' && \
	new_container_id=$(docker ps -f name=$service_name -q | head -n1) && \
	new_container_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $new_container_id) && \
	curl --silent --include --retry-connrefused --retry 30 --retry-delay 1 --fail http://$new_container_ip:$app_port/up || exit 1 && \
	docker stop $old_container_id && \
	docker rm $old_container_id && \
	sops exec-env encrypted.env 'docker compose -p sidekick up -d --scale $service_name=1 --no-recreate $service_name'
	`

var ForceDeployWithEnvScript = `
	export SOPS_AGE_KEY=$age_secret_key && \
	cd $service_name && \
	old_container_id=$(docker ps -f label="traefik.enable=true" -q | tail -n1) && \
	docker stop $old_container_id && \
	docker rm $old_container_id && \
	sops exec-env encrypted.env 'docker compose -p sidekick up -d'
	`

var DeployAppScript = `
	cd $service_name && \
	old_container_id=$(docker ps -f name=$service_name -q | tail -n1) && \
	docker compose -p sidekick up -d --no-deps --scale $service_name=2 --no-recreate $service_name && \
	new_container_id=$(docker ps -f name=$service_name -q | head -n1) && \
	new_container_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $new_container_id) && \
	curl --silent --include --retry-connrefused --retry 30 --retry-delay 1 --fail http://$new_container_ip:$app_port/up || exit 1 && \
	docker stop $old_container_id && \
	docker rm $old_container_id && \
	docker compose -p sidekick up -d --scale $service_name=1 --no-recreate $service_name
	`

var CheckGitTreeScript = `
	if [[ -z $(git status -s) ]]
	then
	  echo "all good"
	else
	  echo "tree is dirty, please commit changes before running this"
	  exit
	fi
	`

var SetupStageScript = `
#!/usr/bin/env bash
set -e

wait_for_locks() {
    echo "Waiting for apt/dpkg locks..."
    while fuser /var/lib/dpkg/lock >/dev/null 2>&1 \
       || fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 \
       || fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
        sleep 1
    done
}

echo "\033[0;32mUpdating SSH config...\033[0m"
sudo sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config && sudo systemctl restart ssh
sudo systemctl restart ssh

echo "\033[0;32mUpdating Packages...\033[0m"
wait_for_locks
sudo apt-get update -y

wait_for_locks
sudo apt-get upgrade -y

echo "\033[0;32mInstalling Necessities ...\033[0m"
wait_for_locks
sudo apt-get install -y age ca-certificates curl vim

echo "\033[0;32mInstalling SOPS...\033[0m"
curl -sLO https://github.com/getsops/sops/releases/download/v3.9.0/sops-v3.9.0.linux.amd64
sudo mv sops-v3.9.0.linux.amd64 /usr/local/bin/sops
sudo chmod +x /usr/local/bin/sops
`

var TraefikDockerComposeFile = `
services:
  traefik-service:
    image: traefik:v3.6.1
    command:
      - --api.insecure=false
      - --entrypoints.web.address=:80
      - --entrypoints.web.http.redirections.entryPoint.to=websecure
      - --entrypoints.web.http.redirections.entryPoint.scheme=https
      - --entrypoints.websecure.address=:443
      - --entrypoints.websecure.http.tls.certresolver=default
      - --providers.docker.exposedbydefault=false
      - --certificatesresolvers.default.acme.email=$EMAIL
      - --certificatesresolvers.default.acme.storage=/ssl-certs/acme.json
      - --certificatesresolvers.default.acme.httpchallenge.entrypoint=web
    ports:
      - "80:80"
      - "443:443"
      # The Web UI (enabled by --api.insecure=true)
      # - "8080:8080"
    volumes:
      # So that Traefik can listen to the Docker events
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik/ssl/:/ssl-certs/
    networks:
      - sidekick

networks:
  sidekick:
    external: true
`
