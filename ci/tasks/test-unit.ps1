trap {
  write-error $_
  exit 1
}

$env:GOPATH = Join-Path -Path $PWD "bosh-dns-release"
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;C:/var/vcap/bosh/bin;" + $env:PATH

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

cd $env:GOPATH/src/bosh-dns

go.exe install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo
ginkgo.exe -p -r -race -keepGoing -randomizeAllSpecs -randomizeSuites dns healthcheck
if ($LastExitCode -ne 0)
{
    Write-Error $_
    exit 1
}
