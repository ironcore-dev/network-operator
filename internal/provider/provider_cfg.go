package provider

import "crypto/tls"


type SonicProviderInitConfig struct {
	ProviderType string

	Address string
	Port   int32
}

func (c *SonicProviderInitConfig) GetProviderType() string {
	return c.ProviderType
}


type OpenconfigProviderInitConfig struct {
	ProviderType string

	// Address is the API address of the device, in the format "host:port".
	Address string
	// Username for basic authentication. Might be empty if the device does not require authentication.
	Username string
	// Password for basic authentication. Might be empty if the device does not require authentication.
	Password string
	// TLS configuration for the connection.
	TLS *tls.Config
}

func (c *OpenconfigProviderInitConfig) GetProviderType() string {
	return c.ProviderType
}
