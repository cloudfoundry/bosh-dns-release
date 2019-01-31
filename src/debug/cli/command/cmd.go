package command

import (
	"github.com/cloudfoundry/bosh-cli/ui"
)

type Commands struct {
	Instances InstancesCmd `command:"instances" description:"Show known instances"`
	Groups    GroupsCmd    `command:"groups" description:"Show group health status"`

	UI ui.UI
}
