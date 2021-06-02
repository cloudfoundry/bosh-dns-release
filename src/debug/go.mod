module debug

go 1.15

require (
	bosh-dns v0.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	github.com/cheggaaa/pb v1.0.29 // indirect
	github.com/cloudfoundry/bosh-cli v6.4.1+incompatible
	github.com/cloudfoundry/bosh-utils v0.0.258
	github.com/cppforlife/go-semi-semantic v0.0.0-20160921010311-576b6af77ae4 // indirect
	github.com/fatih/color v1.12.0 // indirect
	github.com/jessevdk/go-flags v1.5.0
	github.com/kr/pty v1.1.8 // indirect
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/vito/go-interact v1.0.0 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
)

replace bosh-dns => ../bosh-dns
