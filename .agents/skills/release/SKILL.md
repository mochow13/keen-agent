---
name: release
description: Release a new version of Keen Agent.
---

# Release Skill

Target version (optional): $ARGUMENTS

If no target version is provided, inspect the latest release tag and commits since that tag before making any changes. Suggest a semantic-version bump to the user and wait for confirmation:

- **major** for intentional breaking changes.
- **minor** for backward-compatible features.
- **patch** for fixes, documentation, dependencies, and internal changes.

Use `git tag --sort=-version:refname` to identify the latest version tag and `git log <latest-tag>..HEAD --oneline` to review subsequent commits. If no release tags exist, treat the release as the initial `v0.1.0` release and inspect the complete history. Also consider unreleased changelog entries. State the latest tag (or that no tag exists), the commits considered, the recommended version, and a concise rationale. If there are no commits since the latest tag, say that no release is recommended.

Once a version is supplied or confirmed, use it as the target version for the remaining steps.

## Steps

1. **Verify the release state.** Ensure the working tree is clean or explicitly identify any unrelated changes. Check whether the target tag already exists locally or on `origin`; never overwrite or move an existing tag.

2. **Bump the version.** Update the version string, without the `v` prefix, in `cmd/main.go`.

3. **Update `CHANGELOG.md`.**
   - Move all entries under `[Unreleased]` into a new `[X.Y.Z] - YYYY-MM-DD` section below it.
   - Add an empty `[Unreleased]` section at the top if one does not exist.
   - Check commit history for changes missing from the unreleased section.
   - Add or update the release link for `vX.Y.Z`.

4. **Validate the release.**
   ```bash
   go mod tidy
   go test -race ./...
   ```

5. **Commit the release preparation.** Stage only the release files and commit them:
   - `cmd/main.go`
   - `CHANGELOG.md`

6. **Push the release commit, then tag it.**
   ```bash
   git push origin main
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
   Pushing the tag triggers `.github/workflows/release.yml`.

7. **Watch the release workflow to completion.**
   ```bash
   gh run watch $(gh run list --workflow release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
   ```
   The `release` job runs race tests and GoReleaser, which creates the GitHub Release and its artifacts. If the run fails, inspect its logs:
   ```bash
   gh run view <run-id> --log-failed
   ```

8. **Confirm the release.** Verify the job succeeded and report the tag, release commit SHA, and GitHub Release URL:
   ```bash
   gh release view vX.Y.Z --json url
   ```
