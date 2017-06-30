package handlers_test

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	"dns/server/handlers"
	"github.com/miekg/dns"
)

var _ = Describe("ExchangerFactory", func() {
	It("Returns a new Exchanger", func() {
		net := fmt.Sprintf("net-%d", rand.Int())
		timeout := time.Duration(rand.Int())

		exchangerFactory := handlers.NewExchangerFactory(timeout)
		exchanger := exchangerFactory(net)

		Expect(exchanger).To(BeAssignableToTypeOf(&dns.Client{}))

		client := exchanger.(*dns.Client)
		Expect(client.Net).To(Equal(net))
		Expect(client.Timeout).To(Equal(timeout))
	})
})
