package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
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
	"github.com/jbonachera/scaleway-coreos-custom-metadata/state"
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
func saveSSHKeys(user string, md metadata.Metadata) error {
	uid, gid, err := resolveID(user)
	if err != nil {
		return err
	}
	home := fmt.Sprintf("/home/%s", user)
	keysDir := fmt.Sprintf("%s/.ssh/authorized_keys.d/", home)
	err = os.MkdirAll(keysDir, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}
	err = os.Chown(keysDir, uid, gid)
	if err != nil && !os.IsExist(err) {
		return err
	}
	keysFile := fmt.Sprintf("%s/scw-metadata", keysDir)
	sshDest, err := os.Create(keysFile)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	defer sshDest.Close()
	err = os.Chmod(keysFile, 0600)
	err = os.Chown(keysFile, uid, gid)
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

func scalewayLowPortDialer(ctx context.Context, network, addr string) (net.Conn, error) {
	port := rand.Intn(1023)
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			Port: port,
		},
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: false,
	}
	for {
		// Strive to find an available port lower than 1024
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			port = rand.Intn(1023)
			dialer.LocalAddr = &net.TCPAddr{
				Port: port,
			}
		} else {
			log.Printf("INFO: using local port %d", port)
			return conn, nil
		}
	}
}

func httpClient() *http.Client {
	client := http.DefaultClient
	client.Transport = &http.Transport{
		DialContext:     scalewayLowPortDialer,
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

func SSHKeys() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh-keys",
		Short: "fetch ssh-keys for given user",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			user := args[0]
			retries := 5
			client := http.DefaultClient
			var (
				md  metadata.Metadata
				err error
			)
			for retries > 0 {
				md, err = metadata.Self(client)
				if err == nil {
					break
				}
				time.Sleep(5 * time.Second)
				retries--
			}
			if err != nil {
				log.Fatal(err)
			}
			err = saveSSHKeys(user, md)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("INFO: saved ssh-keys for user %q", user)
		},
	}
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
			retries := 5
			client := httpClient()
			var md metadata.Metadata
			for retries > 0 {
				md, err = metadata.Self(client)
				if err == nil {
					break
				}
				log.Println("WARN: failed to load metadata from API, will retry in 5s")
				time.Sleep(5 * time.Second)
				retries--
			}
			if err != nil {
				log.Fatalf("failed to fetch metadata after 5 retries: %v", err)
			}
			log.Println("INFO: loaded metadata from API")
			err = saveMD(md, mdFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("INFO: saved metadata in %s", mdFile)
			ud, err := userdata.Self(client)
			if err != nil {
				log.Fatalf("failed to fetch userdata: %v", err)
			}
			if udCount != "" {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					if v, ok := ud[udCount]; ok && v != "" {
						count, err := strconv.ParseInt(v, 10, 64)
						if err == nil {
							if len(ud) >= int(count) {
								break
							}
							log.Printf("INFO: fetched %d/%d userdata", len(ud), count)
						} else {
							log.Printf("WARN: failed to parse as an int the content of %s key: %v", udCount, err)
						}
					} else {
						log.Printf("INFO: Waiting for %s key to be available", udCount)
					}
					<-ticker.C
					ud, err = userdata.Self(client)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
			log.Printf("INFO: fetched all %d userdata", len(ud))
			err = saveUD(ud, udFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("INFO: saved userdata in %s", udFile)
			err = state.SignalBooted(client)
			if err != nil {
				log.Printf("ERROR: failed to signal the control plane we booted")
				return
			} else {
				log.Printf("INFO: signaled the control plane we booted")
			}
		},
	}
	app.Flags().StringP("wait-for-userdata-count", "c", "", "wait for the given key to appear, and consider its content as the number of keys to wait")
	app.AddCommand(SSHKeys())
	app.Execute()
}
