package lb

type LeastConnModel struct {
	lbType string // 负载均衡算法类型
}

func NewLeastConnModel() *LeastConnModel {
	return &LeastConnModel{
		lbType: "leastconn",
	}
}

func (l *LeastConnModel) LbType() string {
	return l.lbType
}

func (*LeastConnModel) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return &ServiceInstance{}
	}
	// 最少连接数算法
	minConn := instances[0].CurrentWeight
	instance := instances[0]
	for _, inst := range instances {
		if inst.CurrentWeight < minConn {
			minConn = inst.CurrentWeight
			instance = inst
		}
	}
	return &instance
}
