package web

import (
	"context"
	"encoding/base64"
	"html/template"
	"net"
	"sort"
	"strings"

	"rsc.io/qr"
)

// lanURLs lista os endereços pelos quais outros aparelhos da rede local
// alcançam o app (ex.: http://192.168.0.10:8080), do mais provável ao menos.
func lanURLs(port string) []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	type candidate struct {
		url  string
		rank int
	}
	var list []candidate
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || isVirtualInterface(iface.Name) {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			list = append(list, candidate{"http://" + net.JoinHostPort(ip.String(), port), ipRank(ip)})
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].rank != list[j].rank {
			return list[i].rank < list[j].rank
		}
		return list[i].url < list[j].url
	})
	urls := make([]string, len(list))
	for i, c := range list {
		urls[i] = c.url
	}
	return urls
}

// ipRank ordena: redes domésticas (192.168.x, 10.x) primeiro; 172.16-31 depois
// (também usada por Docker/VMs); o resto (ex.: Tailscale 100.x) por último.
func ipRank(ip net.IP) int {
	switch {
	case ip[0] == 192 && ip[1] == 168:
		return 0
	case ip[0] == 10:
		return 1
	case ip.IsPrivate():
		return 2
	default:
		return 3
	}
}

// isVirtualInterface ignora redes internas de Docker, VMs e afins, que o
// celular não alcança.
func isVirtualInterface(name string) bool {
	for _, prefix := range []string{"docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "cni", "flannel", "podman"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// LANURLs são os endereços para abrir o app em outro aparelho da rede.
func (s *Server) LANURLs() []string { return lanURLs(s.port) }

// HasPassword diz se há senha configurada para o acesso pela rede.
func (s *Server) HasPassword(ctx context.Context) bool {
	hash, err := s.passwordHash(ctx)
	return err == nil && hash != ""
}

// qrDataURI gera o QR code do texto como imagem embutida (data URI).
func qrDataURI(text string) template.URL {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return ""
	}
	code.Scale = 6
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG()))
}
