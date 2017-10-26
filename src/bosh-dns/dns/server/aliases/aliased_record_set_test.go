package aliases_test

import (
	"bosh-dns/dns/server/aliases/aliasesfakes"

	"errors"

	"bosh-dns/dns/server/aliases"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AliasedRecordSet", func() {
	var (
		aliasSet      *aliases.AliasedRecordSet
		fakeRecordSet *aliasesfakes.FakeRecordSet
		// fakeLogger *loggerfakes.FakeLogger
	)

	BeforeEach(func() {
		fakeRecordSet = &aliasesfakes.FakeRecordSet{}

		config := aliases.MustNewConfigFromMap(map[string][]string{
			"alias1":   {"a1_domain1", "a1_domain2"},
			"alias2":   {"a2_domain1"},
			"_.alias2": {"_.a2_domain1", "_.b2_domain1"},
		})

		var err error
		aliasSet = aliases.NewAliasedRecordSet(fakeRecordSet, config)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Subscribe", func() {
		It("delegates down the the underlying record set", func() {
			c := make(chan bool)
			defer close(c)
			fakeRecordSet.SubscribeReturns(c)
			aliasChannel := aliasSet.Subscribe()
			go func() { c <- true }()
			Eventually(aliasChannel).Should(Receive(BeTrue()))
			Expect(fakeRecordSet.SubscribeCallCount()).To(Equal(1))
		})
	})

	Describe("Domains", func() {
		It("delegates down the the underlying record set", func() {
			fakeRecordSet.DomainsReturns([]string{"a", "b"})
			Expect(aliasSet.Domains()).To(Equal([]string{"a", "b"}))
			Expect(fakeRecordSet.DomainsCallCount()).To(Equal(1))
		})
	})

	Describe("Resolve", func() {
		Context("when the host contains no aliased names", func() {
			It("resolves from underlying record set", func() {
				fakeRecordSet.ResolveReturns([]string{"1.1.1.1"}, nil)
				resolutions, err := aliasSet.Resolve("anything")

				Expect(err).ToNot(HaveOccurred())
				Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
				Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("anything"))
			})

			Context("and resolving fails", func() {
				It("resolves from underlying record set", func() {
					fakeRecordSet.ResolveReturns(nil, errors.New("could not resolve"))
					_, err := aliasSet.Resolve("anything")

					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("could not resolve"))
					Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("anything"))
				})
			})
		})

		Context("when the message contains a underscore style alias", func() {
			It("translates the question preserving the capture", func() {
				fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
					switch domain {
					case "5.a2_domain1.":
						return []string{"1.1.1.1"}, nil
					case "5.b2_domain1.":
						return []string{"2.2.2.2"}, nil
					default:
						return nil, errors.New("unknown host")
					}
				}
				resolutions, err := aliasSet.Resolve("5.alias2.")

				Expect(err).ToNot(HaveOccurred())
				Expect(resolutions).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
				Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("5.a2_domain1."))
				Expect(fakeRecordSet.ResolveArgsForCall(1)).To(Equal("5.b2_domain1."))
			})

			It("returns a non successful return code when a resoution fails", func() {
				fakeRecordSet.ResolveReturns(nil, errors.New("could not resolve"))
				_, err := aliasSet.Resolve("5.alias2.")

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("could not resolve"))
				Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("5.a2_domain1."))
				Expect(fakeRecordSet.ResolveArgsForCall(1)).To(Equal("5.b2_domain1."))
			})
		})

		Context("when resolving an aliased host", func() {
			It("resolves the alias", func() {
				fakeRecordSet.ResolveReturns([]string{"1.1.1.1"}, nil)
				resolutions, err := aliasSet.Resolve("alias2.")

				Expect(err).ToNot(HaveOccurred())
				Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
				Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("a2_domain1."))
			})

			Context("when alias resolves to multiple hosts", func() {
				It("resolves the alias to all underlying hosts", func() {
					fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
						switch domain {
						case "a1_domain1.":
							return []string{"1.1.1.1"}, nil
						case "a1_domain2.":
							return []string{"2.2.2.2"}, nil
						default:
							return nil, errors.New("unknown host")
						}
					}
					resolutions, err := aliasSet.Resolve("alias1.")

					Expect(err).ToNot(HaveOccurred())
					Expect(resolutions).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
					Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("a1_domain1."))
					Expect(fakeRecordSet.ResolveArgsForCall(1)).To(Equal("a1_domain2."))
				})

				Context("and a subset of the resolutions fails", func() {
					It("returns the ones that succeeded", func() {
						fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
							switch domain {
							case "a1_domain1.":
								return []string{"1.1.1.1"}, nil
							default:
								return nil, errors.New("unknown host")
							}
						}
						resolutions, err := aliasSet.Resolve("alias1.")

						Expect(err).ToNot(HaveOccurred())
						Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
						Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("a1_domain1."))
						Expect(fakeRecordSet.ResolveArgsForCall(1)).To(Equal("a1_domain2."))
					})
				})

				Context("and all of the resolutions fails", func() {
					It("returns an error", func() {
						fakeRecordSet.ResolveReturns(nil, errors.New("could not resolve"))
						_, err := aliasSet.Resolve("alias1.")

						Expect(err).To(HaveOccurred())
						Expect(err).To(MatchError("could not resolve"))
						Expect(fakeRecordSet.ResolveArgsForCall(0)).To(Equal("a1_domain1."))
						Expect(fakeRecordSet.ResolveArgsForCall(1)).To(Equal("a1_domain2."))
					})
				})
			})
		})
	})
})
