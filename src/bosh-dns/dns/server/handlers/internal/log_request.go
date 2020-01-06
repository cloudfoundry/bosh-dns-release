package internal

import (
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"strings"
)

func LogRequest(logger logger.Logger, handler dns.Handler, logTag string, duration int64, request *dns.Msg, code int, customMessage string) {
	types := make([]string, len(request.Question))
	domains := make([]string, len(request.Question))

	for i, q := range request.Question {
		types[i] = dns.Type(q.Qtype).String()
		domains[i] = q.Name
	}

	if customMessage != "" {
		customMessage = customMessage + " "
	}

	logLine := fmt.Sprintf("%T Request qtype=[%s] qname=[%s] rcode=%s %stime=%dns",
		handler,
		strings.Join(types, ","),
		strings.Join(domains, ","),
		dns.RcodeToString[code],
		customMessage,
		duration,
	)

	if code == dns.RcodeSuccess || code == dns.RcodeNotImplemented {
		logger.Debug(logTag, logLine)
	} else {
		logger.Warn(logTag, logLine)
	}
}
