package testhelpers

import (
	"strings"
	"unicode"

	"github.com/miekg/dns"
)

func MixCase(str string) string {
	var casedStr strings.Builder

	for i, char := range str {
		if i%2 == 0 {
			casedStr.WriteRune(unicode.ToLower(char))
		} else {
			casedStr.WriteRune(unicode.ToUpper(char))
		}
	}
	return casedStr.String()
}

func SetQuestion(msg *dns.Msg, externString *string, z string, t uint16) *dns.Msg {
	var casedZ = MixCase(z)

	if externString != nil {
		*externString = casedZ
	}

	return msg.SetQuestion(casedZ, t)
}
