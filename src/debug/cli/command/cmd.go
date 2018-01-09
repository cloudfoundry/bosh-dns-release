package command

import (
	"bosh-dns/dns/api"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry/bosh-cli/ui"
	boshtbl "github.com/cloudfoundry/bosh-cli/ui/table"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type Commands struct {
	Instances InstancesCmd `command:"instances" description:"Show known instances"`

	UI ui.UI
}

type InstancesCmd struct {
	Args InstancesArgs `positional-args:"true"`
	API  string        `long:"api" env:"DNS_DEBUG_API_ADDRESS" description:"API address to talk to"`

	UI ui.UI
}

type InstancesArgs struct {
	Query string `positional-arg-name:"QUERY" description:"BOSH-DNS query formatted instance filter"`
}

func (o *InstancesCmd) Execute(args []string) error {
	if o.UI == nil {
		confUI := ui.NewConfUI(boshlog.NewLogger(boshlog.LevelNone))
		confUI.EnableColor()
		o.UI = confUI
	}

	client := httpclient.CreateDefaultClient(nil)

	requestURL := o.API + "/instances"

	if o.Args.Query != "" {
		requestURL = requestURL + "?address=" + o.Args.Query
	}

	response, err := client.Get(requestURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unable to retrieve instances: Got %s", response.Status)
	}

	table := boshtbl.Table{
		Title: "Known DNS instances",
		Header: []boshtbl.Header{
			boshtbl.NewHeader("ID"),
			boshtbl.NewHeader("Group"),
			boshtbl.NewHeader("Network"),
			boshtbl.NewHeader("Deployment"),
			boshtbl.NewHeader("IP"),
			boshtbl.NewHeader("Domain"),
			boshtbl.NewHeader("AZ"),
			boshtbl.NewHeader("Index"),
			boshtbl.NewHeader("HealthState"),
		},
	}

	decoder := json.NewDecoder(response.Body)

	for decoder.More() {
		var jsonRow api.Record

		err := decoder.Decode(&jsonRow)
		if err != nil {
			return err
		}

		table.Rows = append(table.Rows, []boshtbl.Value{
			boshtbl.NewValueString(jsonRow.ID),
			boshtbl.NewValueString(jsonRow.Group),
			boshtbl.NewValueString(jsonRow.Network),
			boshtbl.NewValueString(jsonRow.Deployment),
			boshtbl.NewValueString(jsonRow.IP),
			boshtbl.NewValueString(jsonRow.Domain),
			boshtbl.NewValueString(jsonRow.AZ),
			boshtbl.NewValueString(jsonRow.Index),
			boshtbl.NewValueString(jsonRow.HealthState),
		})
	}

	o.UI.PrintTable(table)

	return nil
}
