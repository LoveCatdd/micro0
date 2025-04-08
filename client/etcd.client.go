package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"micro0/internal/model"
	"net/http"
	"sync"
	"time"

	"github.com/LoveCatdd/util/pkg/lib/core/log"
	"github.com/gin-gonic/gin"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceInstance struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	InstanceID  string `json:"instance_id"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Status      string `json:"status"` // healthy/unhealthy
}

type ServiceRegistry struct {
	client *clientv3.Client
	mu     sync.Mutex
}

type SrvLeaseMap map[string]clientv3.LeaseID

var srvLeaseMap SrvLeaseMap = make(SrvLeaseMap)

func NewServiceRegistry(endpoints []string) (*ServiceRegistry, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		log.Error(err)
	}
	return &ServiceRegistry{client: client}, err
}

func (r *ServiceRegistry) Registry(si *ServiceInstance, ttl int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf(Key, si.ServiceName)
	data, err := json.Marshal(si)
	if err != nil {
		return err
	}

	// 创建租约
	leaseResp, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	srvLeaseMap[key] = leaseResp.ID // 将租约ID存储在映射中

	// 注册服务并绑定租约
	_, err = r.client.Put(ctx, key, string(data), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return err
	}

	log.Infof("Registered service: %s", key)

	return nil
}

func (r *ServiceRegistry) HealthCheck(service model.Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf(Key, service.Name)
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	if resp.Count == 0 {
		return fmt.Errorf("service %s not found", service.Name)
	}

	kv := resp.Kvs[0]
	var si ServiceInstance
	if err := json.Unmarshal(kv.Value, &si); err != nil {
		return err
	}
	si.Status = "healthy"

	// 重新编码为 JSON
	updatedValue, err := json.Marshal(si)
	if err != nil {
		return err
	}

	// 更新服务健康状态
	r.client.Put(ctx, key, string(updatedValue), clientv3.WithLease(srvLeaseMap[key]))

	log.Infof("Service %s is healthy", service.Name)
	return nil
}

// 心跳检测
func (r *ServiceRegistry) Heartbeat(service model.Service) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 假设你之前注册服务时已经保存了 LeaseID
	leaseID := srvLeaseMap[fmt.Sprintf(Key, service.Name)]
	if leaseID == 0 {
		return fmt.Errorf("lease ID not found for service: %s", service.Name)
	}

	// 检查服务是否健康
	if resp, err := r.RemoteFunc(ctx, service.Name, http.MethodGet, "/health", nil); err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	} else {
		// 解析出resp.body json:{"code":0,"codeName":"success","message":"","resp":{"health":true},"url":""}

		var healthResp map[string]bool
		if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
			return fmt.Errorf("failed to decode health response: %w", err)
		}
		if !healthResp["health"] {
			return fmt.Errorf("service %s is unhealthy", service.Name)
		}
	}

	// 向 etcd 发送 keep-alive 续租请求
	_, err := r.client.KeepAliveOnce(ctx, leaseID)
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}

	return nil
}

func (r *ServiceRegistry) StartHeartbeat(ctx context.Context, service model.Service, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := r.Heartbeat(service)
				if err != nil {
					log.Infof("[Heartbeat ERROR] service: %s, err: %v", service.Name, err)
					delete(srvLeaseMap, fmt.Sprintf(Key, service.Name)) // 删除服务的租约ID
					return
				}
			case <-ctx.Done():
				log.Infof("[Heartbeat STOPPED] service: %s", service.Name)
				return
			}
		}
	}()
}

// 路由分发， gateway 作为网关，转发请求到对应的服务 service-a:{ip:localhost, port:8081}
// 通过 etcd 注册服务，网关根据请求的路径和方法来决定转发到哪个服务
// 如： http://localhost:8080/service/service-a/api/v1/func-a => http://localhost:8081/api/v1/func-a
func RouteHandler(c *gin.Context) {
	// // 获取请求的路径和方法
	// path := c.Request.URL.Path
	// method := c.Request.Method

	// // 根据路径和方法决定转发到哪个服务
	// var serviceName string
	// switch {
	// case path == "/service/service-a/api/v1/func-a" && method == "POST":
	// 	serviceName = "service-a"
	// case path == "/service/service-b/api/v1/func-b" && method == "GET":
	// 	serviceName = "service-b"
	// default:
	// 	c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
	// 	return
	// }

	// // 调用远程服务的函数，传递请求参数
	// resp, err := RemoteFunc(c, serviceName, method, c.Request.Body)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	return
	// }

	// c.JSON(http.StatusOK, resp)
}

// // 编写远程调用工具函数：如service-a 调用 service-b 服务中的函数
func (r *ServiceRegistry) RemoteFunc(ctx context.Context, serviceName, method, path string, req map[string]any) (resp *http.Response, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf(Key, serviceName)
	client, _ := r.client.Get(ctx, key, clientv3.WithPrefix())

	if client.Count == 0 {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	kv := client.Kvs[0]
	var si ServiceInstance
	if err := json.Unmarshal(kv.Value, &si); err != nil {
		return nil, err
	}
	resp, err = http.Get(fmt.Sprintf(URL, si.IP, si.Port, path))

	return HttpFunc(ctx, fmt.Sprintf(URL, si.IP, si.Port, path), method, req)
}

// HttpFunc 发送 HTTP 请求，支持 GET 和 POST 方法, 其他请求待实现
func HttpFunc(ctx context.Context, url, method string, req map[string]any) (*http.Response, error) {

	switch method {
	case http.MethodGet:
		return http.Get(url)

	case http.MethodPost:
		var reqBody io.Reader
		if req != nil {
			jsonData, err := json.Marshal(req)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			reqBody = bytes.NewBuffer(jsonData)
		}
		return http.Post(url, "application/json", reqBody)

	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

// 获取所有服务列表
func (r *ServiceRegistry) GetAllServices(ctx context.Context) ([]ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := "/service/"
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	fmt.Printf("服务列表resp: %v", resp)
	var services []ServiceInstance
	for _, kv := range resp.Kvs {
		var si ServiceInstance
		if err := json.Unmarshal(kv.Value, &si); err != nil {
			return nil, err
		}
		services = append(services, si)
	}

	return services, nil
}
