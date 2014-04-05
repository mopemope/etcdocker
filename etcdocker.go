package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/api/client"
	"github.com/dotcloud/docker/opts"
	"github.com/dotcloud/docker/dockerversion"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/utils"

)

const (
	defaultCaFile = "ca.pem"
	defaultKeyFile = "key.pem"
	defaultCertFile = "cert.pem"
)

var (
	dockerConfDir = os.Getenv("HOME") + "/.docker/"
)

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}

func main() {

	var (
		flVersion = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
		flDebug = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
		flHosts = opts.NewListOpts(api.ValidateHost)
		flTls = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
		flTlsVerify = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
		flCa = flag.String([]string{"-tlscacert"}, dockerConfDir+defaultCaFile, "Trust only remotes providing a certificate signed by the CA given here")
		flCert = flag.String([]string{"-tlscert"}, dockerConfDir+defaultCertFile, "Path to TLS certificate file")
		flKey = flag.String([]string{"-tlskey"}, dockerConfDir+defaultKeyFile, "Path to TLS key file")
	)
	flag.Var(&flHosts, []string{"H", "-host"}, "tcp://host:port, unix://path/to/socket, fd://* or fd://socketfd to use in daemon mode. Multiple sockets can be specified")

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}
	if flHosts.Len() == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")

		if defaultHost == "" {
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		if _, err := api.ValidateHost(defaultHost); err != nil {
			log.Fatal(err)
		}
		flHosts.Set(defaultHost)
	}

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	if flHosts.Len() > 1 {
		log.Fatal("Please specify only one -H")
	}
	protoAddrParts := strings.SplitN(flHosts.GetAll()[0], "://", 2)

	var (
		cli *client.DockerCli
		tlsConfig tls.Config
	)
	tlsConfig.InsecureSkipVerify = true

// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		*flTls = true
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(*flCa)
		if err != nil {
			log.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
		}
		certPool.AppendCertsFromPEM(file)
		tlsConfig.RootCAs = certPool
		tlsConfig.InsecureSkipVerify = false
	}

// If tls is enabled, try to load and send client certificates
	if *flTls || *flTlsVerify {
		_, errCert := os.Stat(*flCert)
		_, errKey := os.Stat(*flKey)
		if errCert == nil && errKey == nil {
			*flTls = true
			cert, err := tls.LoadX509KeyPair(*flCert, *flKey)
			if err != nil {
				log.Fatalf("Couldn't load X509 key pair: %s. Key encrypted?", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	if *flTls || *flTlsVerify {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	} else {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], nil)
	}

	fArgs := flag.Args()
	if size, ecfg, err := checkArgs(cli, fArgs...); err == nil {

		args := modifyArgs(size, fArgs, ecfg)

		if err := cli.ParseCommands(args...); err != nil {
			if sterr, ok := err.(*utils.StatusError); ok {
				if sterr.Status != "" {
					log.Println(sterr.Status)
				}
				os.Exit(sterr.StatusCode)
			}
			log.Fatal(err)
		} else {
			// running 
			if ecfg != nil && ecfg.Name != "" && len(ecfg.NameInfo.PortBindings) > 0 {

				err := setDockerName(ecfg)
				if err != nil {
					log.Fatal(err)
					os.Exit(-1)
				}
			}
		}
	}
}

func modifyArgs (size int, fArgs []string, ecfg *EtcdDockerConfig ) []string {

	var (
		args = make([]string, size)
		i = 0
		nextVal = false
	)
	
	for _, c := range fArgs {
		
		if c != "-peer" &&
			c != "-endpoint"  &&
			c != "--peer" &&
			c != "--endpoint" {
			if (!nextVal) {
				args[i] = c
				i++
			} else {
				nextVal = false
			}
		} else {
			nextVal = true
		}
	}

	if ecfg != nil && ecfg.Envs != nil && len(ecfg.Envs) > 0 {
		args = append(append(args[:1], ecfg.Envs...), args[1:]...)
	}
	//fmt.Println(args)
	return args
}

