trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "bosh-dns-release"
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;C:/var/vcap/bosh/bin;" + $env:PATH

cd $env:GOPATH

powershell.exe scripts/install-go.ps1

$env:GIT_SHA = Get-Content ".git\HEAD" -Raw

cd $env:GOPATH/src/bosh-dns

go.exe run github.com/onsi/ginkgo/ginkgo -r -race -keepGoing -randomizeAllSpecs -randomizeSuites performance_tests

if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
