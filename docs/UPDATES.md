# Application updates

Modbus Studio updates itself from the GitHub Releases of `elmnaveen/AppMODBUS`.

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
| Windows | `ModbusStudio-windows-amd64.exe` | Renames the running exe aside and puts the new one in place. If the folder isn't writable (e.g. `Program Files`), it runs the signed NSIS installer `ModbusStudio-windows-amd64-installer.exe`, which asks for admin rights. |
| macOS | `ModbusStudio-macos-universal.zip` | Swaps the whole `ModbusStudio.app` bundle, then runs `open -n`. |
| Linux (portable) | `ModbusStudio-linux-amd64.tar.gz` | Swaps the binary. |
| Ubuntu `.deb` | `ModbusStudio-ubuntu-18.04-to-22.04-amd64.deb` or `ModbusStudio-ubuntu-24.04-amd64.deb`, matching the installed one | Runs `pkexec dpkg -i` (desktop password prompt), then restarts the app. |

The old version is deleted on the next start.

Settings (channel, automatic check, skipped version) are stored in
`<user config dir>/ModbusStudio/settings.json`, for example `%AppData%\ModbusStudio` on Windows,
`~/Library/Application Support/ModbusStudio` on macOS and `~/.config/ModbusStudio` on Linux.

## One-time setup: signing key

1. Generate a key pair locally:

   ```sh
   go run ./cmd/updatekey generate
   ```

2. In GitHub, open **Settings → Secrets and variables → Actions → New repository secret**.
   Create `UPDATE_SIGNING_KEY` and paste in the first (secret) value. Don't commit it or share it.
3. Put the second (public) value in `PinnedUpdatePublicKey` in `internal/version/version.go`
   and commit it. The public key is safe to publish.

Every build has the pinned public key built in. CI runs `updatekey check` before building and
before signing. If the secret doesn't match the pinned key, the run fails, so a wrong or
replaced secret can never publish a release that installed copies would reject.

> Keep a backup of the secret somewhere safe, such as a password manager. Installed copies
> only accept updates signed with this key.

### Rotating the key

Only do this if the secret is lost or leaked. Generate a new pair, update the secret and
`PinnedUpdatePublicKey` together, and release. Copies built with the old key can't verify the
new release, so their users must download it once by hand from the release page. After that,
automatic updates work again.

## Publishing a release

**From GitHub (recommended):** open **Actions → Build → Run workflow**, keep the branch on
**main**, enter the version (e.g. `0.3.0`, or `0.4.0-beta.1` for a beta) and click
**Run workflow**. The same run:
- tests and builds all three platforms with that version;
- writes `checksums.txt` for all packages, then signs and verifies it;
- creates the `v…` tag on the commit it built;
- publishes the release with auto-generated notes.

It refuses to run if the version is malformed, the tag already exists, or the branch isn't
`main`. Leave the version empty for a test build that publishes nothing.

**From a terminal:** pushing a tag does the same thing:

```sh
git tag v0.3.0          # stable
git tag v0.4.0-beta.1   # beta: any version with "-" becomes a GitHub pre-release
git push origin --tags
```

Edit the release notes on GitHub if you like; they appear in the app's **What's new**
section, which supports headings, bullet lists, **bold** and `code`.

The repository (or at least its releases) must be **public**. The app reads releases
anonymously, so a private repository returns "no releases found".

## Testing against a mirror

Set `MODBUS_STUDIO_UPDATE_API=http://host:port` to point the updater at a GitHub-compatible
API, for example a local test server. Signature verification still applies.

## Coming from AppMODBUS

Modbus Studio was called AppMODBUS up to v2.0.0. Everything now uses the new name: release packages
(`ModbusStudio-…`), the executable (`ModbusStudio.exe`, `ModbusStudio.app`, `/opt/ModbusStudio`), the Debian package
(`modbus-studio`), the Windows install folder (`C:\Program Files\Modbus Studio`) and the settings and recordings folders
(`ModbusStudio`).

- **Updates:** AppMODBUS installs look for `AppMODBUS-…` files and do not find Modbus Studio releases. Install Modbus
  Studio once by hand; it updates itself from then on.
- **Installing over it:** the Windows installer offers to remove AppMODBUS (it removes it without asking in silent
  mode), and the `modbus-studio` package replaces the `appmodbus` package.
- **Settings and data:** on its first start Modbus Studio copies the AppMODBUS config folder (settings, session,
  profiles, alarm history, logs) and moves the default recordings folder. If the recordings cannot be moved, they stay
  where they are and the settings point to them. Theme and menu style are per-installation UI preferences and start at
  their defaults.
- **Files:** workspaces (`.mbw`) and device profiles saved by AppMODBUS open as before.
