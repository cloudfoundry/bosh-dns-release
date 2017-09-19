# bosh-dns-release

* Documentation: [bosh.io/docs/dns](https://bosh.io/docs/dns.html)
* Slack: #bosh on <https://slack.cloudfoundry.org>
* Mailing list: [cf-bosh](https://lists.cloudfoundry.org/pipermail/cf-bosh)
* Roadmap: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/956238)

## Development

This repository is a `GOPATH`. The [`.envrc`](.envrc) file provides a setup that can be used with direnv. The underlying `bosh-dns` package uses [dep](https://github.com/golang/dep) to vendor its dependencies.

Be careful with `go get`. In this repository `go get` will end up putting artifacts in the `src` directory, which you probably don't want to commit. It's impractical to `.gitignore` every possible package root in there so we have to apply discipline.
