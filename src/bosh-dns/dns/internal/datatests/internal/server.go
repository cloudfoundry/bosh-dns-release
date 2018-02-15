package internal

import (
	"bosh-dns/dns/config"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/onsi/gomega/gexec"
	"bosh-dns/dns/internal/testhelpers"

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

	listenPort, err := testhelpers.GetFreePort()
	Expect(err).NotTo(HaveOccurred())

	listenAPIPort, err := testhelpers.GetFreePort()
	Expect(err).NotTo(HaveOccurred())

	configContents, err := json.Marshal(config.Config{
		Address:        "127.0.0.1",
		Port:           listenPort,
		RecordsFile:    "records.json",
		AliasFilesGlob: "aliases.json",
		UpcheckDomains: []string{"health.check.bosh."},

		API: config.APIConfig{
			Port:            listenAPIPort,
			CAFile:          "../../../api/assets/test_certs/test_ca.pem",
			CertificateFile: "../../../api/assets/test_certs/test_server.pem",
			PrivateKeyFile:  "../../../api/assets/test_certs/test_server.key",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	configFile, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(configContents))
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command(executable, "--config", configFile.Name())

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())

	return Server{
		Bind: fmt.Sprintf("127.0.0.1:%d", listenPort),
		session: session,
		cmd:     cmd,
	}
}

func StopSampleServer(server Server) {
	defer gexec.CleanupBuildArtifacts()

	if server.cmd.Process == nil {
		return
	}

	server.session.Kill()
	server.session.Wait()
}
