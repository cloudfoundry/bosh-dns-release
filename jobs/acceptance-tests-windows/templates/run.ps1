$env:GOROOT="C:\var\vcap\packages\golang-windows\go"
$env:GOPATH="C:\var\vcap\packages\acceptance-tests-windows"
$env:PATH="${env:GOPATH}\bin;${env:GOROOT}\bin;${env:PATH}"

go.exe install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo

Push-Location "C:\var\vcap\packages\acceptance-tests-windows\src\bosh-dns\acceptance_tests\windows"

ginkgo -randomizeAllSpecs -randomizeSuites -race <%= p('suites') %>

Pop-Location

Exit 0
