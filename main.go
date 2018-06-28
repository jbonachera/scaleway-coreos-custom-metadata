package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/template"
	"github.com/jbonachera/scaleway-coreos-custom-metadata/metadata"
	"github.com/jbonachera/scaleway-coreos-custom-metadata/userdata"
	"github.com/spf13/cobra"
)

const (
	mdFile = "/run/metadata/coreos"
	udFile = "/etc/userdata.env"
)

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
func saveSSHKeys(md metadata.Metadata) error {
	uid, gid, err := resolveId("core")
	if err != nil {
		return err
	}
	err = os.MkdirAll("/home/core/.ssh/authorized_keys.d/", 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}
	err = os.Chown("/home/core/.ssh/authorized_keys.d/", uid, gid)
	if err != nil && !os.IsExist(err) {
		return err
	}
	sshDest, err := os.Create("/home/core/.ssh/authorized_keys.d/scw-metadata")
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	defer sshDest.Close()
	err = os.Chmod("/home/core/.ssh/authorized_keys.d/scw-metadata", 0600)
	err = os.Chown("/home/core/.ssh/authorized_keys.d/scw-metadata", uid, gid)
	if err != nil {
		return err
	}
	for _, keyMD := range md.SSHPublicKeys {
		fmt.Fprint(sshDest, keyMD.Key)
	}
	return nil
}

type renderedMD struct {
	Hostname  string
	PrivateIP string
	PublicIP  string
	Zone      string
	Tags      []metadata.KVTag
}

func renderUserdata(out io.Writer, ud userdata.Userdata) error {
	funcMap := template.FuncMap{
		"ToUpper": strings.ToUpper,
	}

	templateStr := `{{ range $idx, $tag := . }}{{ $tag.Key | ToUpper }}={{ $tag.Value }}
	{{ end }}`
	template, err := template.New("").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return err
	}

	return template.Execute(out, ud)
}
func renderMetadata(out io.Writer, md metadata.Metadata) error {
	funcMap := template.FuncMap{
		"ToUpper": strings.ToUpper,
	}

	templateStr := `COREOS_CUSTOM_HOSTNAME={{ .Hostname }}
COREOS_CUSTOM_PRIVATE_IPV4={{ .PrivateIP }}
COREOS_CUSTOM_PUBLIC_IPV4={{ .PublicIP }}
COREOS_CUSTOM_ZONE_ID={{ .Zone }}
{{ range $idx, $tag := .Tags }}COREOS_CUSTOM_TAG_{{ $tag.Key | ToUpper }}={{ $tag.Value }}
{{ end }}`
	template, err := template.New("").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return err
	}

	return template.Execute(out, renderedMD{
		Hostname:  md.Hostname,
		PrivateIP: md.PrivateIP,
		PublicIP:  md.PublicIP.Address,
		Zone:      md.Location.ZoneID,
		Tags:      md.KVTags(),
	})
}

func scalewayLowPortDialer() *net.Dialer {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatal(err)
	}
	addresses, err := iface.Addrs()
	if err != nil {
		log.Fatal(err)
	}
	localIPAddr := strings.Split(addresses[0].String(), "/")[0]
	localAddr, err := net.ResolveTCPAddr("tcp4",
		fmt.Sprintf("%s:%d", localIPAddr, 50))
	if err != nil {
		log.Fatal(err)
	}
	return &net.Dialer{
		LocalAddr: localAddr,
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: false,
	}
}

func main() {
	app := cobra.Command{
		Use:   "scaleway-coreos-custom-metadata",
		Short: "Fetch server metadata from scaleway API",
		Run: func(cmd *cobra.Command, _ []string) {

			client := http.DefaultClient
			client.Transport = &http.Transport{
				DialContext:           (scalewayLowPortDialer()).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}
			client.Timeout = 10 * time.Second
			md, err := metadata.Self(client)
			if err != nil {
				log.Fatal(err)
			}
			mdDest, err := os.Create(mdFile)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "WARN: failed to open environment file: %v", err)
			} else {
				defer mdDest.Close()
				err = renderMetadata(mdDest, md)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), "WARN: failed to render environment file: %v", err)
				}
			}
			err = saveSSHKeys(md)
			if err != nil {
				log.Fatal(err)
			}
			ud, err := userdata.Self(client)
			if err != nil {
				log.Fatal(err)
			}
			udDest, err := os.Create(udFile)
			os.Chmod(udFile, 0600)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "WARN: failed to open environment file: %v", err)
			} else {
				defer udDest.Close()
				err = renderUserdata(udDest, ud)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), "WARN: failed to render environment file: %v", err)
				}
			}
		},
	}
	app.Execute()
}
