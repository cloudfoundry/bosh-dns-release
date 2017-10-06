package records

type Record struct {
	ID            string
	GlobalIndex   string
	Group         string
	GroupIDs      []string
	Network       string
	NetworkID     string
	Deployment    string
	IP            string
	Domain        string
	AZID          string
	InstanceIndex string
}
