# Contributing

Please help us build this project!

## Community meetings

Join our bi-weekly meetings if you would like to connect with us via zoom.
You can find them on the [OpenTelemetry calendar](https://calendar.google.com/calendar/embed?src=google.com_b79e3e90j7bbsa2n2p5an5lf60%40group.calendar.google.com&ctz=America%2FLos_Angeles).

## Slack

Comments and questions about the project can be posted in our [slack channel](https://cloud-native.slack.com/archives/C03S01YSAS0).

## Development

### Compiling the project

Linux users can build this repository by running:
`make build`

Windows/Mac users will need to compile this project inside a docker container by running:
`make docker-build IMG=otel-go-agent:v0.1`

### Issues

Questions, bug reports, and feature requests can all be submitted as [issues](https://github.com/open-telemetry/opentelemetry-go-instrumentation/issues/new) to this repository.

### Pull Requests

Development of this repository is done using the [forking model](https://docs.github.com/en/get-started/quickstart/fork-a-repo).

Once you have changes on a branch of your fork that you would like to proposed to this project [create a pull request (PR)](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request-from-a-fork).
If you are still working to finalize your PR, but would like to publish something publicly, create the PR as a draft.

Next, your PR needs to be reviewed and approved by the [project approvers](https://github.com/orgs/open-telemetry/teams/go-instrumentation-approvers).
It will be ready to merge when:

- It has received two approvals from project approvers (at different companies).
- All feedback has been addressed.
- All open comments should be resolved.

A [project maintainer](https://github.com/orgs/open-telemetry/teams/go-instrumentaiton-maintainers) can merge the PR once these conditions are satisfied.
It is up to project maintains to ensure enough time has been allowed for review of PRs.
