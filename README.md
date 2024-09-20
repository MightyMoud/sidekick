<div align="center">
  <div>
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91c-1f3fb.png">
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91b-1f3fb.png">
  </div>

Bare metal to production ready in mins; imagine fly.io on your VPS
  <div>
    <img width="500px" src="/demo/imgs/hero.png">
  </div>

![GitHub](https://img.shields.io/github/license/mightymoud/sidekick)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/mightymoud/sidekick)
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/mightymoud/sidekick)

</div>

## Features
- 👍 One command VPS setup (docker, traefik, sops, age)
- 💻 deploy any application from a dockerfile
- ✊ Zero downtime deployment
- 🌏 High availability and load balancing
- 🔒 Zero config SSL Certs
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

First you need a VPS with Ubuntu LTS. I recommend DigitalOcean. Hetzner also gets very good reviews. You can host your own silicon too. As long as you have a public IP address you can use Sidekick.

Just make sure the following is true:
- VPS running Ubuntu - LTS recommended
- SSH Public Key availble on your machine to login to VPS.

That's it!
### VPS Setup

  <div align="center" >
    <img width="500px" src="/demo/imgs/init.png">
  </div>


First you need to setup your VPS. To do this you need to run:
```bash
sidekick init
```

Then you need to enter the following:
- IP Address of your VPS
- An email address to use for setting up SSL certs
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
* Use `age` to make secret and public keys to use later for encrypting env file.
* Send public key back to host machine to be used later for encryption
* Install Docker
* Setup Traefik and SSL certs on your VPS
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
* Use sops to decrypt your env file and start and env with the values injected
* Spin up your docker image using docker compose and route traffic to it using Traefik on the specified port
</details>

### Deploy a new version 
  <div align="center" >
    <img width="500px" src="/demo/imgs/deploy.png">
  </div>
With your application deployed, it's super simple to redeploy a new version.

At any point any time you need to only run:
```bash
sidekick deploy
```
That's all. It won't take long, we use cache from earlier docker images, your latest version should be up soon.
Sidekick will deploy the new version without any downtime - you can see more in the source code.
This command will also do a couple of things behind the scenes. You can check that below


<details>
  <summary>What does Sidekick do when I run this command</summary>
  
* Build your docker image locally for linux
* Compare your latest env file checksum for changes from last time you deployed your application.
* If your env file has changed, sidekick will re encrypt it and replace the encrypte.env file on your server.
* Deploy the new version with zero downtime deploys so you don't miss any traffic. 
</details>

### Deploy a preview environment
  <div align="center" >
    <img width="500px" src="/demo/imgs/preview.png">
  </div>
Sidekick also allows you to deploy preview envs at any point from your application. Preview envs are attached to your commit hash and require a clean git tree before you can initiate them. 
Once you have a clean git tree, you can run the following command to deploy a preview environment:

```bash
sidekick deploy preview
```


<details>
  <summary>What does Sidekick do when I run this command</summary>
  
* Build your docker image locally for linux
* Tag the new image with the short checksum of your git commit
* Compare your latest env file checksum for changes from last time you deployed your application.
* If your env file has changed, sidekick will re encrypt it and replace the encrypte.env file on your server.
* Add a new folder inside your app folder called "preview" where Sidekick will store and manage all your preview deployments
* Deploy a new version of your app reachable on a short hash based subdomain
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

## Remove sidekick

You can easily remove sidekick if you hate it.

```bash
// Mac/Linux
cd $HOME/go/bin
rm sidekick
cd ~/.config
rm -rf sidekick

// Win
cd %USERPROFILE%\go\bin
del sidekick.exe

// Powershell
cd $HOME\go\bin
Remove-Item sidekick.exe
```

---
## Roadmap
I still have a couple more feature I want to add here. Also considering some of those to be on a paid version.
- Preview env deployments
- A way to deploy more complicated projects defined in docker compose file
- Better zero downtime deploys with watchtower
- Firewall setup
- Managing multiple VPSs
- Easy way to deploy databases with one command
- TUI for monitoring your VPS
- Streaming down compose logs - ala `fly logs`
- Auto deploy on image push - to work with CICD better
