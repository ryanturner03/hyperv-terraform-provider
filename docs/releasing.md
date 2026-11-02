# Releasing

Releases are built by GoReleaser in GitHub Actions and published to the
[GitHub releases page](https://github.com/ryanturner03/hyperv-terraform-provider/releases).
Pushing a semver tag is the only manual step.

## One-time setup

The release workflow signs the checksums file with GPG. Terraform mirrors and
the Terraform Registry require this signature. Configure two repository
secrets under **Settings → Secrets and variables → Actions**:

| Secret | Value |
|---|---|
| `GPG_PRIVATE_KEY` | ASCII-armored private key (`gpg --armor --export-secret-keys <key-id>`) |
| `PASSPHRASE` | Passphrase for that key. Leave empty if the key has none. |

To create a dedicated signing key:

```bash
gpg --full-generate-key          # RSA 4096, no expiry, name "terraform-provider-hyperv release"
gpg --list-secret-keys --keyid-format long
gpg --armor --export-secret-keys <key-id> | pbcopy   # paste into GPG_PRIVATE_KEY
gpg --armor --export <key-id> > docs/release-signing-key.asc   # commit the public key
```

Until the secrets exist, the release workflow fails fast with a clear error
and nothing is published.

## Cutting a release

1. Make sure `main` is green in CI and the changelog has an entry under a new
   version heading.
2. Tag and push:

   ```bash
   git tag -a v0.2.0 -m "v0.2.0"
   git push origin v0.2.0
   ```

3. The `release` workflow builds six binaries (linux, darwin, windows × amd64,
   arm64), zips them, writes `*_SHA256SUMS` and `*_SHA256SUMS.sig`, attaches
   the registry manifest, and creates the GitHub release with notes generated
   from commit messages (`feat:` and `fix:` prefixes are grouped).

Pre-release tags such as `v0.2.0-rc1` are published as GitHub pre-releases.

## Verifying a release locally

```bash
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest build --snapshot --clean
ls dist/
```

CI runs the same two commands on every pull request, so a broken
`.goreleaser.yml` or a platform that no longer compiles is caught before
tagging.

## Artifact layout

```
terraform-provider-hyperv_0.2.0_linux_amd64.zip      # contains terraform-provider-hyperv_v0.2.0
terraform-provider-hyperv_0.2.0_windows_amd64.zip    # contains terraform-provider-hyperv_v0.2.0.exe
...
terraform-provider-hyperv_0.2.0_SHA256SUMS
terraform-provider-hyperv_0.2.0_SHA256SUMS.sig
terraform-provider-hyperv_0.2.0_manifest.json
```

This is the layout the Terraform Registry expects, so publishing there later
only requires registering the public key and the repository with the registry;
no changes to the build are needed. Note that registry publishing would also
mean changing the provider address from `ryan/hyperv` to the registry
namespace, which existing state files would need `terraform state replace-provider`
to follow.

## Consuming releases

End users run `scripts/install.sh` or `scripts/install.ps1` (see the README).
Both scripts download the zip and the checksums file, verify the SHA256, and
unpack into Terraform's implied local mirror directory, so `terraform init`
works with no CLI configuration. `install.sh` accepts `BASE_URL` to download
from an internal mirror instead of GitHub.
