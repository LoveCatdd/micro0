package lb

type ServiceInstance struct {
	Address       string
	Weight        int
	CurrentWeight int
	Healthy       bool
}

type ServerClient struct {
	instances []ServiceInstance // 服务实例列表
	lb        string            // 负载均衡算法
	client    string            // 客户端类型
}

type LoadBalancer interface {
	LbType() string                            // 负载均衡算法类型
	Select([]ServiceInstance) *ServiceInstance // 选择一个服务实例
}

var (
	Roundrobin         LoadBalancer = NewRoundrobinModel()         // 轮询算法
	Random             LoadBalancer = NewRandomModel()             // 随机算法
	LeastConn          LoadBalancer = NewLeastConnModel()          // 最少连接数算法
	WeightedRoundrobin LoadBalancer = NewWeightedRoundrobinModel() // 加权轮询算法
)

// instances: {127.0.0.1:8080, 127.0.0.1:8081}

// lb-new
func NewServerClient(instances []ServiceInstance, lb, client string) *ServerClient {
	return &ServerClient{
		instances: instances,
		lb:        lb,
		client:    client,
	}
}

// lb-check return the one of lb algorithm
func (sc *ServerClient) Next() *ServiceInstance {
	switch sc.lb {
	case Roundrobin.LbType():
		// 轮询算法
		return new(RoundrobinModel).Select(sc.instances)
	case Random.LbType():
		// 随机算法
		return new(RandomModel).Select(sc.instances)
	case LeastConn.LbType():
		// 最少连接数算法
		return new(LeastConnModel).Select(sc.instances)
	case WeightedRoundrobin.LbType():
		// 加权轮询算法
		return new(WeightedRoundrobinModel).Select(sc.instances)
	default:
		return &sc.instances[0]
	}
}

// 超时重试
func (sc *ServerClient) Retry() *ServiceInstance {
	return sc.Next()
}
