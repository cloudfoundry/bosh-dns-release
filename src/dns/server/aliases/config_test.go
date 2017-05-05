package aliases_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/server/aliases"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("IsReduced", func() {
		Context("when an alias refers to another alias in the config", func() {
			It("reports false", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"alias1": {"alias2"},
					"alias2": {"domain"},
				})

				Expect(c.IsReduced()).To(BeFalse())
			})
		})

		Context("when alias tree is flat", func() {
			It("reports true", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"alias1": {"domain"},
					"alias2": {"domain"},
				})

				Expect(c.IsReduced()).To(BeTrue())
			})
		})
	})

	Describe("underscore alias", func() {
		Context("single level aliases", func() {
			It("resolves", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"_.alias1": {"_.domain"},
				})

				Expect(c.Resolutions("x.alias1.")).To(Equal([]string{"x.domain."}))
			})
		})

		Context("alias resolutions with no substitutions", func() {
			It("resolves", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"_.alias1": {"domain.com"},
				})

				Expect(c.Resolutions("x.alias1.")).To(Equal([]string{"domain.com."}))
			})
		})

		Context("multi-level alias", func() {
			It("resolves", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"_.sub.alias1": {"_.deepsub.sub.domain"},
				})

				Expect(c.Resolutions("x.sub.alias1.")).To(Equal([]string{"x.deepsub.sub.domain."}))
			})
		})

		Context("empty underscore request", func() {
			It("should ignore", func() {
				c := MustNewConfigFromMap(map[string][]string{})

				Expect(c.Resolutions("_.")).To(BeNil())
			})
		})
	})

	Describe("Resolutions", func() {
		Context("when the resolving domain is aliased away", func() {
			It("reports the domains pointed to", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"alias": {"domain", "domain2"},
				})

				Expect(c.Resolutions("alias.")).To(Equal([]string{"domain.", "domain2."}))
			})
		})

		Context("when the resolving domain does not appear as an alias", func() {
			It("returns nil", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"alias": {"domain"},
				})

				Expect(c.Resolutions("normal.domain.")).To(BeNil())
			})
		})

		Context("when both a static and underscore alias would match", func() {
			It("resolves with the static alias", func() {
				c := MustNewConfigFromMap(map[string][]string{
					"something.alias": {"domain"},
					"_.alias":         {"underdomain"},
				})

				Expect(c.Resolutions("something.alias.")).To(Equal([]string{"domain."}))
				Expect(c.Resolutions("other.alias.")).To(Equal([]string{"underdomain."}))
			})
		})
	})

	Describe("Merge", func() {
		Context("when the target is an empty config", func() {
			It("presents the original config", func() {
				allConfigs := MustNewConfigFromMap(map[string][]string{
					"alias1":        {"domain1, domain2"},
					"alias2":        {"domain3"},
					"_.underalias1": {"_.underdomain1"},
				}).Merge(MustNewConfigFromMap(map[string][]string{}))

				Expect(allConfigs).To(Equal(MustNewConfigFromMap(map[string][]string{
					"alias1":        {"domain1, domain2"},
					"alias2":        {"domain3"},
					"_.underalias1": {"_.underdomain1"},
				})))
			})
		})

		Context("when there are several configs merging", func() {
			Context("when two or more of the configs share an alias entry", func() {
				It("prefers the entry from the first config it loaded", func() {
					allConfigs := MustNewConfigFromMap(map[string][]string{
						"alias1":        {"domain1, domain2"},
						"alias2":        {"domain3"},
						"_.underalias1": {"_.underdomain1"},
					}).Merge(MustNewConfigFromMap(map[string][]string{
						"alias2":        {"domain1, domain2"},
						"alias3":        {"domain3"},
						"_.underalias1": {"_.underdomain2"},
					}))

					Expect(allConfigs).To(Equal(MustNewConfigFromMap(map[string][]string{
						"alias1":        {"domain1, domain2"},
						"alias2":        {"domain3"},
						"alias3":        {"domain3"},
						"_.underalias1": {"_.underdomain1"},
					})))
				})
			})

			Context("when the configs found have no conflicting entries", func() {
				It("unions the configs", func() {
					allConfigs := MustNewConfigFromMap(map[string][]string{
						"alias1":        {"domain1, domain2"},
						"alias2":        {"domain3"},
						"_.underalias1": {"_.underdomain1"},
					}).Merge(MustNewConfigFromMap(map[string][]string{
						"alias3":        {"domain1, domain2"},
						"alias4":        {"domain3"},
						"_.underalias2": {"_.underdomain2"},
					}))

					Expect(allConfigs).To(Equal(MustNewConfigFromMap(map[string][]string{
						"alias1":        {"domain1, domain2"},
						"alias2":        {"domain3"},
						"alias3":        {"domain1, domain2"},
						"alias4":        {"domain3"},
						"_.underalias1": {"_.underdomain1"},
						"_.underalias2": {"_.underdomain2"},
					})))
				})
			})
		})
	})

	Describe("ReducedForm", func() {
		It("reduces a single alias", func() {
			reduced, err := MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain1"},
			}).ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain1"},
			})))
		})

		It("reduces aliased aliases", func() {
			reduced, err := MustNewConfigFromMap(map[string][]string{
				"alias1": {"alias2"},
				"alias2": {"domain2"},
			}).ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain2"},
				"alias2": {"domain2"},
			})))
		})

		It("reduces deeply aliased aliases", func() {
			reduced, err := MustNewConfigFromMap(map[string][]string{
				"alias1": {"alias2"},
				"alias2": {"alias3"},
				"alias3": {"domain3"},
			}).ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain3"},
				"alias2": {"domain3"},
				"alias3": {"domain3"},
			})))
		})

		It("reduces multiple aliases", func() {
			reduced, err := MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain1", "alias2", "alias3"},
				"alias2": {"domain2"},
				"alias3": {"domain3"},
			}).ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain1", "domain2", "domain3"},
				"alias2": {"domain2"},
				"alias3": {"domain3"},
			})))
		})

		It("errors on cyclic aliases", func() {
			_, err := MustNewConfigFromMap(map[string][]string{
				"alias1": {"domain1", "alias2"},
				"alias2": {"alias1"},
			}).ReducedForm()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve alias1.: recursion detected"))
		})
	})
})
