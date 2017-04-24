package aliases

import (
//"strings"
)
import (
	"errors"
	"github.com/miekg/dns"
	"regexp"
)

type QualifiedName string

var allowedJsonRegexp = regexp.MustCompile(`^"(.+)"$`)

func (q *QualifiedName) UnmarshalJSON(jsonBytes []byte) error {
	matches := allowedJsonRegexp.FindStringSubmatch(string(jsonBytes))
	if len(matches) != 2 {
		return errors.New("json structure invalid")
	}

	name := dns.Fqdn(matches[1])

	*q = QualifiedName(name)
	return nil
}
