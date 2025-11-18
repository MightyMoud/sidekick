# Copyright © 2024 Mahmoud Mousa <m.mousa@hey.com>
#
# Licensed under the GNU GPL License, Version 3.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# https://www.gnu.org/licenses/gpl-3.0.en.html
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Simple script to make droplets and set them up with sidekick for my testing
# Sick of using the UI

#!/usr/bin/env python3
import os
import time
import requests
import subprocess


API = "https://api.digitalocean.com/v2"
REGION = "sgp1"
IMAGE = "ubuntu-24-04-x64"
SIZE = "s-1vcpu-2gb"
SSH_KEY_NAME = "Mac mini-new"

token = os.getenv("DO_TOKEN")
if not token:
    print("❌ Please set DO_TOKEN environment variable")
    exit(1)

headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

# find SSH key named "Mac mini"
r = requests.get(f"{API}/account/keys", headers=headers)
r.raise_for_status()
keys = r.json()["ssh_keys"]
key = next((k for k in keys if k["name"] == SSH_KEY_NAME), None)
if not key:
    print(f"❌ No SSH key found named '{SSH_KEY_NAME}'.")
    print("Available keys:", [k["name"] for k in keys])
    exit(1)

# create droplet
name = f"cheap-sgp1-{int(time.time())}"
payload = {
    "name": name,
    "region": REGION,
    "size": SIZE,
    "image": IMAGE,
    "ssh_keys": [key["id"]],
    "backups": False,
    "ipv6": False,
    "tags": ["sidekick"],
}
r = requests.post(f"{API}/droplets", headers=headers, json=payload)
r.raise_for_status()
droplet_id = r.json()["droplet"]["id"]

print(f"✅ Droplet created (ID: {droplet_id}), waiting for IP...")

# wait for IPv4
for _ in range(120):  # ~10 minutes
    time.sleep(5)
    r = requests.get(f"{API}/droplets/{droplet_id}", headers=headers)
    droplet = r.json()["droplet"]
    networks = droplet["networks"]["v4"]
    ipv4 = next((n["ip_address"] for n in networks if n["type"] == "public"), None)
    if ipv4:
        print(ipv4)
        # print("   Giving the VPS time to boot")
        # time.sleep(10)
        # cmd = ["sidekick", "init", f"-s={ipv4}", "-e=m.mousa@hey.com", "-y"]
        # subprocess.run(cmd, check=True)
        exit(0)
    status = droplet["status"]
    print(f"   still {status}...")

print("❌ Timed out waiting for droplet IP.")
exit(1)
