package command

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudfoundry/bosh-cli/v7/ui"
	boshtbl "github.com/cloudfoundry/bosh-cli/v7/ui/table"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-dns/dns/api"
	"bosh-dns/tlsclient"
)

type LocalGroupsCmd struct {
	API                string `long:"api" env:"DNS_API_ADDRESS" description:"API address to talk to"`
	TLSCACertPath      string `long:"ca-cert-path" env:"DNS_API_TLS_CA_CERT_PATH" description:"CA certificate to use for mutual LS"`
	TLSCertificatePath string `long:"certificate-path" env:"DNS_API_TLS_CERTIFICATE_PATH" description:"Client certificate to use for mutual LS"`
	TLSPrivateKeyPath  string `long:"private-key-path" env:"DNS_API_TLS_PRIVATE_KEY_PATH" description:"Client key to use for mutual LS"`

	UI ui.UI
}

func (o *LocalGroupsCmd) Execute(args []string) error {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	if o.UI == nil {
		confUI := ui.NewConfUI(logger)
		confUI.EnableColor()
		o.UI = confUI
	}

	client, err := tlsclient.NewFromFiles("api.bosh-dns", o.TLSCACertPath, o.TLSCertificatePath, o.TLSPrivateKeyPath, 5*time.Second, logger)
	if err != nil {
		return err
	}

	requestURL := o.API + "/local-groups"

	response, err := client.Get(requestURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unable to retrieve groups: Got %s", response.Status)
	}

	table := boshtbl.Table{
		FillFirstColumn: true,
		Header: []boshtbl.Header{
			boshtbl.NewHeader("JobName"),
			boshtbl.NewHeader("LinkName"),
			boshtbl.NewHeader("LinkType"),
			boshtbl.NewHeader("GroupID"),
			boshtbl.NewHeader("HealthState"),
		},
	}

	decoder := json.NewDecoder(response.Body)

	for decoder.More() {
		var jsonRow api.Group

		err := decoder.Decode(&jsonRow)
		if err != nil {
			return err
		}

		table.Rows = append(table.Rows, []boshtbl.Value{
			boshtbl.NewValueString(jsonRow.JobName),
			boshtbl.NewValueString(jsonRow.LinkName),
			boshtbl.NewValueString(jsonRow.LinkType),
			boshtbl.NewValueString(jsonRow.GroupID),
			boshtbl.NewValueString(jsonRow.HealthState),
		})
	}

	o.UI.PrintTable(table)

	return nil
}
