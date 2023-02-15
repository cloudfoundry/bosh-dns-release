package aliases_test

import (
	"bosh-dns/dns/server/aliases"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAliases(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/aliases")
}

func MustNewConfigFromMap(load map[string][]string) aliases.Config {
	caseScrambledLoad := make(map[string][]string)
	for k, v := range load {
		mixedCaseKey := MixCase(k)
		var mixedCaseValue []string
		for _, value := range v {
			mixedCaseValue = append(mixedCaseValue, MixCase(value))
		}
		caseScrambledLoad[mixedCaseKey] = mixedCaseValue
	}

	config, err := aliases.NewConfigFromMap(caseScrambledLoad)
	if err != nil {
		Fail(err.Error())
	}
	return config
}
