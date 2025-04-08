package main

import (
	"os"

	"github.com/LoveCatdd/micro0/client"
	"github.com/LoveCatdd/micro0/internal/api"
	"github.com/LoveCatdd/micro0/internal/middleware"

	"github.com/LoveCatdd/util/pkg/lib/core/log"
	"github.com/LoveCatdd/util/pkg/lib/core/viper"
	"github.com/LoveCatdd/webctx/pkg/lib/core/web/http"
	"github.com/LoveCatdd/webctx/pkg/lib/core/web/server"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var etcdServer []string

func main() {

	http.NewAppEngine(gin.Default())
	registry, err := client.NewServiceRegistry(etcdServer)
	if err != nil {
		log.Fatal(err)
	}

	v1 := http.RootRouterGroup().Group("/api/v1")
	{
		v1.Use(middleware.CorsMiddleware())
		impl := api.ApiImpl{}
		impl.RouterDefaultApi(v1.Group(""), registry)
		impl.HealthDefaultApi(v1.Group(""))
	}

	http.Run()
}

func init() {
	godotenv.Load("../.env")

	environ := os.Getenv("environment")
	viper.SetEnviro(environ)

	viper.Yaml(server.AppConf)

	viper.Yaml(client.Etcd)

	for _, server := range client.Etcd.Etcd.Server {
		etcdServer = append(etcdServer, server.Ip+":"+server.Port)
	}

	viper.Yaml(log.Config)
	if log.Config.Zap.Enable { // 开启

		log.InitZap()

		defer log.Sync()
	}

}
