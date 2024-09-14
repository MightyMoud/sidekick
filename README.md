
<div align="center">
  <h1>Sidekick</h1>
  <div>
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91c-1f3fb.png">
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91b-1f3fb.png">
  </div>

Bare metal to production ready in mins; imagine fly.io on your VPS

![GitHub](https://img.shields.io/github/license/ms-mousa/sidekick)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ms-mousa/sidekick)
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/ms-mousa/sidekick)

  <div>
    <img width="500px" src="/demo/imgs/hero.png">
  </div>
</div>

## Features
- 👍 One command VPS setup (docker, traefik, sops, age)
- 💻 deploy any application from a dockerfile
- ✊ Zero downtime deployment
- 🌏 High availbility and load balancing
- 🔒 Zero config TLS Certs
- ✅ Connect domains or use sslip.io out of the box
- 🔥 Built in integration with SOPS
- 🛸 Escape the vendorlock forever


## Motivation
I'm tired of the complexity involved in hosting my side projects. While some platforms, like Fly.io, stand out in the crowded field of Heroku replacements, I believe a simple VPS can be just as effective. That's why I created Sidekick: to make hosting side projects as straightforward, affordable, and production-ready as possible. You'll be surprised how much traffic a $8/month instance on DigitalOcean can handle.

## Installation
With GO installed on your system you need to run
```bash
go install github.com/mightymoud/sidekick@latest
```

## Usage
Sidekick helps you along all the steps of deployment on your VPS. From basic setup to zero downtime deploys, we got you! ✊
First you need a VPS with Ubuntu LTS. I recommend DigitalOcean. Hetzner also gets very good reviews. You can host your own silicon too. As long as you have a public IP address you good to go and use sidekick. 

Just make sure the following is true
- VPS running Ubuntu - LTS recommended
- SSH Public Key availble on your machine to login to VPS.

That's it, you're ready to use Sidekick
### VPS Setup
First you need to setup your VPS. To do this you need to run:
```bash
sidekick init
```
Then you need to enter the following:
- IP Address of your VPS
- An email address to use for setting up TSL certs
- Docker registery to host your docker images - defaults to `docker.io`
- Docker username in the said registery
- Confirm you are currently logged in to that said registery with the username - This is needed to be able to push images on your behalf

After that Sidekick will setup many things on your VPS - Usually takes around 2 mins
<details>
  <summary>What does Sidekick do when I run this command</summary>
  
* Login with `root` user
* Make a new user `sidekick` and grant sudo access
* Logout from `root` and login with `sidekick`
* Update and upgrade your Ubuntu system
* Install `sops` and copy over the public key to your sidekick config file
* Install Docker
* Setup Traefik and TLS certs on your VPS
</details>

### Launch a new application
  <div align="center" >
    <img width="500px" src="/demo/imgs/launch.png">
  </div>

In your application folder, make sure you have a working `Dockerfile` that you can build and run. Also make sure you know at which port your app is expecting to recieve traffic.

Then run:
```bash
sidekick launch
```
Then you need to enter the following:
- Url friendly name of your app - if you opt to use `sslip.io` domain for testing this would be your subdomain
- HTTP exposed port for your app to get requests - Sidekick will scan your docker file to try to extract this number and default it.
- Domain at which you want this application to be reachable - If you choose your own domain make sure to point the domain to your VPS IP address; otherwise we default to `sslip.io` domain so you can play around. 
- If you have any `env` file with secrets in it. Sidekick will attempt to find `.env` file in the root of your folder. Sidekick will use `sops` to encrypt your env file and inject the values securely at run time. 

Should take around 2 more mins to be able to visit your application live on the web if all goes well. 

<details>
  <summary>What does Sidekick do when I run this command</summary>
  
* Build your docker image locally for linux
* Push the docker image to the registery
* Encrpt your env file, if available and push it to your VPS
* Spin up your docker image and route traffic to it on the specified port using Traefik
</details>


## Inspiration
- https://fly.io/
- https://kamal-deploy.org/

## Vision
Simple CLI tool that can help you:
- Setup your VPS
- Deploy all your side projects on a single VPS
- Load balance multiple container per project
- Deploy new versions with Zero downtime
- Deploy preview environments with ease
- Manage env secrets in a secure way
- Connect any number of domains and subdomains to your projects with ease


## Roadmap
- Zero downtime deployments
- Inject env secrets securely at run-time
- Deploy preview environments of any application with ease
- Handle more complex projects built with docker compose

## Demo
- Normal deployment
  Empty nextjs project -> run `sidekick launch` -> app live with URL
- Normal deployment + env file
  Empty nextjs project -> make `sidekick.env.yaml` file with two lists "clear" & "secret" -> run `sidekick env` -> run `sidekick lanuch` -> app live with URL
- Normal deployment + env file + new version
  Same project -> run `sidekick deploy` -> Zero downtime deployment -> message when deployment is done and app is healthy
- Normal deployment + env file + preview env
  Make change into last project in home page -> commit file -> run `sidekick deploy preview` -> preview env live with URL

- Docker compose deployment
  Project with docker compose -> run `sidekick launch` -> app live with URL
- Docker compose deployment with env file
  Project with docker compose -> make `sidekick.env.yaml` file with two lists "clear" & "secret" -> run `sidekick env` -> run `sidekick launch` -> app live with URL

- Deploy accessory (mysql, pg, redis)
  Project with just docker file -> run `sidekick accessory pg` -> ask couple of questions -> db live with connection string

## Improvements
- Add new user called `sidekick` and use that instead of root
- Disable password sign in with SSH - Only ssh keys allowed
- Use watch tower to make zero down time deploys instead
- Renew certs somehow
- Ask for email for SSL certs
