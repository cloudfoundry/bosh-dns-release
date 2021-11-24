. C:\var\vcap\packages\golang-1.16-windows\bosh\runtime.ps1

$ErrorActionPreference = "Stop"

New-Item -Path ${env:GOPATH}\src -ItemType directory -Force
Remove-Item -recurse -force -erroraction ignore ${env:GOPATH}\src\bosh-dns
Copy-Item -recurse -force  "C:\var\vcap\packages\acceptance-tests-windows\src\bosh-dns" "$env:GOPATH\src\bosh-dns"

Push-Location "${env:GOPATH}\src\bosh-dns"

$env:LOCAL_IP_ADDRESS = "<%= spec.ip %>"
go.exe run github.com/onsi/ginkgo/ginkgo -randomizeAllSpecs -randomizeSuites -race acceptance_tests/windows/<%= p('suites') %>

Pop-Location

Exit $LASTEXITCODE
