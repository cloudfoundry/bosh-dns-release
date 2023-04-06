package handlers_test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/handlers"
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
