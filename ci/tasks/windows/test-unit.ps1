trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "bosh-dns-release"
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;C:/var/vcap/bosh/bin;" + $env:PATH

cd $env:GOPATH

powershell.exe scripts/install-go.ps1

cd $env:GOPATH/src/bosh-dns

go.exe install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo
ginkgo.exe -p -r -race -keepGoing -randomizeAllSpecs -randomizeSuites dns healthcheck
if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
