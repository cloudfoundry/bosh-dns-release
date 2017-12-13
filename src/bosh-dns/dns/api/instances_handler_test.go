package api_test

import (
	"bosh-dns/dns/api"
	"bosh-dns/dns/server/records"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstancesHandler", func() {
	var (
		handler   *api.InstancesHandler
		recordSet *records.RecordSet

		w *httptest.ResponseRecorder
		r *http.Request
	)

	BeforeEach(func() {
		// URL path doesn't matter here since routing is handled elsewhere
		r = httptest.NewRequest("GET", "/", nil)
		w = httptest.NewRecorder()

		recordSet = &records.RecordSet{}
	})

	JustBeforeEach(func() {
		handler = api.NewInstancesHandler(recordSet)
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
			recordSet.Records = []records.Record{
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
			}
		})

		It("returns an all records in json", func() {
			handler.ServeHTTP(w, r)
			response := w.Result()
			decoder := json.NewDecoder(response.Body)
			records := []api.Record{}
			for decoder.More() {
				var record api.Record
				err := decoder.Decode(&record)
				Expect(err).NotTo(HaveOccurred())
				records = append(records, record)
			}
			Expect(records).To(ConsistOf([]api.Record{
				{
					ID:         "ID1",
					Group:      "Group1",
					Network:    "Network1",
					Deployment: "Deployment1",
					IP:         "IP1",
					Domain:     "Domain1",
					AZ:         "AZ1",
					Index:      "InstanceIndex1",
					Healthy:    true,
				},
				{
					ID:         "ID2",
					Group:      "Group2",
					Network:    "Network2",
					Deployment: "Deployment2",
					IP:         "IP2",
					Domain:     "Domain2",
					AZ:         "AZ2",
					Index:      "InstanceIndex2",
					Healthy:    true,
				},
			}))
		})
	})
})
