//go:build android && cmfa

package route

func init() {
	// 不启用 embed mode，我们的客户端需要通过 REST API 管理配置
	// SetEmbedMode(true)
}
