package command

import (
	"github.com/cloudfoundry/bosh-cli/ui"
)

type Commands struct {
	Instances   InstancesCmd   `command:"instances" description:"Show known instances"`
	LocalGroups LocalGroupsCmd `command:"local-groups" description:"Show health status and link details for groups local to the current instance"`

	UI ui.UI
}
