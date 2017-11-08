. C:\var\vcap\packages\golang-1.8-windows\bosh\runtime.ps1

New-Item -Path ${env:GOPATH}\src -ItemType directory
Copy-Item -recurse -force  "C:\var\vcap\packages\acceptance-tests-windows\src\bosh-dns" "$env:GOPATH\src\bosh-dns"

go.exe install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo

Push-Location "${env:GOPATH}\src\bosh-dns\acceptance_tests\windows"

$env:OS_DNS_CACHE = "<%= p('properties_to_test.os_caching_enabled') %>"

ginkgo -randomizeAllSpecs -randomizeSuites -race <%= p('suites') %>

Pop-Location

Exit 0
