# Release Process

## Release PR

Fetch the latest `main` branch from upstream and checkout a new branch for your prerelease PR:

```
git fetch origin
git checkout origin/main -b prerelease
```

Then, decide which module sets will be released and update their versions
in `versions.yaml`.  Commit this change to a new branch.

1. Run the `prerelease` make target. It creates a branch
    `prerelease_<module set>_<new tag>` that will contain all release changes.

    ```
    make prerelease MODSET=<module set>
    ```

    `<module set>` refers to the [set name in `versions.yaml`](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/f18c1b2e0702d8ac31699c5e923590d714d0c1dc/versions.yaml#L16)

2. Verify the changes.

    ```
    git diff ...prerelease_<module set>_<new tag>
    ```

    This should have changed the version for all modules to be `<new tag>`.
    If these changes look correct, merge them into your pre-release branch:

    ```go
    git merge prerelease_<module set>_<new tag>
    ```

3. Update the [Changelog](./CHANGELOG.md).
   - Make sure all relevant changes for this release are included and are in language that non-contributors to the project can understand.
       To verify this, you can look directly at the commits since the `<last tag>`.

       ```
       git --no-pager log --pretty=oneline "<last tag>..HEAD"
       ```

   - Move all the `Unreleased` changes into a new section following the title scheme (`[<new tag>] - <date of release>`).
   - Update all the appropriate links at the bottom.

4. Update [`CONTRIBUTING.md`](./CONTRIBUTING.md).
   - Ensure all supported instrumentation libraries have the correct versions listed.
     If the integration tests are still succeeding, the upper bound of the supported instrumentation version should be at least that tested version.

5. Update [`version.go`](internal/pkg/instrumentation/version.go) with the latest release version.

6. Push the changes to your branch and create a Pull Request on GitHub.
    Be sure to include the curated changes from the [Changelog](./CHANGELOG.md) in the description.

## Tag

Once the Pull Request with all the version changes has been approved and merged it is time to tag the merged commit.

***IMPORTANT***: It is critical you use the same tag that you used in the Pre-Release step!
Failure to do so will leave things in a broken state. As long as you do not
change `versions.yaml` between pre-release and this step, things should be fine.

***IMPORTANT***: [There is currently no way to remove an incorrectly tagged version of a Go module](https://github.com/golang/go/issues/34189).
It is critical you make sure the version you push upstream is correct.
[Failure to do so will lead to minor emergencies and tough to work around](https://github.com/open-telemetry/opentelemetry-go/issues/331).

1. For each module set that will be released, run the `add-tags` make target
    using the `<commit-hash>` of the commit on the main branch for the merged Pull Request.

    ```
    make add-tags MODSET=<module set> COMMIT=<commit hash>
    ```

    It should only be necessary to provide an explicit `COMMIT` value if the
    current `HEAD` of your working directory is not the correct commit.

2. Push tags to the upstream remote (not your fork: `github.com/open-telemetry/opentelemetry-go-instrumentation.git`).

    ```
    git push upstream <new tag>
    ```

    The release workflow builds and publishes the docker image using the new `<new tag>` as the version.

## Release

Finally create a Release for the new `<new tag>` on GitHub.
The release body should include all the release notes from the Changelog for this release.
