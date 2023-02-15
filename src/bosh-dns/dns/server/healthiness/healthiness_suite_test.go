package healthiness_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHealthiness(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/healthiness")
}
