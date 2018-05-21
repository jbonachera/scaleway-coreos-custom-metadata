package metadata

import (
	"encoding/json"
	"net/http"
)

type httpGetter interface {
	Get(url string) (*http.Response, error)
}

const mdUrl = "http://169.254.42.42/conf?format=json"

type Metadata struct {
	Tags        []interface{} `json:"tags"`
	StateDetail string        `json:"state_detail"`
	PublicIP    struct {
		Dynamic bool   `json:"dynamic"`
		ID      string `json:"id"`
		Address string `json:"address"`
	} `json:"public_ip"`
	SSHPublicKeys []struct {
		Key         string `json:"key"`
		Fingerprint string `json:"fingerprint"`
	} `json:"ssh_public_keys"`
	PrivateIP  string `json:"private_ip"`
	Timezone   string `json:"timezone"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	Bootscript struct {
		Kernel       string `json:"kernel"`
		Title        string `json:"title"`
		Default      bool   `json:"default"`
		Dtb          string `json:"dtb"`
		Public       bool   `json:"public"`
		Initrd       string `json:"initrd"`
		Bootcmdargs  string `json:"bootcmdargs"`
		Architecture string `json:"architecture"`
		Organization string `json:"organization"`
		ID           string `json:"id"`
	} `json:"bootscript"`
	Location struct {
		PlatformID   string `json:"platform_id"`
		HypervisorID string `json:"hypervisor_id"`
		NodeID       string `json:"node_id"`
		ClusterID    string `json:"cluster_id"`
		ZoneID       string `json:"zone_id"`
	} `json:"location"`
	Ipv6           interface{} `json:"ipv6"`
	Organization   string      `json:"organization"`
	CommercialType string      `json:"commercial_type"`
}

// Self returns the server metadata from Scaleway API
// https://developer.scaleway.com/#metadata-server-metadata
func Self(client httpGetter) (Metadata, error) {
	md := Metadata{}
	resp, err := client.Get(mdUrl)
	if err != nil {
		return md, err
	}
	defer resp.Body.Close()
	return md, json.NewDecoder(resp.Body).Decode(&md)
}
