# WTree Release Process

This document outlines how to release new versions of WTree.

## Prerequisites

1. **GoReleaser installed**:
   ```bash
   # macOS
   brew install goreleaser
   
   # Linux
   curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh
   ```

2. **GitHub Token** with repo permissions:
   ```bash
   export GITHUB_TOKEN="your_github_token"
   ```

## Release Steps

### 1. Prepare Release

1. **Update CHANGELOG.md**:
   - Add new version section
   - Document all changes since last release
   - Follow [Keep a Changelog](https://keepachangelog.com/) format

2. **Update version references** (if needed):
   - Check README examples
   - Verify documentation links

3. **Test everything**:
   ```bash
   make test
   make build
   ./wtree --help
   ```

### 2. Create Release

#### Option A: Automated with GoReleaser

1. **Tag the release**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Let GitHub Actions handle the release** (automatic via `.github/workflows/release.yml`)

#### Option B: Manual with GoReleaser

1. **Create release locally**:
   ```bash
   # Check config
   make goreleaser-check
   
   # Test build (doesn't release)
   make goreleaser-build
   
   # Create actual release
   make goreleaser-release
   ```

### 3. Post-Release Tasks

1. **Verify release**:
   - Check [GitHub Releases](https://github.com/awhite/wtree/releases)
   - Verify all binaries are present
   - Test installation methods

2. **Update Homebrew tap** (if not automated):
   - GoReleaser should handle this automatically
   - If manual: Update formula in `homebrew-tap` repository

3. **Test installation**:
   ```bash
   # Test Go install
   go install github.com/awhite/wtree@latest
   
   # Test install script
   curl -sSL https://raw.githubusercontent.com/awhite/wtree/main/install.sh | bash
   ```

4. **Announce release**:
   - Social media
   - Relevant communities
   - Update project status

## Version Strategy

WTree follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (v1.0.0 → v2.0.0): Breaking changes
- **MINOR** (v1.0.0 → v1.1.0): New features, backwards compatible
- **PATCH** (v1.0.0 → v1.0.1): Bug fixes, backwards compatible

### Pre-release versions:
- **Alpha**: v1.0.0-alpha.1 (early development)
- **Beta**: v1.0.0-beta.1 (feature complete, testing)
- **RC**: v1.0.0-rc.1 (release candidate)

## Package Manager Updates

### Homebrew
- Automatically updated by GoReleaser
- Formula location: `awhite/homebrew-tap`
- Users install: `brew install awhite/tap/wtree`

### Scoop (Windows)
- Automatically updated by GoReleaser
- Bucket location: `awhite/scoop-bucket`
- Users install: `scoop install wtree`

### Winget (Windows)
- Automatically updated by GoReleaser
- Creates PR to Microsoft's winget-pkgs repo
- Users install: `winget install awhite.wtree`

## Troubleshooting

### Common Issues

1. **GoReleaser fails**:
   ```bash
   # Check config
   goreleaser check
   
   # Test build locally
   goreleaser build --snapshot --clean
   ```

2. **Homebrew formula fails**:
   - Check the `homebrew-tap` repository
   - Verify GitHub token permissions
   - Test formula locally

3. **Binary won't run**:
   - Check build flags in `.goreleaser.yml`
   - Verify Go version compatibility
   - Test on target platforms

### Manual Steps

If automation fails, you can:

1. **Create release manually**:
   ```bash
   make build-all
   # Upload binaries to GitHub Release
   ```

2. **Update Homebrew manually**:
   - Fork `homebrew-core` or create `homebrew-tap`
   - Update formula with new URL and SHA256
   - Submit PR

3. **Generate checksums**:
   ```bash
   cd dist/
   sha256sum * > checksums.txt
   ```

## Release Checklist

- [ ] CHANGELOG.md updated
- [ ] Tests passing (`make test`)
- [ ] Version tagged
- [ ] GoReleaser config valid
- [ ] GitHub release created
- [ ] Binaries uploaded
- [ ] Homebrew formula updated
- [ ] Installation methods tested
- [ ] Documentation updated
- [ ] Release announced

## Security

- Never commit GitHub tokens
- Use GitHub Actions secrets for automation
- Verify release signatures
- Test binaries on clean systems