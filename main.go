package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
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

func resolveID(name string) (int, int, error) {
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
	uid, gid, err := resolveID("core")
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
	keysUpdater := exec.Command("/usr/bin/update-ssh-keys", "-u", "core")
	return keysUpdater.Run()
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
	templateStr := `{{ range $key, $value := . }}
{{ $key | ToUpper }}={{ $value }}
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
{{ range $tag := .Tags }}COREOS_CUSTOM_TAG_{{ $tag.Key | ToUpper }}={{ $tag.Value }}
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
	localAddr := &net.TCPAddr{
		Port: 10,
	}
	for {
		// Strive to find an available port lower than 1024
		if localAddr.Port >= 1024 {
			log.Fatal("failed to find a useable port lower than 1024")
		}
		ln, err := net.ListenTCP("tcp4", localAddr)
		if err != nil {
			localAddr.Port++
		} else {
			err := ln.Close()
			if err != nil {
				log.Fatalf("failed to close temporary TCP listener on :%d", localAddr.Port)
			}
			break
		}
	}

	return &net.Dialer{
		LocalAddr: localAddr,
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: false,
	}
}

func httpClient() *http.Client {
	client := http.DefaultClient
	client.Transport = &http.Transport{
		DialContext:     (scalewayLowPortDialer()).DialContext,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}
	client.Timeout = 10 * time.Second
	return client
}

func saveMD(md metadata.Metadata, path string) error {
	fd, err := os.Create(path)
	if err != nil {
		return fail("open environment file", err)
	}
	err = renderMetadata(fd, md)
	if err != nil {
		fd.Close()
		return fail("render environment file", err)
	}
	return fd.Close()
}
func saveUD(ud userdata.Userdata, path string) error {
	fd, err := os.Create(path)
	if err != nil {
		return fail("open environment file", err)
	}
	err = os.Chmod(udFile, 0600)
	if err != nil {
		fd.Close()
		return fail("set userdata environment file permissions", err)
	}
	err = renderUserdata(fd, ud)
	if err != nil {
		fd.Close()
		return fail("render environment file", err)
	}
	return fd.Close()
}

func fail(action string, err error) error {
	return fmt.Errorf("failed to %s: %v", action, err)
}
func main() {
	app := cobra.Command{
		Use:   "scaleway-coreos-custom-metadata",
		Short: "Fetch server metadata from scaleway API",
		Run: func(cmd *cobra.Command, _ []string) {
			udCount, err := cmd.Flags().GetString("wait-for-userdata-count")
			if err != nil {
				udCount = ""
			}
			client := httpClient()
			md, err := metadata.Self(client)
			if err != nil {
				log.Fatal(err)
			}
			err = saveMD(md, mdFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("INFO: saved metadata in %s", mdFile)
			err = saveSSHKeys(md)
			if err != nil {
				log.Fatal(err)
			}
			ud, err := userdata.Self(client)
			if err != nil {
				log.Fatal(err)
			}
			if udCount != "" {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					if v, ok := ud[udCount]; ok {
						count, err := strconv.ParseInt(v, 10, 64)
						if err != nil {
							log.Printf("WARN: failed to parse as an int the content of %s key: %s", udCount, v)
							break
						}
						if len(ud) >= int(count) {
							break
						}
						log.Printf("INFO: fetched %d/%d userdata", len(ud), count)
					} else {
						log.Printf("INFO: Waiting for %s key to be available", udCount)
					}
					<-ticker.C
				}
			}
			log.Printf("INFO: fetched all %d userdata", len(ud))
			err = saveUD(ud, udFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("INFO: saved userdata in %s", udFile)
		},
	}
	app.Flags().StringP("wait-for-userdata-count", "c", "", "wait for the given key to appear, and consider its content as the number of keys to wait")
	app.Execute()
}
