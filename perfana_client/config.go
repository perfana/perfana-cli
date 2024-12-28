package perfana_client

// Configuration struct to represent the YAML structure
type Configuration struct {
	ApiKey           string `yaml:"apiKey"`
	BaseUrl          string `yaml:"baseUrl"`
	ClientIdentifier string `yaml:"clientIdentifier"`
	SystemUnderTest  string `yaml:"systemUnderTest"`
	Environment      string `yaml:"environment"`
	Workload         string `yaml:"workload"`
	MTLS             struct {
		Enabled    bool   `yaml:"enabled"`
		ClientCert string `yaml:"clientCert"` // Path to the client certificate
		ClientKey  string `yaml:"clientKey"`  // Path to the client private key
	} `yaml:"mtls"`
}
