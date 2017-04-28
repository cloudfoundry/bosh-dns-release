$env:GOROOT="C:\var\vcap\packages\golang-windows\go"
$env:GOPATH="C:\var\vcap\packages\acceptance-tests-windows"
$env:PATH="${env:GOPATH}\bin;${env:GOROOT}\bin;${env:PATH}"

go.exe install github.com/cloudfoundry/dns-release/src/vendor/github.com/onsi/ginkgo/ginkgo

Push-Location "C:\var\vcap\packages\acceptance-tests-windows\src\github.com\cloudfoundry\dns-release\src\acceptance_tests\windows"

ginkgo -r -randomizeAllSpecs -randomizeSuites -race .

Pop-Location

Exit 0
