package lb

type RoundrobinModel struct {
	lbType string // 负载均衡算法类型

}

func NewRoundrobinModel() *RoundrobinModel {
	return &RoundrobinModel{
		lbType: "roundrobin",
	}
}

func (r *RoundrobinModel) LbType() string {
	return r.lbType
}

func (*RoundrobinModel) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return &ServiceInstance{}
	}
	// 轮询算法
	instance := instances[0]
	instances = append(instances[1:], instance)
	return &instance
}
