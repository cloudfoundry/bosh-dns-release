package api_test

import (
	"bosh-dns/dns/api"
	"bosh-dns/dns/api/apifakes"
	healthapi "bosh-dns/healthcheck/api"
	"bosh-dns/healthconfig"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalGroupsHandler", func() {
	var (
		fakeHealthChecker *apifakes.FakeHealthChecker
		jobs              []healthconfig.Job
		handler           *api.LocalGroupsHandler

		w *httptest.ResponseRecorder
		r *http.Request
	)

	BeforeEach(func() {
		fakeHealthChecker = &apifakes.FakeHealthChecker{}
		r = httptest.NewRequest("GET", "/", nil)
		w = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		handler = api.NewLocalGroupsHandler(jobs, fakeHealthChecker)
	})

	It("returns status ok", func() {
		handler.ServeHTTP(w, r)
		response := w.Result()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
	})

	Context("when jobs are defined", func() {
		var (
			job1 healthconfig.Job
			job2 healthconfig.Job
		)

		BeforeEach(func() {
			job1group1 := healthconfig.LinkMetadata{
				JobName: "job1",
				Type:    "entree",
				Name:    "chow-mein",
				Group:   "1",
			}
			job1group2 := healthconfig.LinkMetadata{
				JobName: "job1",
				Type:    "dessert",
				Name:    "mooncake",
				Group:   "2",
			}
			job1.Groups = []healthconfig.LinkMetadata{job1group1, job1group2}

			job2group1 := healthconfig.LinkMetadata{
				JobName: "job2",
				Type:    "appetizer",
				Name:    "pancakes",
				Group:   "3",
			}
			job2.Groups = []healthconfig.LinkMetadata{job2group1}

			jobs = []healthconfig.Job{job1, job2}
		})

		Context("when health states are defined", func() {
			BeforeEach(func() {
				healthStates := healthapi.HealthResult{
					State: "failing",
					GroupState: map[string]healthapi.HealthStatus{
						"1": "running",
						"2": "failing",
						"3": "running",
					},
				}
				fakeHealthChecker.GetStatusReturns(healthStates)
			})

			It("encodes a json stream with the group health state", func() {
				handler.ServeHTTP(w, r)
				response := w.Result()

				groups := []api.Group{}
				dec := json.NewDecoder(response.Body)

				var instanceState api.Group
				Expect(dec.Decode(&instanceState)).To(Succeed())
				Expect(instanceState).To(Equal(api.Group{
					HealthState: "failing",
				}))

				for dec.More() {
					var group api.Group
					Expect(dec.Decode(&group)).To(Succeed())
					groups = append(groups, group)
				}

				Expect(groups).To(ConsistOf([]api.Group{
					{
						JobName:     "job1",
						LinkType:    "entree",
						LinkName:    "chow-mein",
						GroupID:     "1",
						HealthState: "running",
					},
					{
						JobName:     "job1",
						LinkType:    "dessert",
						LinkName:    "mooncake",
						GroupID:     "2",
						HealthState: "failing",
					},
					{
						JobName:     "job2",
						LinkType:    "appetizer",
						LinkName:    "pancakes",
						GroupID:     "3",
						HealthState: "running",
					},
				}))
			})
		})

		Context("when health states are NOT defined", func() {
			It("encodes a json stream with the group health state", func() {
				handler.ServeHTTP(w, r)
				response := w.Result()

				groups := []api.Group{}
				dec := json.NewDecoder(response.Body)

				var instanceState api.Group
				Expect(dec.Decode(&instanceState)).To(Succeed())
				Expect(instanceState).To(Equal(api.Group{
					HealthState: "",
				}))

				for dec.More() {
					var group api.Group
					Expect(dec.Decode(&group)).To(Succeed())
					groups = append(groups, group)
				}

				Expect(groups).To(ConsistOf([]api.Group{
					{
						JobName:     "job1",
						LinkType:    "entree",
						LinkName:    "chow-mein",
						GroupID:     "1",
						HealthState: "",
					},
					{
						JobName:     "job1",
						LinkType:    "dessert",
						LinkName:    "mooncake",
						GroupID:     "2",
						HealthState: "",
					},
					{
						JobName:     "job2",
						LinkType:    "appetizer",
						LinkName:    "pancakes",
						GroupID:     "3",
						HealthState: "",
					},
				}))
			})
		})
	})
})
