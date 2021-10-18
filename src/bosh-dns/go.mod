module bosh-dns

go 1.15

require (
	code.cloudfoundry.org/clock v1.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20210615191307-5d92ef3894a7
	code.cloudfoundry.org/workpool v0.0.0-20200131000409-2ac56b354115
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cloudfoundry/bosh-utils v0.0.279
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/coredns/coredns v1.8.6
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/miekg/dns v1.1.43
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f
	golang.org/x/sys v0.0.0-20211015200801-69063c4bb744
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20211018162055-cf77aa76bad2 // indirect
	gopkg.in/yaml.v2 v2.4.0
)
