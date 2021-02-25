package record

type Host struct {
	IP   string
	FQDN string
}

type Record struct {
	ID            string
	NumID         string
	Group         string
	GroupIDs      []string
	Network       string
	NetworkID     string
	Deployment    string
	IP            string
	Domain        string
	AZ            string
	AZID          string
	AgentID       string
	InstanceIndex string
}
