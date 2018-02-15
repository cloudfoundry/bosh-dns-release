package sample_test

import (
	"testing"

	datatests "bosh-dns/dns/internal/datatests/internal"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestDatatests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/internal/datatests/prodlike20180124a")
}

var server datatests.Server

var _ = BeforeSuite(func() {
	server = datatests.StartSampleServer()
})

var _ = AfterSuite(func() {
	datatests.StopSampleServer(server)
})

var _ = Describe("samples", func() {
	Describe("aliases", func() {
		datatests.DescribeMatchingAnyA(
			func() datatests.Server { return server },
			Entry("simple alias hostname lookup",
				"f59e2ea3-6359-4985-8329-0bab0d66ffc4.cell.service.cf.internal.",
				"10.0.29.4",
			),
			Entry("expanded alias hostname lookup",
				"auctioneer.service.cf.internal.",
				"10.0.1.39",
				"10.0.2.1",
			),
			Entry("expanded alias hostname lookup",
				"loggregator-trafficcontroller.service.cf.internal.",
				"10.0.2.24",
				"10.0.2.39",
				"10.0.1.29",
				"10.0.2.38",
				"10.0.1.35",
				"10.0.2.31",
				"10.0.1.37",
				"10.0.1.38",
				"10.0.2.19",
				"10.0.2.32",
				"10.0.1.26",
				"10.0.1.36",
				"10.0.1.17",
				"10.0.2.20",
			),
		)
	})

	Describe("long-form queries", func() {
		datatests.DescribeMatchingAnyA(
			func() datatests.Server { return server },
			Entry("instance uuid lookup",
				"f59e2ea3-6359-4985-8329-0bab0d66ffc4.ig-f1f0995e.net-3b01d1a9.dep-7b5223ac.bosh.",
				"10.0.29.4",
			),
			Entry("instance group lookup",
				"q-s3.ig-f1f0995e.net-3b01d1a9.dep-7b5223ac.bosh.",
				"10.0.29.4",
				"10.0.30.4",
			),
			Entry("filtered az instance group lookup",
				"q-a3.ig-f1f0995e.net-3b01d1a9.dep-7b5223ac.bosh.",
				"10.0.30.4",
			),
		)
	})

	Describe("group filtering", func() {
		datatests.DescribeMatchingAnyA(
			func() datatests.Server { return server },
			Entry(
				"specific instance of an instance group",
				"q-i3.q-g3.bosh.",
				"10.0.1.2",
			),
			Entry(
				"specific instance uuid",
				"f59e2ea3-6359-4985-8329-0bab0d66ffc4.q-g156.bosh.",
				"10.0.29.4",
			),
			Entry(
				"in a specific instance group",
				"q-s4.q-g3.bosh.",
				"10.0.1.1",
				"10.0.1.12",
				"10.0.1.18",
				"10.0.1.2",
				"10.0.1.27",
				"10.0.1.28",
				"10.0.1.31",
				"10.0.1.32",
				"10.0.1.33",
				"10.0.1.42",
				"10.0.1.43",
				"10.0.1.45",
				"10.0.1.46",
				"10.0.1.47",
				"10.0.1.55",
				"10.0.2.10",
				"10.0.2.12",
				"10.0.2.13",
				"10.0.2.27",
				"10.0.2.28",
				"10.0.2.3",
				"10.0.2.35",
				"10.0.2.40",
				"10.0.2.42",
				"10.0.2.43",
				"10.0.2.44",
				"10.0.2.45",
				"10.0.2.46",
				"10.0.2.47",
				"10.0.2.6",
			),
			Entry(
				"in a specific instance group and az",
				"q-a2s4.q-g3.bosh.",
				"10.0.1.1",
				"10.0.1.12",
				"10.0.1.18",
				"10.0.1.2",
				"10.0.1.27",
				"10.0.1.28",
				"10.0.1.31",
				"10.0.1.32",
				"10.0.1.33",
				"10.0.1.42",
				"10.0.1.43",
				"10.0.1.45",
				"10.0.1.46",
				"10.0.1.47",
				"10.0.1.55",
			),
		)
	})

	Describe("network filtering", func() {
		datatests.DescribeMatchingAnyA(
			func() datatests.Server { return server },
			Entry(
				"in a specific network",
				"q-n18.q-s4.bosh.",
				"10.0.22.31",
				"10.0.22.32",
				"10.0.22.33",
				"10.0.22.34",
				"10.0.22.35",
				"10.0.22.36",
				"10.0.22.37",
				"10.0.22.38",
				"10.0.22.39",
				"10.0.22.40",
				"10.0.22.41",
				"10.0.22.42",
				"10.0.22.43",
				"10.0.22.44",
				"10.0.22.45",
			),
			Entry(
				"in a specific network and az",
				"q-g6n14.q-s4.bosh.",
				"10.0.25.74",
			),
		)
	})

	Describe("az filtering", func() {
		datatests.DescribeMatchingAnyA(
			func() datatests.Server { return server },
			Entry(
				"in a specific az",
				"q-a4.q-s4.bosh.",
				"10.0.14.10",
				"10.0.14.11",
				"10.0.14.12",
				"10.0.14.14",
				"10.0.14.15",
				"10.0.14.16",
				"10.0.14.17",
				"10.0.14.18",
				"10.0.14.19",
				"10.0.14.20",
				"10.0.14.21",
				"10.0.14.22",
				"10.0.14.23",
				"10.0.14.24",
				"10.0.14.25",
				"10.0.14.26",
				"10.0.14.27",
				"10.0.14.28",
				"10.0.14.29",
				"10.0.14.30",
				"10.0.14.31",
				"10.0.14.32",
				"10.0.14.33",
				"10.0.14.34",
				"10.0.14.35",
				"10.0.14.36",
				"10.0.14.38",
				"10.0.14.39",
				"10.0.14.41",
				"10.0.14.42",
				"10.0.14.54",
				"10.0.19.10",
				"10.0.19.11",
				"10.0.19.12",
				"10.0.19.13",
				"10.0.19.14",
				"10.0.19.15",
				"10.0.19.16",
				"10.0.19.17",
				"10.0.19.18",
				"10.0.19.19",
				"10.0.19.20",
				"10.0.19.21",
				"10.0.19.22",
				"10.0.19.23",
				"10.0.19.24",
				"10.0.19.25",
				"10.0.19.26",
				"10.0.19.27",
				"10.0.19.28",
				"10.0.19.52",
				"10.0.19.53",
				"10.0.19.54",
				"10.0.19.55",
				"10.0.19.56",
			),
		)
	})
})
