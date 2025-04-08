package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LoveCatdd/micro0/client"
	"github.com/LoveCatdd/micro0/internal/model"

	"github.com/LoveCatdd/util/pkg/lib/core/log"
	"github.com/LoveCatdd/webctx/pkg/lib/core/web/response"
	"github.com/gin-gonic/gin"
)

type ApiImpl struct{}

func (ApiImpl) RouterDefaultApi(r *gin.RouterGroup, registry *client.ServiceRegistry) {

	r.POST("registry", func(c *gin.Context) {
		var service model.Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.JSON(http.StatusOK,
				response.FailWithMessage(
					response.JSON_UNMARSHAL_FAIL, c.Request.URL.Path, fmt.Sprintf("服务宕机: %v!, err: %v!", service.Name, err.Error()),
				),
			)
			return
		}

		if err := registry.Registry(&client.ServiceInstance{
			ServiceName: service.Name,
			Version:     service.Version,
			InstanceID:  service.InstanceID,
			IP:          service.IP,
			Port:        service.Port,
			Status:      "healthy",
		}, 30); err != nil {
			log.Error("服务注册失败！", err)
			return
		}
		log.Infof("服务注册成功！%v", service)

		registry.StartHeartbeat(context.Background(), service, 10)
	})
}

func (ApiImpl) HealthDefaultApi(r *gin.RouterGroup) {
	r.GET("health", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]bool{
			"health": true,
		})
	})
}
