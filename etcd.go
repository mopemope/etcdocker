package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/mopemope/etdocker/runconfig"
	"github.com/coreos/etcdctl/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/dotcloud/docker/api/client"
)

type EtcdDockerConfig struct {
	Name     string
	HostIp   string
	Endpoint string
	Sync     bool
	NameInfo nameInfo
	Links    []linkInfo
	Envs     []string
	EtcdClient *etcd.Client
}

type linkInfo struct {
	Name  string
	Alias string
}

type publicPort struct {
	HostIp   string
	HostPort string
}

type portBinding struct {
	ContainerPort string
	PublicPorts   []publicPort
}

type nameInfo struct {
	Name         string
	HostIp       string
	PortBindings []portBinding
}

func localIP() (string, error) {
	tt, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, t := range tt {
		aa, err := t.Addrs()
		if err != nil {
			return "", err
		}
		for _, a := range aa {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			if v4 == nil || v4[0] == 127 || v4[0] == 172 {
				continue
			}
			return v4.String(), nil
		}
	}
	return "", errors.New("cannot find local IP address")
}

func newClient(ecfg *EtcdDockerConfig) *etcd.Client {
	
	if ecfg.EtcdClient != nil {
		return ecfg.EtcdClient
	}

	peers := make([]string, 0)

	if ecfg.Endpoint != "" {
		peers = append(peers, ecfg.Endpoint)
	}

	if len(peers) == 0 {
		peers = append(peers, "127.0.0.1:4001")
	}

	client := etcd.NewClient(peers)

	if ecfg.Sync {
		if ok := client.SyncCluster(); !ok {
			fmt.Println("Cannot sync with the cluster using peers", peers)
			os.Exit(-1)
		}
	}
	ecfg.EtcdClient = client
	return client
}

func getDockerName(ecfg *EtcdDockerConfig) ([]string, error) {

	client := newClient(ecfg)
	envs := make([]string, 0)

	for _, link := range ecfg.Links {
		
		alias := link.Alias
		resp, err := client.Get("/_etcdocker/service/"+link.Name, false, false)
		if err != nil {
			continue
		}
		if resp.Node.Dir {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("%s: is a directory", resp.Node.Key))
			os.Exit(1)
		}
		res := &nameInfo{}
		json.Unmarshal([]byte(resp.Node.Value), &res)

		for _, portBinding := range res.PortBindings {
			cPort := strings.Split(portBinding.ContainerPort, "/")

			port := cPort[0]
			proto := cPort[1]

			for _, publicPort := range portBinding.PublicPorts {
				hostIp := publicPort.HostIp
				hostPort := publicPort.HostPort
				envs = append(envs, "-e")
				envs = append(envs, fmt.Sprintf("%s_PORT=%s://%s:%s", alias, proto, hostIp, hostPort))
				envs = append(envs, "-e")
				envs = append(envs, fmt.Sprintf("%s_PORT_%s_%s=%s://%s:%s", alias, port,
					strings.ToUpper(proto), proto, hostIp, hostPort))
				envs = append(envs, "-e")
				envs = append(envs, fmt.Sprintf("%s_PORT_%s_%s_ADDR=%s", alias, port, strings.ToUpper(proto), hostIp))
				envs = append(envs, "-e")
				envs = append(envs, fmt.Sprintf("%s_PORT_%s_%s_PORT=%s", alias, port, strings.ToUpper(proto), hostPort))
				envs = append(envs, "-e")
				envs = append(envs, fmt.Sprintf("%s_PORT_%s_%s_PROTO=%s", alias, port, strings.ToUpper(proto), proto))
			}
		}
	}
	return envs, nil
}

func setDockerName(ecfg *EtcdDockerConfig) error {

	client := newClient(ecfg)

	b, err := json.Marshal(ecfg.NameInfo)
	if err != nil {
		return err
	}
	if _, err := client.Set("/_etcdocker/service/"+ecfg.Name, string(b), uint64(0)); err != nil {
		return err
	}
	return nil
}

func checkArgs(cli *client.DockerCli, args ...string) (int, *EtcdDockerConfig, error) {
	if len(args) > 0 {
		if args[0] == "run" {
			return checkRunArgs(cli, args[1:]...)
		} else {
			return len(args), nil, nil
		}
	}
	return 0, nil, nil
}

func checkRunArgs(cli *client.DockerCli, args ...string) (int, *EtcdDockerConfig, error) {
	config, hostConfig, cmd, err := runconfig.ParseSubcommand(cli.Subcmd("run", "[OPTIONS] IMAGE [COMMAND] [ARG...]", "Run a command in a new container"), args, nil)

	if err != nil {
		return 0, nil, err
	}

	var (
		extArg       = 0
		flName       = cmd.Lookup("name")
		image        = config.Image
		peer         = config.Peer
		endpoint     = config.Endpoint
		links        = hostConfig.Links
		portBindings = hostConfig.PortBindings
	)

	name := flName.Value.String()

	ecfg := &EtcdDockerConfig{
		Name:     name,
		Endpoint: endpoint,
		Sync:     true,
		NameInfo: nameInfo{
			Name:         name,
			HostIp:       peer,
			PortBindings: make([]portBinding, len(portBindings)),
		},
		Links: make([]linkInfo, len(links)),
	}

	if peer == "" {
		ip, err := localIP()

		if err != nil {
			return 0, nil, err
		}
		ecfg.NameInfo.HostIp = ip
	}

	if len(links) > 0 {
		for i, linkArg := range links {
			link := strings.Split(linkArg, ":")
			ecfg.Links[i] = linkInfo{
				Name:  link[0],
				Alias: link[1],
			}
		}
	}

	idx := 0

	for cPort, ports := range portBindings {
		portBinding := portBinding{
			ContainerPort: string(cPort),
			PublicPorts:   make([]publicPort, len(ports)),
		}
		ecfg.NameInfo.PortBindings[idx] = portBinding

		for i, p := range ports {
			var h string
			if p.HostIp != "" {
				h = p.HostIp
			} else {
				h = ecfg.NameInfo.HostIp
			}
			portBinding.PublicPorts[i] = publicPort{
				HostIp:   h,
				HostPort: p.HostPort,
			}
		}
		idx++
	}

	if peer != "" {
		extArg += 2
	}

	if endpoint != "" {
		extArg += 2
	}

	if image != "" && 
		(name != "" && len(portBindings) > 0) ||
		(len(links) > 0){
		_ = newClient(ecfg)
	}

	if image != "" && len(links) > 0 {

		envs, err := getDockerName(ecfg)
		if err != nil {
			return 0, nil, err
		}
		ecfg.Envs = envs
	}

	return len(args) + 1 - extArg, ecfg, nil
}
