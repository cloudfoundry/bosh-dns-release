package records

type Record struct {
	ID            string
	Group         string
	GroupIDs      []string
	Network       string
	Deployment    string
	IP            string
	Domain        string
	AZID          string
	InstanceIndex string
}
