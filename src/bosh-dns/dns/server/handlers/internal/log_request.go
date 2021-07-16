package internal

import (
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"strings"
)

func LogRequest(logger logger.Logger, handler dns.Handler, logTag string, duration int64, request *dns.Msg, response *dns.Msg, customMessage string) {
	logLine := getLogLine(handler, logTag, duration, request, response, customMessage)

	logger.Debug(logTag, logLine)
}

func LogRecursionInfo(recursionLogger logger.Logger, handler dns.Handler, logTag string, duration int64, request *dns.Msg, response *dns.Msg, customMessage string) {
	logLine := getLogLine(handler, logTag, duration, request, response, customMessage)

	recursionLogger.Info(logTag, logLine)
}

func LogReceivedRequest(logger logger.Logger, handler dns.Handler, logTag string, request *dns.Msg) {
	types, domains := readQuestions(request)
	logLine := fmt.Sprintf("%T Received request id=%d qtype=[%s] qname=[%s]",
		handler,
		request.Id,
		strings.Join(types, ","),
		strings.Join(domains, ","),
	)

	logger.Debug(logTag, logLine)
}

func readQuestions(request *dns.Msg) ([]string, []string) {
	types := make([]string, len(request.Question))
	domains := make([]string, len(request.Question))

	for i, q := range request.Question {
		types[i] = dns.Type(q.Qtype).String()
		domains[i] = q.Name
	}
	return types, domains
}

func getLogLine(handler dns.Handler, logTag string, duration int64, request *dns.Msg, response *dns.Msg, customMessage string) string {
	types, domains := readQuestions(request)

	if customMessage != "" {
		customMessage = customMessage + " "
	}

	rcode := dns.RcodeServerFailure
	numAnswers := 0
	if response != nil {
		rcode = response.Rcode
		numAnswers = len(response.Answer)
	}

	logLine := fmt.Sprintf("%T Request id=%d qtype=[%s] qname=[%s] rcode=%s ancount=%d %stime=%dns",
		handler,
		request.Id,
		strings.Join(types, ","),
		strings.Join(domains, ","),
		dns.RcodeToString[rcode],
		numAnswers,
		customMessage,
		duration,
	)

	return logLine
}
