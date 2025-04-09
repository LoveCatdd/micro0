package lb

import "math/rand"

type RandomModel struct {
	lbType string // 负载均衡算法类型
}

func NewRandomModel() *RandomModel {
	return &RandomModel{
		lbType: "random",
	}
}

func (r *RandomModel) LbType() string {
	return r.lbType
}

func (*RandomModel) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return &ServiceInstance{}
	}
	// 随机算法
	index := rand.Intn(len(instances))
	instance := instances[index]
	return &instance
}
