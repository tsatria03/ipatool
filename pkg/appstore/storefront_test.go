package appstore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CountryCode", func() {
	It("maps a store front to its country code", func() {
		code, err := CountryCode("143441-1,29")
		Expect(err).ToNot(HaveOccurred())
		Expect(code).To(Equal("US"))
	})

	It("fails for an unknown store front", func() {
		_, err := CountryCode("999999")
		Expect(err).To(HaveOccurred())
	})
})
