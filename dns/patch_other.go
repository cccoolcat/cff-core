//go:build !android || !cmfa

package dns

import (
	"github.com/metacubex/mihomo/component/resolver"
)

func FlushCacheWithDefaultResolver() {
	resolver.ClearCache()
	resolver.ResetConnection()
}

func UpdateSystemDNS(addr []string) {
	// no-op on non-Android platforms
}
