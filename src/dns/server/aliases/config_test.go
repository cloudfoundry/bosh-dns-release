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
				c := Config{
					"alias1": {"alias2"},
					"alias2": {"domain"},
				}

				Expect(c.IsReduced()).To(BeFalse())
			})
		})

		Context("when alias tree is flat", func() {
			It("reports true", func() {
				c := Config{
					"alias1": {"domain"},
					"alias2": {"domain"},
				}

				Expect(c.IsReduced()).To(BeTrue())
			})
		})
	})

	Describe("Resolutions", func() {
		Context("when the resolving domain is aliased away", func() {
			It("reports the domains pointed to", func() {
				c := Config{
					"alias": {"domain", "domain2"},
				}

				Expect(c.Resolutions("alias")).To(Equal([]QualifiedName{"domain", "domain2"}))
			})
		})

		Context("when the resolving domain does not appear as an alias", func() {
			It("reports the original domain", func() {
				c := Config{
					"alias": {"domain"},
				}

				Expect(c.Resolutions("normal.domain")).To(Equal([]QualifiedName{"normal.domain"}))
			})
		})
	})

	Describe("Merge", func() {
		Context("when the target is an empty config", func() {
			It("presents the original config", func() {
				allConfigs := Config{
					"alias1": {"domain1, domain2"},
					"alias2": {"domain3"},
				}.Merge(Config{})

				Expect(allConfigs).To(Equal(Config{
					"alias1": {"domain1, domain2"},
					"alias2": {"domain3"},
				}))
			})
		})

		Context("when there are several configs merging", func() {
			Context("when two or more of the configs share an alias entry", func() {
				It("prefers the entry from the first config it loaded", func() {
					allConfigs := Config{
						"alias1": {"domain1, domain2"},
						"alias2": {"domain3"},
					}.Merge(Config{
						"alias2": {"domain1, domain2"},
						"alias3": {"domain3"},
					})

					Expect(allConfigs).To(Equal(Config{
						"alias1": {"domain1, domain2"},
						"alias2": {"domain3"},
						"alias3": {"domain3"},
					}))
				})
			})

			Context("when the configs found have no conflicting entries", func() {
				It("unions the configs", func() {
					allConfigs := Config{
						"alias1": {"domain1, domain2"},
						"alias2": {"domain3"},
					}.Merge(Config{
						"alias3": {"domain1, domain2"},
						"alias4": {"domain3"},
					})

					Expect(allConfigs).To(Equal(Config{
						"alias1": {"domain1, domain2"},
						"alias2": {"domain3"},
						"alias3": {"domain1, domain2"},
						"alias4": {"domain3"},
					}))
				})
			})
		})
	})

	Describe("ReducedForm", func() {
		It("reduces a single alias", func() {
			reduced, err := Config{
				"alias1": {"domain1"},
			}.ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(Config{
				"alias1": {"domain1"},
			}))
		})

		It("reduces aliased aliases", func() {
			reduced, err := Config{
				"alias1": {"alias2"},
				"alias2": {"domain2"},
			}.ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(Config{
				"alias1": {"domain2"},
				"alias2": {"domain2"},
			}))
		})

		It("reduces deeply aliased aliases", func() {
			reduced, err := Config{
				"alias1": {"alias2"},
				"alias2": {"alias3"},
				"alias3": {"domain3"},
			}.ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(Config{
				"alias1": {"domain3"},
				"alias2": {"domain3"},
				"alias3": {"domain3"},
			}))
		})

		It("reduces multiple aliases", func() {
			reduced, err := Config{
				"alias1": {"domain1", "alias2", "alias3"},
				"alias2": {"domain2"},
				"alias3": {"domain3"},
			}.ReducedForm()

			Expect(err).ToNot(HaveOccurred())
			Expect(reduced).To(Equal(Config{
				"alias1": {"domain1", "domain2", "domain3"},
				"alias2": {"domain2"},
				"alias3": {"domain3"},
			}))
		})

		It("errors on cyclic aliases", func() {
			_, err := Config{
				"alias1": {"domain1", "alias2"},
				"alias2": {"alias1"},
			}.ReducedForm()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve alias1: recursion detected"))
		})
	})
})
