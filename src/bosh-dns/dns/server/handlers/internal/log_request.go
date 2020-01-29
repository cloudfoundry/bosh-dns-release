package internal

import (
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"strings"
)

func LogRequest(logger logger.Logger, handler dns.Handler, logTag string, duration int64, request *dns.Msg, response *dns.Msg, customMessage string) {
	types := make([]string, len(request.Question))
	domains := make([]string, len(request.Question))

	for i, q := range request.Question {
		types[i] = dns.Type(q.Qtype).String()
		domains[i] = q.Name
	}

	if customMessage != "" {
		customMessage = customMessage + " "
	}

	rcode := dns.RcodeServerFailure
	numAnswers := 0
	if response != nil {
		rcode = response.Rcode
		numAnswers = len(response.Answer)
	}

	logLine := fmt.Sprintf("%T Request qtype=[%s] qname=[%s] rcode=%s ancount=%d %stime=%dns",
		handler,
		strings.Join(types, ","),
		strings.Join(domains, ","),
		dns.RcodeToString[rcode],
		numAnswers,
		customMessage,
		duration,
	)

	logger.Debug(logTag, logLine)
}
