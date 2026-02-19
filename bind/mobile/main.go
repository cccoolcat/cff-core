package mobile

import (
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/adapter/outboundgroup"
	"github.com/metacubex/mihomo/component/dialer"
	"github.com/metacubex/mihomo/component/mmdb"
	"github.com/metacubex/mihomo/component/resolver"
	"github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/dns"
	"github.com/metacubex/mihomo/hub/executor"
	"github.com/metacubex/mihomo/hub/route"
	"github.com/metacubex/mihomo/listener"
	LC "github.com/metacubex/mihomo/listener/config"
	"github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/tunnel"
	"github.com/metacubex/mihomo/tunnel/statistic"
	"go.uber.org/automaxprocs/maxprocs"
)

// status service status
var status = false

// logHandler stores the current log handler
var logHandler LogHandler
var logDone chan struct{}

func init() {
	_, _ = maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))
}

// ProtectHandler is the callback interface for Android VPN socket protection.
// The client (Java/Kotlin) implements this interface and passes it via SetProtectHandler.
type ProtectHandler interface {
	Protect(fd int)
}

// LogHandler is the callback interface for log event forwarding.
type LogHandler interface {
	OnLog(level, payload string)
}

// ========== Setup & Lifecycle ==========

func SetHomeDir(homeDir string) bool {
	info, err := os.Stat(homeDir)
	if err != nil {
		log.Errorln("[CFF Lib] SetHomeDir: %s : %+v", homeDir, err)
		return false
	}
	if !info.IsDir() {
		log.Errorln("[CFF Lib] SetHomeDir: Path is not directory %s", homeDir)
		return false
	}
	constant.SetHomeDir(homeDir)
	return true
}

func SetConfig(configFile string) bool {
	if configFile == "" {
		return false
	}
	if !filepath.IsAbs(configFile) {
		configFile = filepath.Join(constant.Path.HomeDir(), configFile)
	}
	constant.SetConfig(configFile)
	return true
}

func VerifyMMDB(path string) bool {
	return mmdb.Verify(path)
}

func StartRust(addr string) string {
	route.ReCreateServer(&route.Config{
		Addr: addr,
	})
	return addr
}

func StartService() bool {
	if status {
		return status
	}
	if constant.Path.Config() == "config.yaml" {
		configFile := filepath.Join(constant.Path.HomeDir(), constant.Path.Config())
		constant.SetConfig(configFile)
	}
	cfg, err := executor.Parse()
	if err != nil {
		log.Errorln("[CFF Lib] StartService: Parse config error: %+v", err)
		return status
	}
	executor.ApplyConfig(cfg, true)
	status = true
	return status
}

func StopService() {
	listener.Cleanup()
	status = false
}

// ========== Socket Protection (Android VPN) ==========

// SetProtectHandler sets the socket protection callback for Android VPN.
// This prevents routing loops by letting the VPN service protect sockets.
func SetProtectHandler(handler ProtectHandler) {
	if handler == nil {
		dialer.DefaultSocketHook = nil
		return
	}
	dialer.DefaultSocketHook = func(network, address string, conn syscall.RawConn) error {
		return conn.Control(func(fd uintptr) {
			handler.Protect(int(fd))
		})
	}
}

// ========== TUN Control ==========

func OperateTun(enable bool, fileDescriptor, mtu int32) {
	tunConf := LC.Tun{
		Enable:              enable,
		Device:              "web3jsq",
		Stack:               constant.TunGvisor,
		DNSHijack:           []string{"0.0.0.0:53"},
		AutoRoute:           false,
		AutoDetectInterface: false,
		Inet4Address:        []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30")},
		MTU:                 uint32(mtu),
		FileDescriptor:      int(fileDescriptor),
	}
	log.Infoln("[Mobile] OperateTun enable=%v fd=%d mtu=%d", enable, fileDescriptor, mtu)
	listener.ReCreateTun(tunConf, tunnel.Tunnel)
	// TUN 创建后重置 DNS 连接，确保走新的网络路径
	if enable {
		resolver.ResetConnection()
	}
}

func StopTun() {
	tunConf := LC.Tun{Enable: false}
	listener.ReCreateTun(tunConf, tunnel.Tunnel)
}

// ========== DNS Management ==========

// UpdateSystemDNS updates the system DNS servers (comma-separated, e.g. "8.8.8.8:53,1.1.1.1:53")
func UpdateSystemDNS(dnsAddrs string) {
	if dnsAddrs == "" {
		dns.UpdateSystemDNS([]string{})
		return
	}
	dns.UpdateSystemDNS(strings.Split(dnsAddrs, ","))
	dns.FlushCacheWithDefaultResolver()
}

// ========== Lifecycle ==========

// Suspend pauses/resumes the tunnel (省电：屏幕关闭时暂停)
func Suspend(suspended bool) {
	if suspended {
		tunnel.OnSuspend()
	} else {
		tunnel.OnRunning()
	}
}

// ForceGC forces garbage collection and memory release
func ForceGC() {
	runtime.GC()
	debug.FreeOSMemory()
}

// ========== Config Hot-Reload ==========

func ReloadConfig() bool {
	cfg, err := executor.Parse()
	if err != nil {
		log.Errorln("[CFF Lib] ReloadConfig: Parse error: %+v", err)
		return false
	}
	executor.ApplyConfig(cfg, false)
	return true
}

func UpdateConfig(configPath string) bool {
	if !SetConfig(configPath) {
		return false
	}
	return ReloadConfig()
}

// ========== Traffic Statistics ==========

