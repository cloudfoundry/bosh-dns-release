module debug

go 1.15

require (
	bosh-dns v0.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	github.com/cheggaaa/pb v1.0.29 // indirect
	github.com/cloudfoundry/bosh-cli v6.4.1+incompatible
	github.com/cloudfoundry/bosh-utils v0.0.0-20210225204315-79b702191f60
	github.com/cppforlife/go-semi-semantic v0.0.0-20160921010311-576b6af77ae4 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/kr/pty v1.1.8 // indirect
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/vito/go-interact v1.0.0 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
)

replace bosh-dns => ../bosh-dns
