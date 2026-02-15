# CFF Core 构建指南

基于 mihomo (Clash.Meta) 内核，通过 bind/ 层为 Flutter 客户端提供跨平台绑定。

## 环境要求

- Go >= 1.22
- gomobile（移动端构建）
- xgo（桌面端交叉编译）
- Android NDK r26（Android 构建）
- Xcode（iOS 构建，需 macOS）

## 构建标签

- `with_gvisor` — 启用 gVisor 用户态网络栈（TUN 模式必需）

## 版本注入

通过 ldflags 注入版本信息：

```
-X 'github.com/metacubex/mihomo/constant.Version=VERSION'
-X 'github.com/metacubex/mihomo/constant.BuildTime=TIME'
```

---

## 本地构建

### CLI（调试用）

```bash
go build -tags with_gvisor -o mihomo .
```

### Desktop — C-shared 动态库

macOS (arm64):
```bash
go build -tags with_gvisor -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -buildmode=c-shared \
  -o libclash.dylib ./bind/desktop
```

macOS (amd64):
```bash
CGO_ENABLED=1 GOARCH=amd64 go build -tags with_gvisor -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -buildmode=c-shared \
  -o libclash.dylib ./bind/desktop
```

### Desktop — 交叉编译（xgo）

需要先安装 xgo：
```bash
go install src.techknowlogick.com/xgo@latest
```

Windows (amd64):
```bash
xgo --targets=windows/amd64 -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -tags="with_gvisor" -buildmode=c-shared \
  -out=build/libclash.dll ./bind/desktop
```

Linux (amd64 / arm64):
```bash
xgo --targets=linux/amd64 -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -tags="with_gvisor" -buildmode=c-shared \
  -out=build/libclash.so ./bind/desktop
```

### Mobile — Android (.aar)

安装 gomobile：
```bash
go install golang.org/x/mobile/cmd/gomobile@latest
go get golang.org/x/mobile/bind
gomobile init
```

Android arm64:
```bash
gomobile bind -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -tags="with_gvisor" \
  -o libclash.aar -target=android/arm64 -androidapi 29 \
  -javapkg com.web3jsq \
  github.com/metacubex/mihomo/bind/mobile
```

Android amd64（模拟器）:
```bash
gomobile bind -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -tags="with_gvisor" \
  -o libclash.aar -target=android/amd64 -androidapi 29 \
  -javapkg com.web3jsq \
  github.com/metacubex/mihomo/bind/mobile
```

### Mobile — iOS (.xcframework)

需要 macOS + Xcode：
```bash
gomobile bind -trimpath \
  -ldflags="-X 'github.com/metacubex/mihomo/constant.Version=dev' -w -s" \
  -tags="with_gvisor" \
  -o libclash.xcframework -target=ios \
  github.com/metacubex/mihomo/bind/mobile
```

---

## CI/CD 构建

GitHub Actions 自动构建配置在 `build.yml`，触发条件：
- 手动触发（workflow_dispatch）
- 发布 Release 时自动构建

产物列表：

| 平台 | 产物 | 构建方式 |
|------|------|---------|
| Windows amd64 | `libclash.dll` | xgo c-shared |
| Linux amd64 | `libclash.so` | xgo c-shared |
| Linux arm64 | `libclash.so` | xgo c-shared |
| macOS arm64 | `libclash.dylib` | xgo c-shared |
| macOS amd64 | `libclash.dylib` | xgo c-shared |
| Android arm64 | `libclash.aar` | gomobile bind |
| Android amd64 | `libclash.aar` | gomobile bind |
| iOS | `libclash.xcframework` | gomobile bind |

---

## 导出 API 概览

### Desktop (C-shared)

通过 `bind/desktop/main.go` 导出的 C 函数：

