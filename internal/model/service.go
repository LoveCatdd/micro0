package model

type Service struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	InstanceID string `json:"instance_id"`
	IP         string `json:"ip"`
	Port       int    `json:"port"`
}
