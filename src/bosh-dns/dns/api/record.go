package api

type Record struct {
	ID         string `json:"id"`
	Group      string `json:"group"`
	Network    string `json:"network"`
	Deployment string `json:"deployment"`
	IP         string `json:"ip"`
	Domain     string `json:"domain"`
	AZ         string `json:"az"`
	Index      string `json:"index"`
	Healthy    bool   `json:"healthy"`
}
