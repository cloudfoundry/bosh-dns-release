package command_test

import (
	"debug/cli/command"
	"net/http"

	uifakes "github.com/cloudfoundry/bosh-cli/ui/fakes"
	boshtbl "github.com/cloudfoundry/bosh-cli/ui/table"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("GroupsCmd", func() {
	var (
		api    string
		server *ghttp.Server
		ui     *uifakes.FakeUI
		cmd    command.GroupsCmd
	)

	BeforeEach(func() {
		server = newFakeAPIServer()

		api = server.URL()
		ui = &uifakes.FakeUI{}
		cmd = command.GroupsCmd{
			UI:                 ui,
			API:                api,
			TLSCACertPath:      "../../../bosh-dns/dns/api/assets/test_certs/test_ca.pem",
			TLSCertificatePath: "../../../bosh-dns/dns/api/assets/test_certs/test_wrong_cn_client.pem",
			TLSPrivateKeyPath:  "../../../bosh-dns/dns/api/assets/test_certs/test_client.key",
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when the DNS server responds with some groups", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/groups"),
					ghttp.RespondWith(http.StatusOK, `
							{
								"job_name": null,
								"link_name": null,
								"link_type": null,
								"group_id": 3,
								"health_state": "running"
							}
							{
								"job_name": "zookeeper",
								"link_name": "conn",
								"link_type": "zookeeper",
								"group_id": 4,
								"health_state": "failing"
							}
							{
								"job_name": "zookeeper",
								"link_name": "peers",
								"link_type": "zookeeper_peers",
								"group_id": 5,
								"health_state": "running"
							}
						`),
				),
			)
		})

		It("formats the contents like a table", func() {
			Expect(cmd.Execute(nil)).To(Succeed())

			Expect(ui.Table).To(Equal(boshtbl.Table{
				FillFirstColumn: true,
				Header: []boshtbl.Header{
					boshtbl.NewHeader("JobName"),
					boshtbl.NewHeader("LinkName"),
					boshtbl.NewHeader("LinkType"),
					boshtbl.NewHeader("GroupID"),
					boshtbl.NewHeader("HealthState"),
				},
				Rows: [][]boshtbl.Value{
					{
						boshtbl.NewValueString(""),
						boshtbl.NewValueString(""),
						boshtbl.NewValueString(""),
						boshtbl.NewValueInt(3),
						boshtbl.NewValueString("running"),
					},
					{
						boshtbl.NewValueString("zookeeper"),
						boshtbl.NewValueString("conn"),
						boshtbl.NewValueString("zookeeper"),
						boshtbl.NewValueInt(4),
						boshtbl.NewValueString("failing"),
					},
					{
						boshtbl.NewValueString("zookeeper"),
						boshtbl.NewValueString("peers"),
						boshtbl.NewValueString("zookeeper_peers"),
						boshtbl.NewValueInt(5),
						boshtbl.NewValueString("running"),
					},
				},
			}))
		})
	})

	Context("when the server does not respond 200", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/groups"),
					ghttp.RespondWith(http.StatusUnprocessableEntity, []byte{}),
				),
			)
		})

		It("raises an error", func() {
			Expect(cmd.Execute(nil)).ToNot(Succeed())
		})
	})
})
