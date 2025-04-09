package lb

type WeightedRoundrobinModel struct {
	lbType string // 负载均衡算法类型

}

func NewWeightedRoundrobinModel() *WeightedRoundrobinModel {
	return &WeightedRoundrobinModel{
		lbType: "weightedroundrobin",
	}
}

func (w *WeightedRoundrobinModel) LbType() string {
	return w.lbType
}

func (*WeightedRoundrobinModel) Select(instances []ServiceInstance) *ServiceInstance {
	return &ServiceInstance{}
}
