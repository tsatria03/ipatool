package appstore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("itemNameAndVersion", func() {
	It("reads the store name and short version", func() {
		name, version := itemNameAndVersion(downloadItemResult{Metadata: map[string]interface{}{
			"itemName": "Dice Only", "bundleShortVersionString": "1.9", "bundleVersion": "42",
		}})
		Expect(name).To(Equal("Dice Only"))
		Expect(version).To(Equal("1.9"))
	})

	It("falls back to the build version for older apps", func() {
		_, version := itemNameAndVersion(downloadItemResult{Metadata: map[string]interface{}{
			"itemName": "101 Free Alerts", "bundleVersion": "1.0.0",
		}})
		Expect(version).To(Equal("1.0.0"))
	})

	It("returns empty strings when Apple sends neither", func() {
		name, version := itemNameAndVersion(downloadItemResult{})
		Expect(name).To(BeEmpty())
		Expect(version).To(BeEmpty())
	})
})
