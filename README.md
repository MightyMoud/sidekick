# Sidekick

<div align="center">
  <div>
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91c-1f3fb.png">
    <img width="110px" src="https://emoji.aranja.com/static/emoji-data/img-apple-160/1f91b-1f3fb.png">
  </div>

Bare metal to production ready in mins; imagine fly.io on your VPS

![GitHub](https://img.shields.io/github/license/ms-mousa/sidekick)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ms-mousa/sidekick)
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/ms-mousa/sidekick)

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
