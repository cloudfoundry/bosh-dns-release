package internal

import (
	"bosh-dns/dns/config"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"bosh-dns/dns/internal/testhelpers"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Server struct {
	Bind    string
	cmd     *exec.Cmd
	session *gexec.Session
}

func StartSampleServer() Server {
	executable, err := gexec.Build("bosh-dns/dns")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)

	listenPort, err := testhelpers.GetFreePort()
	Expect(err).NotTo(HaveOccurred())

	listenAPIPort, err := testhelpers.GetFreePort()
	Expect(err).NotTo(HaveOccurred())

	jobsDir, err := os.MkdirTemp("", "jobs")
	Expect(err).NotTo(HaveOccurred())

	cfg := config.NewDefaultConfig()
	cfg.Address = "127.0.0.1"
	cfg.Port = listenPort
	cfg.RecordsFile = "records.json"
	cfg.AliasFilesGlob = "aliases.json"
	cfg.UpcheckDomains = []string{"health.check.bosh."}
	cfg.JobsDir = jobsDir
	cfg.API = config.APIConfig{
		Port:            listenAPIPort,
		CertificateFile: "../../../api/assets/test_certs/test_server.pem",
		PrivateKeyFile:  "../../../api/assets/test_certs/test_server.key",
		CAFile:          "../../../api/assets/test_certs/test_ca.pem",
	}

	configContents, err := json.Marshal(cfg)
	Expect(err).NotTo(HaveOccurred())

	configFile, err := os.CreateTemp("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(configContents))
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command(executable, "--config", configFile.Name())

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())

	return Server{
		Bind:    fmt.Sprintf("127.0.0.1:%d", listenPort),
		session: session,
		cmd:     cmd,
	}
}

func StopSampleServer(server Server) {
	defer gexec.CleanupBuildArtifacts()

	if server.cmd == nil || server.cmd.Process == nil {
		return
	}

	server.session.Kill()
	server.session.Wait()
}
