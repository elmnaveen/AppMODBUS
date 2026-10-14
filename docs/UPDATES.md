# Application updates

AppMODBUS updates itself from the GitHub Releases of `elmnaveen/AppMODBUS`.

## How it works

1. On startup (if enabled) and from **About → Check now**, the app reads
   `GET https://api.github.com/repos/elmnaveen/AppMODBUS/releases`. It picks the newest
   release that is newer than the running version: stable releases only on the **Stable**
   channel, or pre-releases too on **Beta**. Drafts are ignored, and so are releases that
   are missing the package for this platform or their signed checksums.
2. A banner offers **Install & restart**, **What's new**, **Skip this version** and **Later**.
3. On install, the app:
   - downloads `checksums.txt` and `checksums.txt.sig`;
   - checks the Ed25519 signature against the public key compiled into the build;
   - downloads the package for this platform and checks its SHA-256 against the signed list;
   - swaps the files and restarts.

   If any check fails, nothing is changed.

| Platform | Package | Install method |
| --- | --- | --- |
| Windows | `AppMODBUS-windows-amd64.exe` | Renames the running exe aside and puts the new one in place. If the folder isn't writable (e.g. `Program Files`), it runs the signed NSIS installer `AppMODBUS-windows-amd64-installer.exe`, which asks for admin rights. |
| macOS | `AppMODBUS-macos-universal.zip` | Swaps the whole `AppMODBUS.app` bundle, then runs `open -n`. |
| Linux | `AppMODBUS-linux-amd64.tar.gz` | Swaps the binary. |

The old version is deleted on the next start.

Settings (channel, automatic check, skipped version) are stored in
`<user config dir>/AppMODBUS/settings.json`, for example `%AppData%\AppMODBUS` on Windows,
`~/Library/Application Support/AppMODBUS` on macOS and `~/.config/AppMODBUS` on Linux.

## One-time setup: signing key

1. Generate a key pair locally:

   ```sh
   go run ./cmd/updatekey generate
   ```

2. In GitHub, open **Settings → Secrets and variables → Actions → New repository secret**.
   Create `UPDATE_SIGNING_KEY` and paste in the first (secret) value. Don't commit it or share it.
3. That's all. CI derives the public key from the secret and compiles it into every build.

> Keep a backup of the secret somewhere safe, such as a password manager. Installed copies
> only accept updates signed with this key. If you lose or change it, every user has to
> download one new version manually.

Builds made without the secret (local `wails build`, forks) can still check for updates.
They show a **Download** button instead of installing automatically.

## Publishing a release

```sh
git tag v0.2.0          # stable
git tag v0.3.0-beta.1   # beta: any tag with "-" becomes a GitHub pre-release
git push origin --tags
```

The `Build` workflow builds all three platforms with the version taken from the tag, then:
- writes `checksums.txt` for all packages;
- signs and verifies it;
- publishes the release with auto-generated notes.

Edit the release notes on GitHub if you like; they appear in the app's **What's new**
section, which supports headings, bullet lists, **bold** and `code`.

The repository (or at least its releases) must be **public**. The app reads releases
anonymously, so a private repository returns "no releases found".

## Testing against a mirror

Set `APPMODBUS_UPDATE_API=http://host:port` to point the updater at a GitHub-compatible
API, for example a local test server. Signature verification still applies.
