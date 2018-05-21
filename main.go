package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/template"
	"github.com/jbonachera/scaleway-coreos-custom-metadata/metadata"
	"github.com/spf13/cobra"
)

const mdFile = "/run/metadata/coreos"

func main() {
	app := cobra.Command{
		Use:   "scaleway-coreos-custom-metadata",
		Short: "Fetch server metadata from scaleway API",
		Run: func(cmd *cobra.Command, _ []string) {
			client := http.DefaultClient
			client.Timeout = 10 * time.Second
			md, err := metadata.Self(client)
			if err != nil {
				log.Fatal(err)
			}
			templateStr := `COREOS_CUSTOM_HOSTNAME={{ .Hostname }}
COREOS_CUSTOM_PRIVATE_IPV4={{ .PrivateIP }}
COREOS_CUSTOM_PUBLIC_IPV4={{ .PublicIP.Address }}
COREOS_CUSTOM_ZONE_ID={{ .Location.ZoneID }}
`
			template, err := template.New("").Parse(templateStr)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStderr(), err)
			}
			mdDest, err := os.Create(mdFile)
			if err != nil {
				log.Fatal(err)
			}
			template.Execute(mdDest, md)
			mdDest.Close()
			sshDest, err := os.Create("/home/core/.ssh/authorized_keys.d/scw-metadata")
			if err != nil {
				log.Fatal(err)
			}
			for _, keyMD := range md.SSHPublicKeys {
				fmt.Fprint(sshDest, keyMD.Key)
			}
			sshDest.Close()
		},
	}
	app.Execute()
}
