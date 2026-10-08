# Releasing

## Versioning

Releases follow [Semantic Versioning](https://semver.org/). The version is
derived from the git tag — there is no version file to update manually.
GoReleaser injects the tag into the binary.

## Release Process

Tag the commit and push:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The GitHub Actions workflow (`.github/workflows/release.yml`) handles
everything else.

## Local Test Build

To build packages locally without publishing or signing:

```bash
# Build CLI-only RPM for your host architecture
./scripts/build-packages.sh cli

# Build full server package (rebuilds React UI)
./scripts/build-packages.sh server

# Build both
./scripts/build-packages.sh all
```

The script uses [nfpm](https://nfpm.goreleaser.com/). It auto-detects your host architecture, package format (RPM on
Fedora/RHEL, DEB on Debian/Ubuntu), and generates a dev version from the latest
git tag (e.g. `v1.1.1` → `1.1.2~dev1`).

Override any default:

```bash
./scripts/build-packages.sh cli --arch arm64 --format deb --version 2.0.0~beta1
```

Packages are written to `dist/`. See
[Building from Source](building-from-source.md) for prerequisites and full
usage.

## Testing Packages

`scripts/test-packages.sh` installs a server package in a container running
systemd: a `.deb` on Debian, a `.rpm` on Fedora. It checks that the hub comes
up with no step in between when the package is installed over the last 1.5.x
release, on a host with a TPM and on one without, and on a fresh install:

```bash
./scripts/test-packages.sh dist/sensor-hub_*_linux_amd64.deb
./scripts/test-packages.sh dist/sensor-hub_*_linux_amd64.rpm --from 1.5.2
```

It needs curl, jq and a Docker that can run privileged containers, and uses the
package for the architecture Docker runs. CI's Package Tests job builds the
packages with GoReleaser, as the release does, and runs it on both.

## GPG Key Management

Release packages (RPM and DEB) are signed with GPG so users can verify package
integrity before installation.

| Secret / File               | Location                           | Purpose              |
|-----------------------------|------------------------------------|----------------------|
| `GPG_PRIVATE_KEY`           | GitHub repo secret                 | Signs packages       |
| `GPG_PASSPHRASE`            | GitHub repo secret                 | Unlocks private key  |
| `sensor-hub-gpg-public.key` | Repo root and every GitHub Release | Verifies signatures  |

### Rotating the GPG Key

1. Generate a new keypair:
   ```bash
   gpg --full-generate-key   # choose RSA 4096
   ```
2. Export the private key:
   ```bash
   gpg --armor --export-secret-keys FINGERPRINT > new-key.key
   ```
3. Update the GitHub repository secret `GPG_PRIVATE_KEY` with the contents of
   `new-key.key`.
4. Update `GPG_PASSPHRASE` if the passphrase changed.
5. Export the public key:
   ```bash
   gpg --armor --export FINGERPRINT > sensor-hub-gpg-public.key
   ```
6. Commit the updated `sensor-hub-gpg-public.key` to the repo root.
7. **Delete the exported private key file** — it must not be committed:
   ```bash
   rm new-key.key
   ```

> **Note:** Old releases retain their original signatures. Only new releases
> will be signed with the new key.
