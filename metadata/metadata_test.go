package metadata

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockMDClient struct{}
type mockBody struct {
	r io.Reader
}

func (m *mockBody) Close() error {
	return nil
}
func (m *mockBody) Read(b []byte) (int, error) {
	return m.r.Read(b)
}

func (m *mockMDClient) Get(url string) (*http.Response, error) {
	body := strings.NewReader(`{
		"tags": ["foo", "bar", "key=value"],
		"state_detail": "booted",
		"public_ip": {
			"dynamic": true,
			"id": "00000000-0000-0000-0000-000000000000",
			"address": "192.0.2.1"
		},
		"ssh_public_keys": [
			{
				"key": "key1",
				"fingerprint": "fprint1"
			}
		],
		"private_ip": "10.0.0.1",
		"timezone": "UTC",
		"id": "00000000-0000-0000-0000-000000000000",
		"extra_networks": [],
		"name": "master",
		"hostname": "master",
		"bootscript": {
			"kernel": "http://169.254.42.24/kernel/x86_64-mainline-lts-4.4-4.4.127-rev1/vmlinuz-4.4.127",
			"title": "x86_64 mainline 4.4.127 rev1",
			"default": true,
			"dtb": "",
			"public": true,
			"initrd": "http://169.254.42.24/initrd/initrd-Linux-x86_64-v3.14.4.gz",
			"bootcmdargs": "LINUX_COMMON scaleway boot=local nbd.max_part=16",
			"architecture": "x86_64",
			"organization": "00000000-0000-0000-0000-000000000000",
			"id": "00000000-0000-0000-0000-000000000000"
		},
		"location": {
			"platform_id": "13",
			"hypervisor_id": "408",
			"node_id": "6",
			"cluster_id": "6",
			"zone_id": "par1"
		},
		"volumes": {
			"0": {
				"name": "name",
				"modification_date": "2018-05-19T17:13:32.377027+00:00",
				"export_uri": "device://dev/vda",
				"volume_type": "l_ssd",
				"creation_date": "2018-05-19T17:13:32.377027+00:00",
				"organization": "00000000-0000-0000-0000-000000000000",
				"server": {
					"id": "00000000-0000-0000-0000-000000000000",
					"name": "master"
				},
				"id": "00000000-0000-0000-0000-000000000000",
				"size": 50000000000
			}
		},
		"ipv6": null,
		"organization": "00000000-0000-0000-0000-000000000000",
		"commercial_type": "START1-S"
	}
	`)
	return &http.Response{
		Body: &mockBody{r: body},
	}, nil
}
func TestSelf(t *testing.T) {
	h := &mockMDClient{}
	md, err := Self(h)
	assert.Nil(t, err)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", md.Organization)
	assert.Equal(t, []string{"foo", "bar", "key=value"}, md.Tags)
}
