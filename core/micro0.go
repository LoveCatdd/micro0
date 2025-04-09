package micro0

import (
	"context"
	"net/http"
	"os"

	"github.com/LoveCatdd/micro0/client"
	"github.com/LoveCatdd/util/pkg/lib/core/viper"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// HealthDefaultApi 默认健康检查接口
func HealthDefaultApi(r *gin.RouterGroup) {
	r.GET("health", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]bool{
			"health": true,
		})
	})
}

// // 编写远程调用工具函数：如service-a 调用 service-b 服务中的函数
func RemoteFunc(ctx context.Context, serviceName, method, path string, req map[string]any, lb string) (resp *http.Response, err error) {

	return client.RemoteFunc(Registry, ctx, serviceName, method, path, req, lb)
}

var Registry *client.ServiceRegistry
var EtcdServer []string

func init() {

	godotenv.Load("../.env")

	environ := os.Getenv("environment")
	viper.SetEnviro(environ)

	viper.Yaml(client.Etcd)

	for _, server := range client.Etcd.Etcd.Server {
		EtcdServer = append(EtcdServer, server.Ip+":"+server.Port)
	}

	Registry, _ = client.NewServiceRegistry(EtcdServer)
}
