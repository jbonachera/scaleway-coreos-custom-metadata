package main

import (
	"bytes"
	"testing"

	"github.com/jbonachera/scaleway-coreos-custom-metadata/metadata"
	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	md := metadata.Metadata{
		Hostname:  "host-1",
		PrivateIP: "10.0.0.1",
		Tags: []string{
			"prod",
			"seed=12",
			"foo=bar",
		},
	}
	buf := bytes.NewBufferString("")
	err := renderMetadata(buf, md)
	assert.Nil(t, err)
	assert.Equal(t, `COREOS_CUSTOM_HOSTNAME=host-1
COREOS_CUSTOM_PRIVATE_IPV4=10.0.0.1
COREOS_CUSTOM_PUBLIC_IPV4=
COREOS_CUSTOM_ZONE_ID=
COREOS_CUSTOM_TAG_SEED=12
COREOS_CUSTOM_TAG_FOO=bar
`, buf.String())
}
