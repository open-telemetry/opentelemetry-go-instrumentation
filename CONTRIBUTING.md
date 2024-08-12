# Contributing to opentelemetry-go-instrumentation

The Go Instrumnentation special interest group (SIG) meets regularly. See the
OpenTelemetry
[community](https://github.com/open-telemetry/community)
repo for information on this and other language SIGs.

See the [public meeting
notes](https://docs.google.com/document/d/1P6am_r_cxCX1HcpDQlznrTrTOvwN2whshL0f58lXSWI/edit)
for a summary description of past meetings. To request edit access,
join the meeting or get in touch on
[Slack](https://cloud-native.slack.com/archives/C03S01YSAS0).

## Development

### Compiling the project

Building requires Linux with kernel 4.4 or higher.
In addition, use the latest versions of the following components:

- `clang`
- `gcc`
- `go`
- `libbpf-dev`
- `llvm`
- `make`

Build the Go Automatic Instrumnentation binary by running:
`make build`

Alternatively, you can create a Linux Docker container:
`make docker-build`.

### Issues

Questions, bug reports, and feature requests can all be submitted as [issues](https://github.com/open-telemetry/opentelemetry-go-instrumentation/issues/new) to this repository.

## Pull Requests

### How to Send Pull Requests

Everyone is welcome to contribute code to `opentelemetry-go-instrumentation` via
GitHub pull requests (PRs).

To create a new PR, fork the project in GitHub and clone the upstream
repo:

```sh
go get -d go.opentelemetry.io/auto
```

(This may print some warning about "build constraints exclude all Go
files", just ignore it.)

This will put the project in `${GOPATH}/src/go.opentelemetry.io/auto`. You
can alternatively use `git` directly with:

```sh
git clone https://github.com/open-telemetry/opentelemetry-go-instrumentation
```

(Note that `git clone` is *not* using the `go.opentelemetry.io/auto` name -
that name is a kind of a redirector to GitHub that `go get` can
understand, but `git` does not.)

This would put the project in the `opentelemetry-go-instrumentation` directory in
current working directory.

Enter the newly created directory and add your fork as a new remote:

```sh
git remote add <YOUR_FORK> git@github.com:<YOUR_GITHUB_USERNAME>/opentelemetry-go-instrumentation
```

Check out a new branch, make modifications, run linters and tests, update
`CHANGELOG.md`, and push the branch to your fork:

```sh
git checkout -b <YOUR_BRANCH_NAME>
# edit files
# update changelog
make precommit
git add -p
git commit
git push <YOUR_FORK> <YOUR_BRANCH_NAME>
```

Additionally, there is a `codespell` target that checks for common
typos in the code. It is not run by default, but you can run it
manually with `make codespell`. It will set up a virtual environment
in `venv` and install `codespell` there.

Open a pull request against the main `opentelemetry-go-instrumentation` repo. Be sure to add the pull
request ID to the entry you added to `CHANGELOG.md`.

### How to Receive Comments

* If the PR is not ready for review, please put `[WIP]` in the title,
  tag it as `work-in-progress`, or mark it as
  [`draft`](https://github.blog/2019-02-14-introducing-draft-pull-requests/).
* Make sure CLA is signed and CI is clear.

### How to Get PRs Merged

A PR is considered **ready to merge** when:

* It has received at least one qualified approval[^1].

  For complex or sensitive PRs maintainers may require more than one qualified
  approval.

* All feedback has been addressed.
  * All PR comments and suggestions are resolved.
  * All GitHub Pull Request reviews with a status of "Request changes" have
    been addressed. Another review by the objecting reviewer with a different
    status can be submitted to clear the original review, or the review can be
    dismissed by a [Maintainer] when the issues from the original review have
    been addressed.
  * Any comments or reviews that cannot be resolved between the PR author and
    reviewers can be submitted to the community [Approver]s and [Maintainer]s
    during the weekly SIG meeting. If consensus is reached among the
    [Approver]s and [Maintainer]s during the SIG meeting the objections to the
    PR may be dismissed or resolved or the PR closed by a [Maintainer].
  * Any substantive changes to the PR require existing Approval reviews be
    cleared unless the approver explicitly states that their approval persists
    across changes. This includes changes resulting from other feedback.
    [Approver]s and [Maintainer]s can help in clearing reviews and they should
    be consulted if there are any questions.

* The PR branch is up to date with the base branch it is merging into.
  * To ensure this does not block the PR, it should be configured to allow
    maintainers to update it.

* All required GitHub workflows have succeeded.
* Urgent fix can take exception as long as it has been actively communicated
  among [Maintainer]s.

Any [Maintainer] can merge the PR once the above criteria have been met.

[^1]: A qualified approval is a GitHub Pull Request review with "Approve"
  status from an OpenTelemetry Go [Approver] or [Maintainer].

## Appovers and Maintainers

### Approvers

- [Aaron Clawson](https://github.com/MadVikingGod), LightStep
- [Dinesh Gurumurthy](https://github.com/dineshg13), DataDog
- [Mike Dame](https://github.com/damemi), Google
- [Robert Pająk](https://github.com/pellared), Splunk
- [Ron Federman](https://github.com/RonFed), Odigos

### Maintainers

- [Eden Federman](https://github.com/edeNFed), Odigos
- [Przemyslaw Delewski](https://github.com/pdelewski), SumoLogic
- [Tyler Yahn](https://github.com/MrAlias), Splunk

### Emeritus

- [Mike Goldsmith](https://github.com/MikeGoldsmith), Honeycomb

### Become an Approver or a Maintainer

See the [community membership document in OpenTelemetry community
repo](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md).

[Approver]: #approvers
[Maintainer]: #maintainers
