trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "bosh-dns-release"
$env:PATH = $env:GOPATH + "/bin;" + $env:PATH

cd $env:GOPATH
cd $env:GOPATH/src/bosh-dns

go.exe run github.com/onsi/ginkgo/v2/ginkgo -p -r -race -keepGoing -randomizeAllSpecs -randomizeSuites dns healthcheck

if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}

go.exe run github.com/onsi/ginkgo/v2/ginkgo -r -race -keepGoing -randomizeAllSpecs -randomizeSuites integration-tests

if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
