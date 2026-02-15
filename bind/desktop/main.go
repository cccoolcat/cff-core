package main

import "C"
import (
	"encoding/json"
	"github.com/metacubex/mihomo/component/mmdb"
	"github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub/executor"
	"github.com/metacubex/mihomo/hub/route"
	"github.com/metacubex/mihomo/listener"
	"github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/tunnel"
	"github.com/metacubex/mihomo/tunnel/statistic"
	"go.uber.org/automaxprocs/maxprocs"
	"os"
	"path/filepath"
)

// status service status
var status = false

func init() {
	_, _ = maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))
}

//export SetHomeDir
func SetHomeDir(homeStr *C.char) bool {
	homeDir := C.GoString(homeStr)
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

//export SetConfig
func SetConfig(configStr *C.char) bool {
	configFile := C.GoString(configStr)
	if configFile == "" {
		return false
	}
	if !filepath.IsAbs(configFile) {
		configFile = filepath.Join(constant.Path.HomeDir(), configFile)
	}
	constant.SetConfig(configFile)
	return true
}

//export VerifyMMDB
func VerifyMMDB(path *C.char) bool {
	return mmdb.Verify(C.GoString(path))
}

//export StartRust
func StartRust(addr *C.char) *C.char {
	route.ReCreateServer(&route.Config{
		Addr: C.GoString(addr),
	})
	return addr
}

//export StartService
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

//export StopService
func StopService() {
	listener.Cleanup()
	status = false
}

//export IsRunning
func IsRunning() bool {
	return status
}

//export ReloadConfig
func ReloadConfig() bool {
	cfg, err := executor.Parse()
	if err != nil {
		log.Errorln("[CFF Lib] ReloadConfig: Parse error: %+v", err)
		return false
	}
	executor.ApplyConfig(cfg, false)
	return true
}

//export UpdateConfig
func UpdateConfig(configStr *C.char) bool {
	configFile := C.GoString(configStr)
	if configFile == "" {
		return false
	}
	if !filepath.IsAbs(configFile) {
		configFile = filepath.Join(constant.Path.HomeDir(), configFile)
	}
	constant.SetConfig(configFile)
	return ReloadConfig()
}

//export GetTrafficNow
func GetTrafficNow() *C.char {
	up, down := statistic.DefaultManager.Now()
	data, _ := json.Marshal(map[string]int64{"up": up, "down": down})
	return C.CString(string(data))
}

//export GetTrafficTotal
func GetTrafficTotal() *C.char {
	up, down := statistic.DefaultManager.Total()
	data, _ := json.Marshal(map[string]int64{"up": up, "down": down})
	return C.CString(string(data))
}

//export ResetTraffic
func ResetTraffic() {
	statistic.DefaultManager.ResetStatistic()
}

//export GetMode
func GetMode() *C.char {
	return C.CString(tunnel.Mode().String())
}

//export SetMode
func SetMode(modeStr *C.char) bool {
	mode := C.GoString(modeStr)
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

//export CloseAllConnections
func CloseAllConnections() {
	statistic.DefaultManager.Range(func(c statistic.Tracker) bool {
		_ = c.Close()
		return true
	})
}

//export GetConfig
func GetConfig() *C.char {
	general := executor.GetGeneral()
	data, _ := json.Marshal(general)
	return C.CString(string(data))
}

func main() {}
