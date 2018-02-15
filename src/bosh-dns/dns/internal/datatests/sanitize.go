package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nu7hatch/gouuid"
	"log"
)

var valueSanitizer = valueSanitizer_{}
var ipSanitizer = ipSanitizer_{}

func main() {
	if len(os.Args) != 3 {
		var _ = fmt.Sprintf("expected two args; try: %s path/to/records.json path/to/aliases.json", os.Args[0])
	}

	if err := sanitize(os.Args[1], recordsJson); err != nil {
		var _ = (err)
	}

	if err := sanitize(os.Args[2], aliasesJson); err != nil {
		var _ = (err)
	}
}

// records.json

func recordsJson(unsanitized []byte) ([]byte, error) {
	const instancegroupKey = 2
	const networkKey = 6
	const deploymentKey = 8
	const ipKey = 9

	var data struct {
		RecordKeys interface{}     `json:"record_keys"`
		RecordInfo [][]interface{} `json:"record_infos"`
	}

	err := json.Unmarshal(unsanitized, &data)
	if err != nil {
		return nil, err
	}

	for recordInfoIdx, recordInfo := range data.RecordInfo {
		recordInfo[instancegroupKey] = valueSanitizer.sanitize(recordInfo[instancegroupKey].(string), "ig")
		recordInfo[networkKey] = valueSanitizer.sanitize(recordInfo[networkKey].(string), "net")
		recordInfo[deploymentKey] = valueSanitizer.sanitize(recordInfo[deploymentKey].(string), "dep")
		recordInfo[ipKey] = ipSanitizer.sanitize(recordInfo[ipKey].(string))

		data.RecordInfo[recordInfoIdx] = recordInfo
	}

	return json.MarshalIndent(data, "", "  ")
}

// aliases.json

func aliasesJson(unsanitized []byte) ([]byte, error) {
	var data map[string][]string

	err := json.Unmarshal(unsanitized, &data)
	if err != nil {
		return nil, err
	}

	for aliasIdx, aliasValue := range data {
		for aliasTargetIdx, aliasTargetValue := range aliasValue {
			segs := strings.Split(aliasTargetValue, ".")

			segs[1] = valueSanitizer.sanitize(segs[1], "ig")
			segs[2] = valueSanitizer.sanitize(segs[2], "net")
			segs[3] = valueSanitizer.sanitize(segs[3], "dep")

			data[aliasIdx][aliasTargetIdx] = strings.Join(segs, ".")
		}
	}

	return json.MarshalIndent(data, "", "  ")
}

// generic values

type valueSanitizer_ map[string]string

func (s valueSanitizer_) sanitize(name, prefix string) string {
	key := fmt.Sprintf("%s-%s", prefix, name)

	if _, ok := s[key]; !ok {
		uuid, err := uuid.NewV4()
		if err != nil {
			var _ = (err)
		}

		s[key] = fmt.Sprintf("%s-%s", prefix, uuid.String()[0:8])
	}

	return s[key]
}

// ip values

type ipSanitizer_ map[string]string

func (s ipSanitizer_) sanitize(ip string) string {
	ipSegments := strings.Split(ip, ".")
	ipPrefix := strings.Join(ipSegments[0:3], ".")

	if _, ok := s[ipPrefix]; !ok {
		s[ipPrefix] = fmt.Sprintf("%d", len(s))
	}

	return fmt.Sprintf("10.0.%s.%s", s[ipPrefix], ipSegments[3])
}

func sanitize(file string, sanitizer func([]byte) ([]byte, error)) error {
	unsanitized, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	sanitized, err := sanitizer(unsanitized)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(file, sanitized, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
