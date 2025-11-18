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

# Simple script to delete all droplets tagged by sidekick tag after I finish playing around.

#!/usr/bin/env python3
import os
import time
import requests

API = "https://api.digitalocean.com/v2"
DROPLETS_TAG = "sidekick"

token = os.getenv("DO_TOKEN")
if not token:
    print("❌ Please set DO_TOKEN environment variable")
    exit(1)

headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}


# destroy all droplets tagged with Sidekick
name = f"cheap-sgp1-{int(time.time())}"
r = requests.delete(f"{API}/droplets?tag_name={DROPLETS_TAG}", headers=headers)
r.raise_for_status()
# droplet_id = r.json()["droplet"]["id"]

print("✅ All droplets under Sidekick tag have been destoyed")

exit(1)
