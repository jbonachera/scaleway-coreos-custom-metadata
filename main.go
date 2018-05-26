package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/alecthomas/template"
	"github.com/jbonachera/scaleway-coreos-custom-metadata/metadata"
	"github.com/spf13/cobra"
)

const mdFile = "/run/metadata/coreos"

func resolveId(name string) (int, int, error) {
	user, err := user.Lookup(name)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.ParseInt(user.Uid, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return int(uid), int(gid), nil
}

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
			uid, gid, err := resolveId("core")
			if err != nil {
				log.Fatal(err)
			}
			err = os.MkdirAll("/home/core/.ssh/authorized_keys.d/", 0700)
			if err != nil {
				log.Fatal(err)
			}
			err = os.Chown("/home/core/.ssh/authorized_keys.d/", uid, gid)
			if err != nil {
				log.Fatal(err)
			}
			sshDest, err := os.Create("/home/core/.ssh/authorized_keys.d/scw-metadata")
			if err != nil {
				log.Fatal(err)
			}
			err = os.Chmod("/home/core/.ssh/authorized_keys.d/scw-metadata", 0600)
			err = os.Chown("/home/core/.ssh/authorized_keys.d/scw-metadata", uid, gid)
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
