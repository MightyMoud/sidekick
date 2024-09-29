/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

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

var ImageMoveScript = `
	appName=$1
	user=$2
	server=$3
	tag=${4:-"latest"}

	scp -C $appName-$tag.tar $user@$server:./$appName

	rm $appName-$tag.tar
	`

var DockerBuildAndSaveScript = `
	appName=$1
	projectFolder=$2
	tag=${3:-"latest"}

	docker build --cache-from=$appName:latest --tag $appName:$tag --platform linux/amd64 $projectFolder 

	docker save -o $appName-$tag.tar $appName
	`

var EnvEncryptionScript = `
	PUBKEY=$1
	ENVFILE=$2

	sops encrypt --age $PUBKEY $ENVFILE > encrypted.env
	`

var DeployAppWithEnvScript = `
	cd $service_name && \
	old_container_id=$(docker ps -f name=$service_name -q | tail -n1) && \
	sops exec-env encrypted.env 'docker compose -p sidekick up -d --no-deps --scale $service_name=2 --no-recreate $service_name' && \
	new_container_id=$(docker ps -f name=$service_name -q | head -n1) && \
	new_container_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $new_container_id) && \
	curl --silent --include --retry-connrefused --retry 30 --retry-delay 1 --fail http://$new_container_ip:$app_port/ || exit 1 && \
	docker stop $old_container_id && \
	docker rm $old_container_id && \
	sops exec-env encrypted.env 'docker compose -p sidekick up -d --no-deps --scale $service_name=1 --no-recreate $service_name'
	`

var DeployAppScript = `
	cd $service_name && \
	old_container_id=$(docker ps -f name=$service_name -q | tail -n1) && \
	docker compose -p sidekick up -d --no-deps --scale $service_name=2 --no-recreate $service_name && \
	new_container_id=$(docker ps -f name=$service_name -q | head -n1) && \
	new_container_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $new_container_id) && \
	curl --silent --include --retry-connrefused --retry 30 --retry-delay 1 --fail http://$new_container_ip:$app_port/ || exit 1 && \
	docker stop $old_container_id && \
	docker rm $old_container_id && \
	docker compose -p sidekick up -d --no-deps --scale $service_name=1 --no-recreate $service_name
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
