package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/api"
	"bosh-dns/dns/api/fakes"
	"bosh-dns/dns/server/record"
)

var _ = Describe("InstancesHandler", func() {
	var (
		fakeHealthStateGetter *fakes.FakeHealthStateGetter
		fakeRecordManager     *fakes.FakeRecordManager
		handler               *api.InstancesHandler

		w *httptest.ResponseRecorder
		r *http.Request
	)

	BeforeEach(func() {
		fakeHealthStateGetter = &fakes.FakeHealthStateGetter{}
		fakeRecordManager = &fakes.FakeRecordManager{}
		// URL path doesn't matter here since routing is handled elsewhere
		r = httptest.NewRequest("GET", "/?address=foo", nil)
		w = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		handler = api.NewInstancesHandler(fakeRecordManager, fakeHealthStateGetter)
	})

	It("returns status ok", func() {
		handler.ServeHTTP(w, r)
		response := w.Result()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
	})

	Context("when record set is empty", func() {

		It("returns an empty json array", func() {
			handler.ServeHTTP(w, r)
			response := w.Result()
			decoder := json.NewDecoder(response.Body)
			Expect(decoder.More()).To(BeFalse())
		})
	})

	Context("when record set has records", func() {
		BeforeEach(func() {
			fakeHealthStateGetter.HealthStateStringStub = func(ip string) string {
				switch ip {
				case "IP1":
					return "potato"
				case "IP2":
					return "lightbulb"
				default:
					Fail("ip is not recognized" + ip)
					return ""
				}
			}

		})

		It("filters records", func() {
			fakeRecordManager.ExpandAliasesReturns([]string{"mashed potatoes"})
			fakeRecordManager.ResolveRecordsReturns([]record.Record{
				{
					ID:            "ID1",
					NumID:         "NumId1",
					Group:         "Group1",
					GroupIDs:      []string{"GroupIDs1"},
					Network:       "Network1",
					NetworkID:     "NetworkID1",
					Deployment:    "Deployment1",
					IP:            "IP1",
					Domain:        "Domain1",
					AZ:            "AZ1",
					AZID:          "AZID1",
					InstanceIndex: "InstanceIndex1",
				},
			}, nil)
			r = httptest.NewRequest("GET", "/?address=potatoFilter.", nil)
			handler.ServeHTTP(w, r)
			response := w.Result()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			decoder := json.NewDecoder(response.Body)
			records := []api.InstanceRecord{}
			for decoder.More() {
				var record api.InstanceRecord
				err := decoder.Decode(&record)
				Expect(err).NotTo(HaveOccurred())
				records = append(records, record)
			}

			Expect(fakeRecordManager.ExpandAliasesCallCount()).To(Equal(1))
			Expect(fakeRecordManager.ExpandAliasesArgsForCall(0)).To(Equal("potatoFilter."))
			Expect(fakeRecordManager.ResolveRecordsCallCount()).To(Equal(1))
			Expect(fakeRecordManager.ResolveRecordsArgsForCall(0)).To(Equal([]string{"mashed potatoes"}))
			Expect(records).To(ConsistOf([]api.InstanceRecord{
				{
					ID:          "ID1",
					Group:       "Group1",
					Network:     "Network1",
					Deployment:  "Deployment1",
					IP:          "IP1",
					Domain:      "Domain1",
					AZ:          "AZ1",
					Index:       "InstanceIndex1",
					HealthState: "potato",
				},
			}))
		})

		It("returns all records in json", func() {
			r = httptest.NewRequest("GET", "/", nil)
			fakeRecordManager.AllRecordsReturns([]record.Record{
				{
					ID:            "ID1",
					NumID:         "NumId1",
					Group:         "Group1",
					GroupIDs:      []string{"GroupIDs1"},
					Network:       "Network1",
					NetworkID:     "NetworkID1",
					Deployment:    "Deployment1",
					IP:            "IP1",
					Domain:        "Domain1",
					AZ:            "AZ1",
					AZID:          "AZID1",
					InstanceIndex: "InstanceIndex1",
				},
				{
					ID:            "ID2",
					NumID:         "NumId2",
					Group:         "Group2",
					GroupIDs:      []string{"GroupIDs2"},
					Network:       "Network2",
					NetworkID:     "NetworkID2",
					Deployment:    "Deployment2",
					IP:            "IP2",
					Domain:        "Domain2",
					AZ:            "AZ2",
					AZID:          "AZID2",
					InstanceIndex: "InstanceIndex2",
				},
			})
			handler.ServeHTTP(w, r)
			response := w.Result()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			decoder := json.NewDecoder(response.Body)
			records := []api.InstanceRecord{}
			for decoder.More() {
				var record api.InstanceRecord
				err := decoder.Decode(&record)
				Expect(err).NotTo(HaveOccurred())
				records = append(records, record)
			}
			Expect(fakeRecordManager.AllRecordsCallCount()).To(Equal(1))
			Expect(fakeRecordManager.ExpandAliasesCallCount()).To(Equal(0))
			Expect(fakeRecordManager.ResolveRecordsCallCount()).To(Equal(0))
			Expect(records).To(ConsistOf([]api.InstanceRecord{
				{
					ID:          "ID1",
					Group:       "Group1",
					Network:     "Network1",
					Deployment:  "Deployment1",
					IP:          "IP1",
					Domain:      "Domain1",
					AZ:          "AZ1",
					Index:       "InstanceIndex1",
					HealthState: "potato",
				},
				{
					ID:          "ID2",
					Group:       "Group2",
					Network:     "Network2",
					Deployment:  "Deployment2",
					IP:          "IP2",
					Domain:      "Domain2",
					AZ:          "AZ2",
					Index:       "InstanceIndex2",
					HealthState: "lightbulb",
				},
			}))
		})

		It("handles unprocessable requests correctly", func() {
			fakeRecordManager.ResolveRecordsReturns([]record.Record{}, fmt.Errorf("yo!"))
			handler.ServeHTTP(w, r)
			resp := w.Result()
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("yo!"))
		})

		Context("when there is no trailing dot", func() {
			It("a dot is appended to the query param", func() {
				r = httptest.NewRequest("GET", "/?address=potatoFilter", nil)
				handler.ServeHTTP(w, r)
				response := w.Result()
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				Expect(fakeRecordManager.ExpandAliasesCallCount()).To(Equal(1))
				Expect(fakeRecordManager.ExpandAliasesArgsForCall(0)).To(Equal("potatoFilter."))
			})
		})
	})
})
