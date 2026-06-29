package check

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	CheckTypeDNSLinkResolution = "DNSLinkResolution"
	CheckTypeProviderDiscovery = "ProviderDiscovery"
	CheckTypeBitswapRetrieval  = "BitswapRetrieval"
)

const (
	SourceDHT  = "Amino DHT"
	SourceIPNI = "IPNI"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var ns int64
	if err := json.Unmarshal(data, &ns); err == nil {
		*d = Duration(ns)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("duration must be int64 nanoseconds or Go duration string: %w", err)
	}

	s = strings.Trim(s, `"`)

	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration string %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(d), 10)), nil
}

type MutableResolution struct {
	InputPath      string `json:"InputPath,omitempty"`
	ResolvedPath   string `json:"ResolvedPath,omitempty"`
	DiagnosticURL  string `json:"DiagnosticURL,omitempty"`
	Error          string `json:"Error,omitempty"`
	IsMutableInput bool   `json:"IsMutableInput,omitempty"`
}

type BitswapCheckOutput struct {
	Enabled   bool     `json:"Enabled"`
	Duration  Duration `json:"Duration"`
	Found     bool     `json:"Found"`
	Responded bool     `json:"Responded"`
	Error     string   `json:"Error,omitempty"`
}

type HTTPCheckOutput struct {
	Enabled   bool     `json:"Enabled"`
	Duration  Duration `json:"Duration"`
	Endpoints []string `json:"Endpoints,omitempty"`
	Connected bool     `json:"Connected"`
	Requested bool     `json:"Requested"`
	Found     bool     `json:"Found"`
	Error     string   `json:"Error,omitempty"`
}

type ProviderOutput struct {
	ID                       string             `json:"ID"`
	ConnectionError          string             `json:"ConnectionError,omitempty"`
	Addrs                    []string           `json:"Addrs,omitempty"`
	ConnectionMaddrs         []string           `json:"ConnectionMaddrs,omitempty"`
	DataAvailableOverBitswap BitswapCheckOutput `json:"DataAvailableOverBitswap"`
	DataAvailableOverHTTP    HTTPCheckOutput    `json:"DataAvailableOverHTTP"`
	Source                   string             `json:"Source"`
	AgentVersion             string             `json:"AgentVersion,omitempty"`
}

type PeerCheckOutput struct {
	ConnectionError              string             `json:"ConnectionError,omitempty"`
	PeerFoundInDHT               map[string]int     `json:"PeerFoundInDHT,omitempty"`
	ProviderRecordFromPeerInDHT  bool               `json:"ProviderRecordFromPeerInDHT"`
	ProviderRecordFromPeerInIPNI bool               `json:"ProviderRecordFromPeerInIPNI"`
	ConnectionMaddrs             []string           `json:"ConnectionMaddrs,omitempty"`
	DataAvailableOverBitswap     BitswapCheckOutput `json:"DataAvailableOverBitswap"`
	DataAvailableOverHTTP        HTTPCheckOutput    `json:"DataAvailableOverHTTP"`
}

type CheckResponse struct {
	MutableResolution *MutableResolution `json:"MutableResolution,omitempty"`
	Providers         []ProviderOutput   `json:"Providers,omitempty"`
	Result            *PeerCheckOutput   `json:"Result,omitempty"`
}

func (cr *CheckResponse) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var providers []ProviderOutput
		if err := json.Unmarshal(data, &providers); err != nil {
			return fmt.Errorf("unmarshaling bare provider array: %w", err)
		}
		cr.Providers = providers
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling check response: %w", err)
	}

	if v, ok := raw["MutableResolution"]; ok {
		var mr MutableResolution
		if err := json.Unmarshal(v, &mr); err != nil {
			return fmt.Errorf("unmarshaling MutableResolution: %w", err)
		}
		cr.MutableResolution = &mr
	}

	if v, ok := raw["Providers"]; ok {
		var providers []ProviderOutput
		if err := json.Unmarshal(v, &providers); err != nil {
			return fmt.Errorf("unmarshaling Providers: %w", err)
		}
		cr.Providers = providers
	}

	if v, ok := raw["Result"]; ok {
		var result PeerCheckOutput
		if err := json.Unmarshal(v, &result); err != nil {
			return fmt.Errorf("unmarshaling Result: %w", err)
		}
		cr.Result = &result
	}

	return nil
}
