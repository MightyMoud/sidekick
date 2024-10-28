<div align="center">
  <div>
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91c-1f3fb.png">
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91b-1f3fb.png">
  </div>

Bare metal to production ready in mins; imagine fly.io on your VPS

  <div>
    <img width="600px" src="/demo/imgs/hero.png">
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

Using brew:

```bash
brew install --formula sidekick
```

NOTE: Sidekick uses `brew` later on to handle installing `sops` on your local. So `brew` is a requirement at this point. Sidekick will throw an error if `brew` is not found. You can install `brew` from [here](https://brew.sh/).

## Usage

Sidekick helps you along all the steps of deployment on your VPS. From basic setup to zero downtime deploys, we got you! ✊

First you need a VPS with Ubuntu LTS. I recommend DigitalOcean. Hetzner also gets very good reviews. You can host your own silicon too. As long as you have a public IP address you can use Sidekick.

Just make sure the following is true:

- VPS running Ubuntu - LTS recommended
- SSH Key available on your machine to login to VPS.

That's it!

### VPS Setup

  <div align="center" >
    <img width="600px" src="/demo/imgs/init.png">
  </div>

First you need to setup your VPS. To do this you need to run:

```bash
sidekick init
```

Then you need to enter the following:

- IP Address of your VPS
- An email address to use for setting up SSL certs

After that Sidekick will setup many things on your VPS - Usually takes around 2 mins.
If you run this command once more and enter a different IP Address, Sidekick will warn you that you are overriding the current config with a prompt.

You can use flags instead. Read more [in the docs](https://www.sidekickdeploy.com/docs/command/init/).

<details>
  <summary>What does Sidekick do when I run this command?</summary>
  
* Login with `root` user
* Make a new user `sidekick` and grant sudo access
* Logout from `root` and login with `sidekick`
* Disable login with `root` user - security best practice
* Update and upgrade your Ubuntu system
* Install `sops` and copy over the public key to your sidekick config file
* Use `age` to make secret and public keys to use later for encrypting env file.
* Send public key back to host machine to be used later for encryption
* Install Docker
* Add user sidekick to docker group
* Setup Traefik and SSL certs on your VPS
</details>

<details>
  <summary>Which SSH key will Sidekick use to login?</summary>

Sidekick will look up the default keys in your default .ssh directory in the following order:

- id_rsa.pub
- id_ecdsa.pub
- id_ed25519.pub

Sidekick will also get all keys from the `ssh-agent` and try them as well. If you want to use a custom key and not a default one, you would need to add the to your agent first by running `ssh-add KEY_FILE`

</details>

Read more details about flags and other options for this command [on the docs](https://www.sidekickdeploy.com/docs/command/init/)

### Launch a new application

  <div align="center" >
    <img width="600px" src="/demo/imgs/launch.png">
  </div>

In your application folder, make sure you have a working `Dockerfile` that you can build and run. Also make sure you know at which port your app is expecting to receive traffic.

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
* Move the docker image to your VPS directly
* Encrypt your env file, if available and push it to your VPS
* Use sops to decrypt your env file and start and env with the values injected
* Spin up your docker image using docker compose and route traffic to it using Traefik on the specified port
</details>

### Deploy a new version

  <div align="center" >
    <img width="600px" src="/demo/imgs/deploy.png">
  </div>
With your application deployed, it's super simple to deploy a new version.

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
* If your env file has changed, sidekick will re-encrypt it and replace the encrypted.env file on your server.
* Deploy the new version with zero downtime deploys so you don't miss any traffic. 
</details>

### Deploy a preview environment/app

  <div align="center" >
    <img width="600px" src="/demo/imgs/preview.png">
  </div>
Sidekick also allows you to deploy preview apps at any point from your application. Preview apps are attached to your commit hash and require a clean git tree before you can initiate them. 
Once you have a clean git tree, you can run the following command to deploy a preview app:

```bash
sidekick deploy preview
```

<details>
  <summary>What does Sidekick do when I run this command</summary>
  
* Build your docker image locally for linux
* Tag the new image with the short checksum of your git commit
* Encrypt your env file, if available and push it to your VPS
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
brew uninstall sidekick
```

---

## Roadmap

I still have a couple more feature I want to add here. Also considering some of those to be on a paid version.

- ✅ Preview env deployments
- A way to deploy more complicated projects defined in docker compose file
- Better zero downtime deploys with watchtower
- Firewall setup
- Managing multiple VPSs
- Easy way to deploy databases with one command
- TUI for monitoring your VPS
- Streaming down compose logs - ala `fly logs`
- Auto deploy on image push - to work with CICD better
- Git hooks setup for managing migrations and other concerns
