trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "gopath"
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;C:/var/vcap/bosh/bin;" + $env:PATH

mkdir -path $env:GOPATH/src/github.com/cloudfoundry
mv ./dns-release $env:GOPATH/src/github.com/cloudfoundry/dns-release

cd $env:GOPATH/src/dns

if ((Get-Command "go.exe" -ErrorAction SilentlyContinue) -eq $null)
{
  Write-Host "Installing Go 1.7.5!"
  Invoke-WebRequest https://storage.googleapis.com/golang/go1.7.5.windows-amd64.msi -OutFile go.msi

  $p = Start-Process -FilePath "msiexec" -ArgumentList "/passive /norestart /i go.msi" -Wait -PassThru

  if($p.ExitCode -ne 0)
  {
    throw "Golang MSI installation process returned error code: $($p.ExitCode)"
  }
  Write-Host "Go is installed!"
}

go.exe install github.com/onsi/ginkgo/ginkgo
ginkgo.exe -r -race -keepGoing -randomizeAllSpecs -randomizeSuites
if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}

cd $env:GOPATH/src/healthcheck
ginkgo.exe -r -race -keepGoing -randomizeAllSpecs -randomizeSuites
if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
