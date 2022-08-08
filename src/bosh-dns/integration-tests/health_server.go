package integration_tests

import (
	"bosh-dns/dns/config"
	"bosh-dns/healthcheck/api"
	"bosh-dns/healthconfig"
	"bosh-dns/tlsclient"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type testHealthServer struct {
	address             string
	port                int
	agentHealthFileName string
	CAFile              string
	CertificateFile     string
	PrivateKeyFile      string
	client              *httpclient.HTTPClient
	rootDir             string
	ctx                 context.Context
	session             *gexec.Session
	stopHealthServer    func()
}

const jobsCount = 2

func NewTestHealthServer(address string) *testHealthServer {

	t := &testHealthServer{
		address:         address,
		port:            2345,
		CertificateFile: "../healthcheck/assets/test_certs/test_server.pem",
		PrivateKeyFile:  "../healthcheck/assets/test_certs/test_server.key",
		CAFile:          "../healthcheck/assets/test_certs/test_ca.pem",
	}

	return t
}

func (t *testHealthServer) Bootstrap() error {
	rootDir, err := os.MkdirTemp("", "root-dir")
	if err != nil {
		return err
	}
	t.rootDir = rootDir

	fmt.Fprintf(GinkgoWriter, "Created root-dir: %s\n", rootDir)
	for i := 0; i < jobsCount; i++ {
		err := os.MkdirAll(filepath.Join(t.rootDir, "jobs", strconv.Itoa(i), "bin", "dns"), 0740)
		if err != nil {
			return err
		}

		err = t.MakeHealthyExit(i, 0)
		if err != nil {
			return err
		}

		err = os.MkdirAll(filepath.Join(t.rootDir, "jobs", strconv.Itoa(i), ".bosh"), 0740)
		if err != nil {
			return err
		}

		err = t.MakeJobLinks(i)
		if err != nil {
			return err
		}
	}

	return t.updateAgentHealthFile()
}

func (t *testHealthServer) MakeJobLinks(index int) error {
	links := make([]healthconfig.LinkMetadata, 1)
	links[0].Name = "name"
	links[0].Group = strconv.Itoa(index)
	links[0].Type = "type"

	f, err := os.OpenFile(filepath.Join(t.rootDir, "jobs", strconv.Itoa(index), ".bosh", "links.json"),
		os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(links)
	if err != nil {
		return err
	}
	_, err = f.Write(jsonBytes)
	if err != nil {
		return err
	}
	f.Close()

	return nil
}

func (t *testHealthServer) MakeHealthyExit(index, status int) error {
	healthExecutable := "healthy"
	if runtime.GOOS == "windows" {
		healthExecutable = "healthy.ps1"
	}
	healthScript, err := os.OpenFile(
		filepath.Join(t.rootDir, "jobs", strconv.Itoa(index), "bin", "dns", healthExecutable),
		os.O_WRONLY|os.O_CREATE,
		0700,
	)
	if err != nil {
		return err
	}
	defer healthScript.Close()
	if runtime.GOOS == "windows" {
		fmt.Fprintf(healthScript, "exit %d", status)
		return nil
	}

	fmt.Fprintf(healthScript, "#!/bin/bash\n\nexit %d", status)
	return nil
}

func (t *testHealthServer) writeConfig() (string, error) {
	c := healthconfig.HealthCheckConfig{
		Address:                  t.address,
		Port:                     t.port,
		CAFile:                   t.CAFile,
		CertificateFile:          t.CertificateFile,
		PrivateKeyFile:           t.PrivateKeyFile,
		HealthExecutableInterval: config.DurationJSON(time.Second),
		HealthFileName:           t.agentHealthFileName,
		HealthExecutablePath:     "bin/dns/healthy",
		JobsDir:                  filepath.Join(t.rootDir, "jobs"),
	}

	if runtime.GOOS == "windows" {
		c.HealthExecutablePath = "bin/dns/healthy.ps1"
	}

	jsonBytes, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	configFile, err := os.OpenFile(filepath.Join(t.rootDir, "config.json"), os.O_WRONLY|os.O_CREATE, 0700)
	if err != nil {
		return "", err
	}

	defer configFile.Close()

	if _, err := configFile.Write(jsonBytes); err != nil {
		return configFile.Name(), err
	}

	return configFile.Name(), nil
}

func (t *testHealthServer) GetResponseBody() (api.HealthResult, error) {
	resp, err := t.GetResponse()
	if err != nil {
		return api.HealthResult{}, err
	}
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return api.HealthResult{}, err
	}

	var healthResp api.HealthResult
	err = json.Unmarshal(data, &healthResp)
	if err != nil {
		return healthResp, err
	}

	return healthResp, nil
}

func (t *testHealthServer) GetResponse() (*http.Response, error) {
	return t.client.Get(fmt.Sprintf("https://%s:%d/health", t.address, t.port))
}

func (t *testHealthServer) Start() error {
	err := t.Bootstrap()
	if err != nil {
		return err
	}

	healthServer, err := gexec.Build("bosh-dns/healthcheck/")
	if err != nil {
		return err
	}

	configPath, err := t.writeConfig()
	if err != nil {
		return err
	}

	t.ctx, t.stopHealthServer = context.WithCancel(context.Background())
	b := exec.CommandContext(t.ctx, healthServer, configPath)
	t.session, err = gexec.Start(b, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, io.Discard)
	t.client, err = tlsclient.NewFromFiles(
		"health.bosh-dns",
		t.CAFile,
		"../healthcheck/assets/test_certs/test_client.pem",
		"../healthcheck/assets/test_certs/test_client.key",
		5*time.Second,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() api.HealthResult {
		Expect(t.session.ExitCode()).To(Equal(-1), "Health server may already be running")

		res, _ := t.GetResponseBody()
		return res
	}, 10*time.Second, 2*time.Second).Should(Equal(api.HealthResult{
		State: api.StatusRunning,
		GroupState: map[string]api.HealthStatus{
			"0": api.StatusRunning,
			"1": api.StatusRunning,
		},
	}))

	return nil
}

func (t *testHealthServer) Stop() {
	t.stopHealthServer()
	t.session.Wait()
	os.RemoveAll(t.rootDir)
}

func (t *testHealthServer) updateAgentHealthFile() error {
	t.agentHealthFileName = filepath.Join(t.rootDir, "agent-health-file.json")
	healthRaw, err := json.Marshal(api.HealthResult{State: "running"})
	if err != nil {
		return err
	}

	err = os.WriteFile(t.agentHealthFileName, healthRaw, 0640)
	if err != nil {
		return err
	}

	return nil
}
