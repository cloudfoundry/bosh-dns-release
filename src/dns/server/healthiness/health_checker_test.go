package healthiness_test

import (
	"errors"
	"fmt"

	"dns/server/healthiness"

	httpclientfakes "github.com/cloudfoundry/bosh-utils/httpclient/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthChecker", func() {
	var (
		ip            string
		fakeClient    *httpclientfakes.FakeHTTPClient
		healthChecker healthiness.HealthChecker
	)

	BeforeEach(func() {
		fakeClient = &httpclientfakes.FakeHTTPClient{}
		healthChecker = healthiness.NewHealthChecker(fakeClient, 8081)
	})

	Describe("GetStatus", func() {
		Context("when healthy", func() {
			BeforeEach(func() {
				ip = "127.0.0.1"
			})

			It("returns true", func() {
				fakeClient.SetGetBehavior(`{"state":"running"}`, 200, nil)

				Expect(healthChecker.GetStatus(ip)).To(BeTrue())
				Expect(fakeClient.GetInputs).To(HaveLen(1))
				Expect(fakeClient.GetInputs[0].Endpoint).To(Equal(fmt.Sprintf("https://%s:8081/health", ip)))
			})
		})

		Context("when unhealthy", func() {
			BeforeEach(func() {
				ip = "127.0.0.2"
			})

			It("returns false", func() {
				fakeClient.SetGetBehavior(`{"state":"stopped"}`, 200, nil)

				Expect(healthChecker.GetStatus(ip)).To(BeFalse())
				Expect(fakeClient.GetInputs).To(HaveLen(1))
				Expect(fakeClient.GetInputs[0].Endpoint).To(Equal(fmt.Sprintf("https://%s:8081/health", ip)))
			})
		})

		Context("when unable to fetch status", func() {
			BeforeEach(func() {
				ip = "127.0.0.3"
			})

			It("returns false", func() {
				fakeClient.SetGetBehavior("", 0, errors.New("fake connect err"))

				Expect(healthChecker.GetStatus(ip)).To(BeFalse())
				Expect(fakeClient.GetInputs).To(HaveLen(1))
				Expect(fakeClient.GetInputs[0].Endpoint).To(Equal(fmt.Sprintf("https://%s:8081/health", ip)))
			})
		})

		Context("when status is invalid json", func() {
			BeforeEach(func() {
				ip = "127.0.0.3"
			})

			It("returns false", func() {
				fakeClient.SetGetBehavior("duck?", 0, nil)

				Expect(healthChecker.GetStatus(ip)).To(BeFalse())
				Expect(fakeClient.GetInputs).To(HaveLen(1))
				Expect(fakeClient.GetInputs[0].Endpoint).To(Equal(fmt.Sprintf("https://%s:8081/health", ip)))
			})
		})

		Context("when response is not 200 OK", func() {
			BeforeEach(func() {
				ip = "127.0.0.3"
			})

			It("returns false", func() {
				fakeClient.SetGetBehavior(`{"state":"running"}`, 400, nil)

				Expect(healthChecker.GetStatus(ip)).To(BeFalse())
				Expect(fakeClient.GetInputs).To(HaveLen(1))
				Expect(fakeClient.GetInputs[0].Endpoint).To(Equal(fmt.Sprintf("https://%s:8081/health", ip)))
			})
		})
	})
})
