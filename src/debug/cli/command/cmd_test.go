package command_test

import (
	. "debug/cli/command"
	"net/http"

	uifakes "github.com/cloudfoundry/bosh-cli/ui/fakes"
	boshtbl "github.com/cloudfoundry/bosh-cli/ui/table"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("InstancesCmd", func() {
	var (
		api    string
		server *ghttp.Server
		ui     *uifakes.FakeUI
	)

	BeforeEach(func() {
		server = ghttp.NewServer()

		api = server.URL()
		ui = &uifakes.FakeUI{}
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when the DNS server responds with some instances", func() {
		Context("when no address arg is given to cli", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/instances"),
						ghttp.RespondWith(http.StatusOK, `
							{
								"id":           "3",
								"group":        "1",
								"network":      "default",
								"deployment":   "dep",
								"ip":           "1.2.3.4",
								"domain":       "bosh",
								"az":           "z1",
								"index":        "0",
								"health_state": "healthy"
							}
							{
								"id":           "4",
								"group":        "2",
								"network":      "private",
								"deployment":   "dep-2",
								"ip":           "4.5.6.7",
								"domain":       "bosh",
								"az":           "z2",
								"index":        "1",
								"health_state": "unhealthy"
							}
						`),
					),
				)
			})

			It("formats the contents like a table", func() {
				cmd := InstancesCmd{UI: ui, API: api}
				Expect(cmd.Execute(nil)).To(Succeed())

				Expect(ui.Table).To(Equal(boshtbl.Table{
					Title: "Known DNS instances",
					Header: []boshtbl.Header{
						boshtbl.NewHeader("ID"),
						boshtbl.NewHeader("Group"),
						boshtbl.NewHeader("Network"),
						boshtbl.NewHeader("Deployment"),
						boshtbl.NewHeader("IP"),
						boshtbl.NewHeader("Domain"),
						boshtbl.NewHeader("AZ"),
						boshtbl.NewHeader("Index"),
						boshtbl.NewHeader("HealthState"),
					},
					Rows: [][]boshtbl.Value{
						{
							boshtbl.NewValueString("3"),
							boshtbl.NewValueString("1"),
							boshtbl.NewValueString("default"),
							boshtbl.NewValueString("dep"),
							boshtbl.NewValueString("1.2.3.4"),
							boshtbl.NewValueString("bosh"),
							boshtbl.NewValueString("z1"),
							boshtbl.NewValueString("0"),
							boshtbl.NewValueString("healthy"),
						},
						{
							boshtbl.NewValueString("4"),
							boshtbl.NewValueString("2"),
							boshtbl.NewValueString("private"),
							boshtbl.NewValueString("dep-2"),
							boshtbl.NewValueString("4.5.6.7"),
							boshtbl.NewValueString("bosh"),
							boshtbl.NewValueString("z2"),
							boshtbl.NewValueString("1"),
							boshtbl.NewValueString("unhealthy"),
						},
					},
				}))
			})
		})

		Context("when address arg is given", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/instances", "address=my-query"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{}),
					),
				)
			})

			It("includes an address arg as a query param", func() {
				cmd := InstancesCmd{UI: ui, Args: InstancesArgs{Query: "my-query"}, API: api}
				Expect(cmd.Execute(nil)).To(Succeed())
				Expect(ui.Table).To(BeAssignableToTypeOf(boshtbl.Table{}))
			})
		})
	})

	Context("when the server does not respond 200", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/instances"),
					ghttp.RespondWith(http.StatusUnprocessableEntity, []byte{}),
				),
			)
		})

		It("raises an error", func() {
			cmd := InstancesCmd{}
			Expect(cmd.Execute(nil)).ToNot(Succeed())
		})
	})
})
