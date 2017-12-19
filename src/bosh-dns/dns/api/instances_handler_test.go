package api_test

import (
	"bosh-dns/dns/api"
	"bosh-dns/dns/api/apifakes"
	"bosh-dns/dns/server/records"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstancesHandler", func() {
	var (
		fakeHealthStateGetter *apifakes.FakeHealthStateGetter
		fakeRecordManager     *apifakes.FakeRecordManager
		handler               *api.InstancesHandler
		recordSet             *records.RecordSet

		w *httptest.ResponseRecorder
		r *http.Request
	)

	BeforeEach(func() {
		fakeHealthStateGetter = &apifakes.FakeHealthStateGetter{}
		fakeRecordManager = &apifakes.FakeRecordManager{}
		// URL path doesn't matter here since routing is handled elsewhere
		r = httptest.NewRequest("GET", "/?address=foo", nil)
		w = httptest.NewRecorder()

		recordSet = &records.RecordSet{}
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
		BeforeEach(func() {
			recordSet.Records = []records.Record{}
		})

		It("returns an empty json array", func() {
			handler.ServeHTTP(w, r)
			response := w.Result()
			decoder := json.NewDecoder(response.Body)
			Expect(decoder.More()).To(BeFalse())
		})
	})

	Context("when record set has records", func() {
		BeforeEach(func() {
			fakeHealthStateGetter.HealthStateStub = func(ip string) string {
				switch ip {
				case "IP1":
					return "potato"
				case "IP2":
					return "lightbulb"
				default:
					panic("ip is not recognized" + ip)
				}
			}

		})

		It("filters records", func() {
			fakeRecordManager.ExpandAliasesReturns([]string{"mashed potatoes"})
			fakeRecordManager.FilterReturns([]records.Record{
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
			r = httptest.NewRequest("GET", "/?address=potatoFilter", nil)
			handler.ServeHTTP(w, r)
			response := w.Result()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			decoder := json.NewDecoder(response.Body)
			records := []api.Record{}
			for decoder.More() {
				var record api.Record
				err := decoder.Decode(&record)
				Expect(err).NotTo(HaveOccurred())
				records = append(records, record)
			}

			Expect(fakeRecordManager.ExpandAliasesCallCount()).To(Equal(1))
			Expect(fakeRecordManager.ExpandAliasesArgsForCall(0)).To(Equal("potatoFilter"))
			Expect(fakeRecordManager.FilterCallCount()).To(Equal(1))
			Expect(fakeRecordManager.FilterArgsForCall(0)).To(Equal([]string{"mashed potatoes"}))
			Expect(records).To(ConsistOf([]api.Record{
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
			fakeRecordManager.AllRecordsReturns(&[]records.Record{
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
			records := []api.Record{}
			for decoder.More() {
				var record api.Record
				err := decoder.Decode(&record)
				Expect(err).NotTo(HaveOccurred())
				records = append(records, record)
			}
			Expect(fakeRecordManager.AllRecordsCallCount()).To(Equal(1))
			Expect(fakeRecordManager.ExpandAliasesCallCount()).To(Equal(0))
			Expect(fakeRecordManager.FilterCallCount()).To(Equal(0))
			Expect(records).To(ConsistOf([]api.Record{
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
			fakeRecordManager.FilterReturns([]records.Record{}, fmt.Errorf("yo!"))
			handler.ServeHTTP(w, r)
			resp := w.Result()
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("yo!"))
		})
	})
})
