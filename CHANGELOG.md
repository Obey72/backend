# reup, how to reupload animations

---

## v1.7.1

- added **Themes** tab  50 built-in themes to choose from (midnight slate, dracula, nord dark, gruvbox dark, synthwave, and 45 more)
- added **community themes** section  submit your own theme as json, pending owner review before it goes live
- added **custom theme builder**  describe what you want in plain english and the assistant generates a theme for you, falls back to a local color parser if the backend is offline
- added **saved custom themes**  manually paste json to add and save your own theme, apply or delete any time
- theme assistant hits `/theme-assist` on the backend; offline fallback reads keywords (dark, light, blue, red, gold, etc.) and builds a reasonable palette locally
- version bumped to 1.7.1, github release tag updated

---

## before you start

- the place you're reuploading animations **from** must be **public** (not private, not friends-only), the tool downloads the original animations from roblox's servers and private places block that
- you must have **edit access** to the place you're reuploading **into**, this is the place that gets modified, not necessarily the same place
- you need a **roblox open cloud api key**, see step 1
- roblox studio must be installed

---

## step 1, create an api key

1. go to `https://create.roblox.com/dashboard/credentials?activeTab=ApiKeysTab`
2. click **create api key**
3. give it any name
4. under **experience operations**, not needed
5. under **assets** then add then select **read** and **write**
6. under **asset permissions** then add then select **write** (needed if you want to share access to reuploaded assets)
7. under **ip allowlist** then add `0.0.0.0/0` (allows all ips, safest for home use)
8. click **save**
9. copy the key, you only see it once

---

## step 2, set up the app

1. open **reup**
2. paste your api key into the **open cloud api key** field, it validates automatically, wait for the green checkmark
3. if it shows your user id automatically you're good, if not, paste your user id manually (find it at `roblox.com/users/profile`)

---

## step 3, install the plugin

1. in the app click **install plugin**
2. if roblox studio is already open it'll restart it automatically, let it
3. if it doesn't restart, close and reopen studio yourself
4. the plugin is now installed, you only need to do this once per app update

---

## step 4, enable studio settings

do this once, inside roblox studio:

1. go to **file then studio settings then security**
   - turn on **allow http requests**
2. go to **file then beta features**
   - turn on **lua asset creation api**
3. **restart studio** after enabling both, required, it won't work without restarting

---

## step 5, open your place

1. in the app, paste the **place id** of the place you want to scan (the one you have edit access to)
   - find it in the url on the roblox website: `roblox.com/games/PLACEID/...`
2. click **launch studio**, this opens studio directly into that place
3. wait for the plugin to connect, the app shows **connected** in green when it's ready
   - if it stays disconnected after 30 seconds, check that http requests are enabled in studio settings and restart studio

---

## step 6, configure upload destination

1. under **creator**, choose where the reuploaded animations will be owned:
   - **my account**, uploads under your personal account
   - **a group**, paste your group id, uploads under the group (your api key must have group permissions)
2. optionally under **permissions**, paste user ids or group ids to automatically grant them access to the new assets, leave blank to skip

---

## step 7, start reuploading

1. under **what to reupload**, turn on **animations**
2. click **start reuploading**
3. the plugin scans the place for every animation id, sends them to the app, downloads and reuploads each one, then rewrites every reference in the place to point at the new ids
4. watch the log, each line shows the old id then new id
5. wait until you see `=== finished ===`

---

## step 8, save

1. in roblox studio press **ctrl + s** to save the place
2. this is required, the replaced ids only exist in memory until you save
3. if you close studio without saving, nothing is committed and you'll have to redo it

---

## what to do if animations fail

**`Animation failed to load`**
- the original animation is private or owned by someone else's private account, can't be downloaded, nothing you can do

**`UploadFailed`**
- roblox rate limited you, the tool retries automatically with backoff, just wait it out

**`attempt to call a nil value` in log**
- the plugin installed in studio is outdated, click **install plugin** again in the app and restart studio

**`place X is private`**
- the source place is private, whoever owns it needs to make it public first, then retry

**plugin stays disconnected**
- http requests not enabled in studio then file then studio settings then security then allow http requests then restart studio
- beta features not on then file then beta features then lua asset creation api then restart studio

**animations still showing old ids after finish**
- you didn't save, press ctrl + s in studio

---

## notes

- the place you're **scanning** (source of animations) must be public, even if it's not the place being modified
- the place you're **editing** (destination) must be one you have edit access to
- reup runs entirely locally, nothing leaves your machine except requests to roblox's own api
- the `reup_data` folder next to the exe stores your saved cookies, api keys, user ids, and place ids for quick access next time
- if you move the exe, reup will search your drive for the existing `reup_data` folder automatically