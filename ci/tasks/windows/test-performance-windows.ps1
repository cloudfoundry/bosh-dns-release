trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "bosh-dns-release"
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;C:/var/vcap/bosh/bin;" + $env:PATH

function NeedsToInstallGo() {
    Write-Host "Checking if Go needs to be installed or updated..."
    if ((Get-Command 'go.exe' -ErrorAction SilentlyContinue) -eq $null) {
        Write-Host "Go.exe not found, Go will be installed"
        return $true
    }
    $version = "$(go.exe version)"
    if ($version -match 'go version go1\.[1-8]\.\d windows\/amd64') {
        Write-Host "Installed version of Go is not supported, Go will be updated"
        return $true
    }
    Write-Host "Found Go version '$version' installed on the system, skipping install"
    return $false
}

if (NeedsToInstallGo) {
    Write-Host "Installing Go 1.9.1"

    Invoke-WebRequest 'https://storage.googleapis.com/golang/go1.9.1.windows-amd64.msi' `
        -UseBasicParsing -OutFile go.msi

    $p = Start-Process -FilePath "msiexec" `
        -ArgumentList "/passive /norestart /i go.msi" `
        -Wait -PassThru
    if ($p.ExitCode -ne 0) {
        throw "Golang MSI installation process returned error code: $($p.ExitCode)"
    }

    Write-Host "Successfully installed go version: $(go version)"
}

$env:GIT_SHA = Get-Content "bosh-dns-release\.git\HEAD" -Raw

cd $env:GOPATH/src/bosh-dns

go.exe install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo

cd performance_tests

ginkgo.exe -r -race -keepGoing -randomizeAllSpecs -randomizeSuites .

if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
