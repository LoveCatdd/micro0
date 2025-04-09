package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/LoveCatdd/micro0/internal/model"
	"github.com/LoveCatdd/micro0/lb"

	"github.com/LoveCatdd/util/pkg/lib/core/log"
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
	if resp, err := RemoteFunc(r, ctx, service.Name, http.MethodGet, "health", nil, ""); err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	} else {

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

	// Heartbeat
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
					// Heartbeat STOPPED
					return
				}
			case <-ctx.Done():
				log.Infof("[Heartbeat STOPPED] service: %s", service.Name)
				return
			}
		}
	}()

}

// 编写远程调用工具函数：如service-a 调用 service-b 服务中的函数
func RemoteFunc(r *ServiceRegistry, ctx context.Context, serviceName, method, path string, req map[string]any, lbType string) (resp *http.Response, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf(Key, serviceName)
	client, _ := r.client.Get(ctx, key, clientv3.WithPrefix())

	if client.Count == 0 {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	var si []ServiceInstance

	for _, kv := range client.Kvs {
		var instance ServiceInstance
		if err := json.Unmarshal(kv.Value, &instance); err != nil {
			return nil, err
		}
		si = append(si, instance)
	}

	// 调用负载均衡算法&超时重试

	return CallWithRetryFailover(ctx, si, len(si), 1*time.Second, path, method, lbType, req, HttpFunc)
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

// 超时重试
func CallWithRetryFailover(
	ctx context.Context,
	sie []ServiceInstance,
	maxAttempts int,
	timeoutPerTry time.Duration,
	url, method, lbType string, req map[string]any,
	callFunc func(ctx context.Context, url, method string, req map[string]any) (*http.Response, error),
) (*http.Response, error) {
	tried := make(map[string]bool)

	for i := 0; i < maxAttempts; i++ {
		// 负载均衡获得节点
		sc := lb.NewServerClient(
			func() []lb.ServiceInstance {
				var instances []lb.ServiceInstance
				for _, instance := range sie {
					if _, ok := tried[instance.IP]; !ok {
						instances = append(instances, lb.ServiceInstance{
							Address:       fmt.Sprintf(URL, instance.IP, instance.Port, url),
							Weight:        1,
							CurrentWeight: 1,
							Healthy:       true,
						})
					}
				}
				return instances
			}(),
			lbType,
			sie[0].ServiceName,
		)
		var lbSi *lb.ServiceInstance
		for {
			lbSi = sc.Next()

			if lbSi == nil {
				return nil, fmt.Errorf("no available service instance")
			}

			if !tried[lbSi.Address] {
				tried[lbSi.Address] = true
				break
			}
		}

		// 发送请求
		callCtx, cancel := context.WithTimeout(ctx, timeoutPerTry)
		resp, err := callFunc(callCtx, lbSi.Address, method, req)
		cancel()

		if err == nil {
			return resp, err
		}
		log.Errorf("attempt %d failed: %v, instance: %v", i+1, err, lbSi.Address)
	}
	return nil, fmt.Errorf("all attempts failed")
}
