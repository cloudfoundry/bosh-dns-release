package manager

import "github.com/cloudfoundry/bosh-utils/system"

type systemdResolvedManager struct {
	cmdRunner system.CmdRunner
}

func NewSystemdResolvedManager(cmdRunner system.CmdRunner) systemdResolvedManager {
	return systemdResolvedManager{
		cmdRunner: cmdRunner,
	}
}

func (r systemdResolvedManager) UpdateDomains(domains []string) error {
	resolvectlArgs := []string{"domain", "bosh-dns"}
	for _, d := range domains {
		resolvectlArgs = append(resolvectlArgs, "~"+d)
	}

	_, _, _, err := r.cmdRunner.RunCommand("resolvectl", resolvectlArgs...)
	return err
}
