package records_test

import (
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/records"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNS encoding", func() {
	var (
		aliasEncoder     *records.AliasEncoder
		aliasDefinitions map[string][]records.AliasDefinition
	)

	BeforeEach(func() {
		aliasEncoder = &records.AliasEncoder{}
	})

	Context("when the aliases are derived in records file", func() {
		Context("with no criteria", func() {
			BeforeEach(func() {
				aliasDefinitions = map[string][]records.AliasDefinition{
					"custom-alias.": []records.AliasDefinition{
						{
							GroupID:    "1",
							RootDomain: "a2_domain1",
						},
					},
				}
			})

			It("expands to a simple alias", func() {
				encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
					[]record.Record{{
						GroupIDs: []string{"1"},
						Domain:   "a2_domain1.",
					}},
					aliasDefinitions,
				)
				Expect(encodedAliases).To(
					Equal(
						map[string][]string{"custom-alias.": {"q-s0.q-g1.a2_domain1."}},
					),
				)
			})
		})

		Context("with health_filter", func() {
			Context("when smart", func() {
				BeforeEach(func() {
					aliasDefinitions = map[string][]records.AliasDefinition{
						"custom-alias.": []records.AliasDefinition{
							{
								GroupID:      "1",
								RootDomain:   "a2_domain1",
								HealthFilter: "smart",
							},
						},
					}
				})

				It("includes correct s filter", func() {
					encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
						[]record.Record{{
							GroupIDs: []string{"1"},
							Domain:   "a2_domain1.",
						}},
						aliasDefinitions,
					)
					Expect(encodedAliases).To(
						Equal(
							map[string][]string{"custom-alias.": {"q-s0.q-g1.a2_domain1."}},
						),
					)
				})
			})

			Context("when healthy", func() {
				BeforeEach(func() {
					aliasDefinitions = map[string][]records.AliasDefinition{
						"custom-alias.": []records.AliasDefinition{
							{
								GroupID:      "1",
								RootDomain:   "a2_domain1",
								HealthFilter: "healthy",
							},
						},
					}
				})

				It("includes correct s filter", func() {
					encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
						[]record.Record{{
							GroupIDs: []string{"1"},
							Domain:   "a2_domain1.",
						}},
						aliasDefinitions,
					)
					Expect(encodedAliases).To(
						Equal(
							map[string][]string{"custom-alias.": {"q-s3.q-g1.a2_domain1."}},
						),
					)
				})
			})

			Context("when all", func() {
				BeforeEach(func() {
					aliasDefinitions = map[string][]records.AliasDefinition{
						"custom-alias.": []records.AliasDefinition{
							{
								GroupID:      "1",
								RootDomain:   "a2_domain1",
								HealthFilter: "all",
							},
						},
					}
				})

				It("includes correct s filter", func() {
					encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
						[]record.Record{{
							GroupIDs: []string{"1"},
							Domain:   "a2_domain1.",
						}},
						aliasDefinitions,
					)
					Expect(encodedAliases).To(
						Equal(
							map[string][]string{"custom-alias.": {"q-s4.q-g1.a2_domain1."}},
						),
					)
				})
			})

			Context("when unhealthy", func() {
				BeforeEach(func() {
					aliasDefinitions = map[string][]records.AliasDefinition{
						"custom-alias.": []records.AliasDefinition{
							{
								GroupID:      "1",
								RootDomain:   "a2_domain1",
								HealthFilter: "unhealthy",
							},
						},
					}
				})

				It("includes correct s filter", func() {
					encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
						[]record.Record{{
							GroupIDs: []string{"1"},
							Domain:   "a2_domain1.",
						}},
						aliasDefinitions,
					)
					Expect(encodedAliases).To(
						Equal(
							map[string][]string{"custom-alias.": {"q-s1.q-g1.a2_domain1."}},
						),
					)
				})
			})
		})

		Context("with initial_health_check", func() {
			Context("when asynchronous", func() {
				BeforeEach(func() {
					aliasDefinitions = map[string][]records.AliasDefinition{
						"custom-alias.": []records.AliasDefinition{
							{
								GroupID:            "1",
								RootDomain:         "a2_domain1",
								InitialHealthCheck: "asynchronous",
							},
						},
					}
				})

				It("includes correct y filter", func() {
					encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
						[]record.Record{{
							GroupIDs: []string{"1"},
							Domain:   "a2_domain1.",
						}},
						aliasDefinitions,
					)
					Expect(encodedAliases).To(
						Equal(
							map[string][]string{"custom-alias.": {"q-s0y0.q-g1.a2_domain1."}},
						),
					)
				})

				Context("when synchronous", func() {
					BeforeEach(func() {
						aliasDefinitions = map[string][]records.AliasDefinition{
							"custom-alias.": []records.AliasDefinition{
								{
									GroupID:            "1",
									RootDomain:         "a2_domain1",
									InitialHealthCheck: "synchronous",
								},
							},
						}
					})

					It("includes correct y filter", func() {
						encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
							[]record.Record{{GroupIDs: []string{"1"}, Domain: "a2_domain1"}},
							aliasDefinitions,
						)
						Expect(encodedAliases).To(
							Equal(
								map[string][]string{"custom-alias.": {"q-s0y1.q-g1.a2_domain1."}},
							),
						)
					})
				})
			})
		})

		Context("with placeholder_type", func() {
			Context("when uuid", func() {
				BeforeEach(func() {
					aliasDefinitions = map[string][]records.AliasDefinition{
						"_.custom-alias": []records.AliasDefinition{
							{
								GroupID:         "1",
								RootDomain:      "a2_domain1",
								PlaceholderType: "uuid",
							},
						},
					}
				})

				It("includes an entry for each matching group ID", func() {
					encodedAliases := aliasEncoder.EncodeAliasesIntoQueries(
						[]record.Record{{
							ID:       "instance0",
							GroupIDs: []string{"1"},
							NumID:    "0",
							Domain:   "a2_domain1.",
						}},
						aliasDefinitions,
					)
					Expect(encodedAliases).To(
						Equal(
							map[string][]string{"instance0.custom-alias.": {"q-m0s0.q-g1.a2_domain1."}},
						),
					)
				})
			})
		})

	})
})
