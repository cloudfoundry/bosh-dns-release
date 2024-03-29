﻿trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "bosh-dns-release"
$env:PATH = $env:GOPATH + "/bin;" + $env:PATH

cd $env:GOPATH
cd $env:GOPATH/src/bosh-dns

go.exe run github.com/onsi/ginkgo/v2/ginkgo -p -r -race --keep-going --randomize-all --randomize-suites dns healthcheck

if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}

go.exe run github.com/onsi/ginkgo/v2/ginkgo -r -race --keep-going --randomize-all --randomize-suites integration_tests

if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
