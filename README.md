# bosh-dns-release

* Documentation: [bosh.io/docs/dns](https://bosh.io/docs/dns.html)
* Slack: #bosh on <https://slack.cloudfoundry.org>
* Mailing list: [cf-bosh](https://lists.cloudfoundry.org/pipermail/cf-bosh)
* Roadmap: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/956238)

This release provides DNS for Bosh. It has replaced consul. 

## Usage
Download the lastest release off of [bosh.io/releases](https://bosh.io/releases/github.com/cloudfoundry/bosh-dns-release?all=1).
Reference the [bosh.io/docs/dns](https://bosh.io/docs/dns.html) documentation for usage instructions.

## Development

This repository is a `GOPATH`. The [`.envrc`](.envrc) file provides a setup that can be used with direnv. The underlying `bosh-dns` package uses [dep](https://github.com/golang/dep) to vendor its dependencies.

Be careful with `go get`. In this repository `go get` will end up putting artifacts in the `src` directory, which you probably don't want to commit. It's impractical to `.gitignore` every possible package root in there so we have to apply discipline.

To build a dev release, run a `bosh create-release` from this repo.

## Running the tests

Before the tests can be run, you need to execute the following commands:

### On macOS

```bash
sudo ifconfig lo0 alias 127.0.0.2 up
sudo ifconfig lo0 alias 127.0.0.3 up
sudo ifconfig lo0 alias 127.0.0.253 up
sudo ifconfig lo0 alias 127.0.0.254 up
sudo sysctl -w kern.ipc.somaxconn=1024  # default is 128
```

Then run the tests:

```bash
./scripts/test-unit
```

You could also use ginkgo to run a specific test suite

```bash
ginkgo -v --procs=1 src/bosh-dns/dns/
```