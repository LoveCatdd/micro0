package client

import "github.com/LoveCatdd/util/pkg/lib/core/viper"

const (
	Key = "/service/%s"     // /service/{name}
	URL = "http://%s:%d/%v" // http://{ip}:{port}/{path}
)

type EtcdConf struct {
	Etcd struct {
		Server []struct {
			Port string `mapstructure:"port"`
			Ip   string `mapstructure:"ip"`
		} `mapstructure:"server"`
	} `mapstructure:"etcd"`
}

func (s *EtcdConf) FileType() string {
	return viper.VIPER_YAML
}

var Etcd = new(EtcdConf)
