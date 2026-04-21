package domain

type Bridge struct {
	ID              string `json:"id" yaml:"id"`
	Name            string `json:"name" yaml:"name"`
	Address         string `json:"address" yaml:"address"`
	Username        string `json:"username,omitempty" yaml:"username,omitempty"`
	ClientKey       string `json:"client_key,omitempty" yaml:"client_key,omitempty"`
	CertFingerprint string `json:"cert_fingerprint,omitempty" yaml:"cert_fingerprint,omitempty"`
	APIBaseV2       string `json:"api_base_v2,omitempty" yaml:"api_base_v2,omitempty"`
	APIBaseV1       string `json:"api_base_v1,omitempty" yaml:"api_base_v1,omitempty"`
}

type CommandContext struct {
	JSON       bool
	Bridge     string
	AllBridges bool
	Broadcast  bool
	TimeoutSec int
	Verbose    int
	NoColor    bool
	Command    string
}

type BridgeResult struct {
	BridgeID   string         `json:"bridge_id"`
	BridgeName string         `json:"bridge_name"`
	Success    bool           `json:"success"`
	StatusCode int            `json:"status_code,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type AggregateResult struct {
	Items   []BridgeResult `json:"items"`
	Summary map[string]any `json:"summary"`
}

type ResourceMatch struct {
	BridgeID     string `json:"bridge_id"`
	BridgeName   string `json:"bridge_name"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
}
