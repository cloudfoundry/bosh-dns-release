package api

type InstanceRecord struct {
	ID          string `json:"id"`
	Group       string `json:"group"`
	Network     string `json:"network"`
	Deployment  string `json:"deployment"`
	IP          string `json:"ip"`
	Domain      string `json:"domain"`
	AZ          string `json:"az"`
	Index       string `json:"index"`
	HealthState string `json:"health_state"`
}

type Group struct {
	JobName     string `json:"job_name"`
	LinkName    string `json:"link_name"`
	LinkType    string `json:"link_type"`
	GroupID     int    `json:"group_id"`
	HealthState string `json:"health_state"`
}
