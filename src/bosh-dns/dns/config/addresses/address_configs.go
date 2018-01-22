package addresses

type AddressConfigs []AddressConfig

type AddressConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}
