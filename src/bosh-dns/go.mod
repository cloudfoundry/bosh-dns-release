module bosh-dns

go 1.16

require (
	code.cloudfoundry.org/clock v1.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20211123175040-23cc9f05b6b3
	code.cloudfoundry.org/workpool v0.0.0-20200131000409-2ac56b354115
	github.com/cloudfoundry/bosh-utils v0.0.298
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/coredns/coredns v1.8.7
	github.com/miekg/dns v1.1.45
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/prometheus/client_golang v1.12.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/sys v0.0.0-20220128215802-99c3d69c2c27
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/cloudfoundry/socks5-proxy v0.2.40 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	golang.org/x/tools v0.1.9 // indirect
	google.golang.org/genproto v0.0.0-20220126215142-9970aeb2e350 // indirect
	google.golang.org/grpc v1.44.0 // indirect
)