type trafficData struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// GetTrafficNow returns current traffic speed in bytes/sec as JSON.
func GetTrafficNow() string {
	up, down := statistic.DefaultManager.Now()
	data, _ := json.Marshal(trafficData{Up: up, Down: down})
	return string(data)
}

// GetTrafficTotal returns total accumulated traffic as JSON.
func GetTrafficTotal() string {
	up, down := statistic.DefaultManager.Total()
	data, _ := json.Marshal(trafficData{Up: up, Down: down})
	return string(data)
}

func ResetTraffic() {
	statistic.DefaultManager.ResetStatistic()
}

// ========== Proxy Management ==========

type proxyInfo struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Now     string   `json:"now,omitempty"`
	All     []string `json:"all,omitempty"`
	History []int    `json:"history,omitempty"`
}

// GetProxies returns all proxy groups and their proxies as JSON.
func GetProxies() string {
	proxies := tunnel.Proxies()
	var result []proxyInfo
	for name, proxy := range proxies {
		info := proxyInfo{
			Name: name,
			Type: proxy.Type().String(),
		}
		if adapterProxy, ok := proxy.(*adapter.Proxy); ok {
			if selector, ok := adapterProxy.ProxyAdapter.(outboundgroup.SelectAble); ok {
				if s, ok := selector.(interface{ Now() string }); ok {
					info.Now = s.Now()
				}
			}
			if group, ok := adapterProxy.ProxyAdapter.(interface{ Proxies() []constant.Proxy }); ok {
				for _, p := range group.Proxies() {
					info.All = append(info.All, p.Name())
				}
			}
		}
		result = append(result, info)
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ChangeProxy switches the selected proxy in a group. Returns true on success.
func ChangeProxy(groupName, proxyName string) bool {
	proxies := tunnel.Proxies()
	proxy, ok := proxies[groupName]
	if !ok {
		return false
	}
	adapterProxy, ok := proxy.(*adapter.Proxy)
	if !ok {
		return false
	}
	selector, ok := adapterProxy.ProxyAdapter.(outboundgroup.SelectAble)
	if !ok {
		return false
	}
	return selector.Set(proxyName) == nil
}

// GetProxyDelay tests the delay of a proxy. Returns delay in ms, or -1 on error.
func GetProxyDelay(proxyName, url string, timeout int) int {
	proxies := tunnel.Proxies()
	proxy, ok := proxies[proxyName]
	if !ok {
		return -1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()
	delay, err := proxy.URLTest(ctx, url, nil)
	if err != nil {
		return -1
	}
	return int(delay)
}

// ========== Connection Management ==========

type connectionInfo struct {
	ID       string `json:"id"`
	Network  string `json:"network"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	DstIP    string `json:"dstIP"`
	DstPort  uint16 `json:"dstPort"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	Start    string `json:"start"`
	Chains   string `json:"chains"`
	Rule     string `json:"rule"`
}

// GetConnections returns all active connections as JSON.
func GetConnections() string {
	snapshot := statistic.DefaultManager.Snapshot()
	conns := make([]connectionInfo, 0, len(snapshot.Connections))
	for _, c := range snapshot.Connections {
		chains := ""
		if len(c.Chain) > 0 {
			chains = c.Chain[0]
		}
		conns = append(conns, connectionInfo{
			ID:       c.UUID.String(),
			Network:  c.Metadata.NetWork.String(),
			Type:     c.Metadata.Type.String(),
			Host:     c.Metadata.Host,
			DstIP:    c.Metadata.DstIP.String(),
			DstPort:  c.Metadata.DstPort,
			Upload:   c.UploadTotal.Load(),
			Download: c.DownloadTotal.Load(),
			Start:    c.Start.Format(time.RFC3339),
			Chains:   chains,
			Rule:     c.Rule,
		})
	}
	data, _ := json.Marshal(conns)
	return string(data)
}

func CloseConnection(id string) bool {
	c := statistic.DefaultManager.Get(id)
	if c == nil {
		return false
	}
	return c.Close() == nil
}

func CloseAllConnections() {
	statistic.DefaultManager.Range(func(c statistic.Tracker) bool {
		_ = c.Close()
		return true
	})
}

// ========== Log Forwarding ==========

// SetLogHandler sets a callback for log events. Pass nil to stop.
func SetLogHandler(handler LogHandler) {
	// Stop existing subscription
	if logDone != nil {
		close(logDone)
		logDone = nil
	}
	logHandler = handler
	if handler == nil {
		return
	}
	logDone = make(chan struct{})
	sub := log.Subscribe()
	go func() {
		defer log.UnSubscribe(sub)
		for {
			select {
			case <-logDone:
				return
			case ev, ok := <-sub:
				if !ok {
					return
				}
				if ev.LogLevel < log.Level() {
					continue
				}
				handler.OnLog(ev.Type(), ev.Payload)
			}
		}
	}()
}

// ========== Mode Control ==========

// GetMode returns the current tunnel mode: "rule", "global", or "direct".
func GetMode() string {
	return tunnel.Mode().String()
}

// SetMode sets the tunnel mode. Valid values: "rule", "global", "direct".
func SetMode(mode string) bool {
	switch mode {
	case "rule":
		tunnel.SetMode(tunnel.Rule)
	case "global":
		tunnel.SetMode(tunnel.Global)
	case "direct":
		tunnel.SetMode(tunnel.Direct)
	default:
		return false
	}
	return true
}

// ========== Service Status ==========

func IsRunning() bool {
	return status
}

// GetConfig returns the current general config as JSON.
func GetConfig() string {
	general := executor.GetGeneral()
	data, _ := json.Marshal(general)
	return string(data)
}