| 函数 | 说明 |
|------|------|
| `SetHomeDir(path)` | 设置工作目录 |
| `SetConfig(path)` | 设置配置文件路径 |
| `VerifyMMDB(path)` | 验证 MMDB 文件 |
| `StartRust(addr)` | 启动 RESTful API 服务 |
| `StartService()` | 启动代理服务 |
| `StopService()` | 停止服务 |
| `IsRunning()` | 查询运行状态 |
| `ReloadConfig()` | 热更新配置 |
| `UpdateConfig(path)` | 切换并重载配置 |
| `GetTrafficNow()` | 获取实时流量 (JSON) |
| `GetTrafficTotal()` | 获取累计流量 (JSON) |
| `ResetTraffic()` | 重置流量统计 |
| `GetMode()` | 获取当前模式 |
| `SetMode(mode)` | 设置模式 (rule/global/direct) |
| `CloseAllConnections()` | 关闭所有连接 |
| `GetConfig()` | 获取当前配置 (JSON) |

### Mobile (gomobile)

通过 `bind/mobile/main.go` 导出，包含 Desktop 所有功能，额外支持：

| 函数 | 说明 |
|------|------|
| `SetProtectHandler(handler)` | 设置 VPN socket 保护回调 |
| `OperateTun(enable, fd, mtu)` | 控制 TUN（传入 VPN FD） |
| `StopTun()` | 停止 TUN |
| `GetProxies()` | 获取所有代理组 (JSON) |
| `ChangeProxy(group, name)` | 切换代理 |
| `GetProxyDelay(name, url, timeout)` | 测试代理延迟 |
| `GetConnections()` | 获取活跃连接 (JSON) |
| `CloseConnection(id)` | 关闭指定连接 |
| `CloseAllConnections()` | 关闭所有连接 |
| `SetLogHandler(handler)` | 设置日志回调 |

gomobile 接口（客户端需实现）：
- `ProtectHandler.Protect(fd int)` — Android VPN socket 保护
- `LogHandler.OnLog(level, payload string)` — 日志事件回调

---

## 客户端集成（web3faster）

### 产物放置路径

| 平台 | 产物 | 客户端路径 |
|------|------|-----------|
| macOS arm64 | libclash.dylib | `macos/Frameworks/libclash.dylib` |
| macOS amd64 | libclash.dylib | `macos/Frameworks/libclash-intel.dylib` |
| Windows | libclash.dll | `windows/core/libclash.dll` |
| Linux | libclash.so | `linux/core/libclash.so` |
| Android | libclash.aar | `android/app/libs/libclash.aar` |

### Desktop FFI 绑定更新

Desktop 编译（c-shared）会同时产出 `.dylib` 和 `libclash.h`。当 API 有变动时需要更新客户端的 Dart FFI 绑定：

```bash
# 1. 编译 desktop 库（产出 libclash.dylib + libclash.h）
CGO_ENABLED=1 go build -tags with_gvisor -buildmode=c-shared -o libclash.dylib ./bind/desktop

# 2. 复制头文件到客户端
cp libclash.h /path/to/web3faster/core/libclash.h

# 3. 复制动态库到客户端
cp libclash.dylib /path/to/web3faster/macos/Frameworks/libclash.dylib

# 4. 在客户端目录重新生成 Dart FFI 绑定
cd /path/to/web3faster
dart run ffigen
```

生成的绑定文件：`lib/clash_generated_bindings.dart`，ffigen 配置在客户端 `pubspec.yaml` 的 `ffigen:` 段。

### Mobile 不需要 FFI

Android/iOS 通过 gomobile 编译，自动生成 Java/ObjC 绑定（如 `com.web3jsq.mobile.Mobile`），客户端 Kotlin/Swift 直接 import 调用，无需头文件和 ffigen。

Dart 层通过 MethodChannel 与原生层通信：
- Desktop: `Dart → dart:ffi → libclash.dylib`
- Mobile: `Dart → MethodChannel → Kotlin/Swift → Mobile.xxx()`

