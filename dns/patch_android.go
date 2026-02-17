//go:build android && cmfa

package dns

import (
	"github.com/metacubex/mihomo/component/resolver"
)

var systemResolver []dnsClient

func init() {
	// 默认初始化系统 DNS，避免 resolver 为 nil 导致所有连接失败
	UpdateSystemDNS([]string{"223.5.5.5:53", "119.29.29.29:53", "8.8.8.8:53"})
}

func FlushCacheWithDefaultResolver() {
	resolver.ClearCache()
	resolver.ResetConnection()
}

func UpdateSystemDNS(addr []string) {
	if len(addr) == 0 {
		systemResolver = nil
	}

	ns := make([]NameServer, 0, len(addr))
	for _, d := range addr {
		ns = append(ns, NameServer{Addr: d})
	}

	systemResolver = transform(ns, nil)
}

func (c *systemClient) getDnsClients() ([]dnsClient, error) {
	return systemResolver, nil
}

func (c *systemClient) ResetConnection() {
	for _, r := range systemResolver {
		r.ResetConnection()
	}
}
