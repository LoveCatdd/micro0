package micro0

import (
	"context"
	"net/http"

	"github.com/LoveCatdd/micro0/client"
	"github.com/gin-gonic/gin"
)

// HealthDefaultApi 默认健康检查接口
func HealthDefaultApi(r *gin.RouterGroup) {
	r.GET("health", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]bool{
			"health": true,
		})
	})
}

var Registry *client.ServiceRegistry

// // 编写远程调用工具函数：如service-a 调用 service-b 服务中的函数
func RemoteFunc(ctx context.Context, serviceName, method, path string, req map[string]any) (resp *http.Response, err error) {
	return client.RemoteFunc(Registry, ctx, serviceName, method, path, req)
}
