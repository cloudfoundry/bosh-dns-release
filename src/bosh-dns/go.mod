module bosh-dns

go 1.15

require (
	code.cloudfoundry.org/clock v1.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	code.cloudfoundry.org/workpool v0.0.0-20200131000409-2ac56b354115
	github.com/cloudfoundry/bosh-utils v0.0.256
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/cloudfoundry/socks5-proxy v0.2.6 // indirect
	github.com/coredns/caddy v1.1.1 // indirect
	github.com/coredns/coredns v1.8.3
	github.com/miekg/dns v1.1.42
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.16.2
	github.com/onsi/gomega v1.12.0
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.24.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	golang.org/x/net v0.0.0-20210510120150-4163338589ed
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015
	google.golang.org/genproto v0.0.0-20210513213006-bf773b8c8384 // indirect
	google.golang.org/grpc v1.37.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
)
