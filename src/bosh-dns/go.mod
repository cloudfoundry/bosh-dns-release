module bosh-dns

go 1.15

require (
	code.cloudfoundry.org/clock v1.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	code.cloudfoundry.org/workpool v0.0.0-20200131000409-2ac56b354115
	github.com/cloudfoundry/bosh-utils v0.0.0-20210327100132-d1d58eaab188
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/coredns/coredns v1.8.3
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/miekg/dns v1.1.41
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.20.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	golang.org/x/net v0.0.0-20210331212208-0fccb6fa2b5c
	golang.org/x/sys v0.0.0-20210402192133-700132347e07
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/genproto v0.0.0-20210402141018-6c239bbf2bb1 // indirect
	gopkg.in/yaml.v2 v2.4.0
)
