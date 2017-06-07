package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var (
	logger boshlog.Logger
	logTag string
)

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	listenPort := os.Args[1]

	logger = boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, os.Stdout, os.Stderr)
	logTag = "main"
	defer logger.FlushTimeout(5 * time.Second)

	http.HandleFunc("/health", healthEntryPoint)
	logger.Error(logTag, "http healthcheck ending with %s", http.ListenAndServe(fmt.Sprintf(":%s", listenPort), nil))

	return 0
}

func healthEntryPoint(w http.ResponseWriter, _ *http.Request) {
	ret := map[string]string{
		"state": "running",
	}

	asJson, err := json.Marshal(ret)
	if err != nil {
		logger.Error(logTag, "Couldn't marshall healthcheck data %s. error: %s", ret, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(asJson)
}
