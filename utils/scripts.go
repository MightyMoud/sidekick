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

var DeployApp = `
#!/usr/bin/env bash
set -euo pipefail

# minimal configuration (edit as needed)
SERVICE="$service_name"
APP_PORT="$app_port"
SLEEP_AFTER_START=3
COMPOSE_PROJECT="sidekick"

# helper for nicer logs
log() { echo "[$(date +'%T')] $*"; }

# move into service dir (assumes compose file lives in ./<service>/)
if [[ ! -d "$SERVICE" ]]; then
  log "ERROR: service directory '$SERVICE' not found."
  exit 2
fi

cd "$SERVICE"

log "Starting rolling replace for service='$SERVICE', port=$APP_PORT"

# find the old container (oldest for this service)
old_container_id=$(docker ps -f "name=${SERVICE}" -q | tail -n1 || true)
if [[ -z "$old_container_id" ]]; then
  log "ERROR: no running containers found for service '${SERVICE}'."
  exit 3
fi
log "Old container (to be replaced): $old_container_id"

# create a new instance by scaling up to 2 (no deps, don't recreate existing)
log "Scaling up to 2 (creating a new container)..."
docker compose -p "$COMPOSE_PROJECT" up -d --no-deps --scale "$SERVICE"=2 --no-recreate "$SERVICE"

# optional small wait for the container to begin initializing
if (( SLEEP_AFTER_START > 0 )); then
  log "Sleeping $SLEEP_AFTER_START seconds for startup..."
  sleep "$SLEEP_AFTER_START"
fi

# find newest container for this service
new_container_id=$(docker ps -f "name=${SERVICE}" -q | head -n1 || true)
if [[ -z "$new_container_id" ]]; then
  log "ERROR: failed to detect new container after scaling."
  exit 4
fi

# safety: ensure new != old
if [[ "$new_container_id" == "$old_container_id" ]]; then
  log "ERROR: detected the same container as new and old ($new_container_id). Aborting."
  exit 5
fi

log "New container: $new_container_id"

# get internal IP of the new container
new_container_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$new_container_id" || true)
if [[ -z "$new_container_ip" ]]; then
  log "ERROR: could not determine IP of new container $new_container_id"
  # clean up the new container to avoid leaving an extra one
  docker rm -f "$new_container_id" || true
  # restore scale to 1 (best effort)
  docker compose -p "$COMPOSE_PROJECT" up -d --scale "$SERVICE"=1 --no-recreate "$SERVICE" || true
  exit 6
fi

log "New container IP: $new_container_ip"

# health check (preserve your curl options)
HEALTH_URL="http://$new_container_ip:$APP_PORT/"
log "Health checking $HEALTH_URL (this may retry internally via curl)..."

if ! curl --silent --include --retry-connrefused --retry 30 --retry-delay 1 --fail "$HEALTH_URL" >/dev/null 2>&1; then
  log "ERROR: health check failed against $HEALTH_URL"
  log "Removing failed new container $new_container_id and restoring state..."
  docker rm -f "$new_container_id" || true
  docker compose -p "$COMPOSE_PROJECT" up -d --scale "$SERVICE"=1 --no-recreate "$SERVICE" || true
  exit 7
fi

log "Health check passed. Swapping containers..."

# stop & remove the old container (now safe)
docker stop "$old_container_id"
docker rm "$old_container_id"

# scale back to 1 (remove the spare)
docker compose -p "$COMPOSE_PROJECT" up -d --scale "$SERVICE"=1 --no-recreate "$SERVICE"

log "Done — replaced $old_container_id with $new_container_id and scaled to 1."

exit 0


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
