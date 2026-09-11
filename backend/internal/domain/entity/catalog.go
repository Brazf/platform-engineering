package entity

// CatalogImage é uma imagem curada. Port 0 = não expor (só terminal).
type CatalogImage struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Image       string   `json:"image"`
	Port        int      `json:"port"`
	Protocol    string   `json:"protocol"`
	MountPath   string   `json:"mount_path"`
	Command     string   `json:"command"`
	RequiredEnv []string `json:"required_env"`
}

var catalog = []CatalogImage{
	{Key: "nginx", Label: "Nginx", Image: "nginx:1.27", Port: 80, Protocol: ProtocolHTTP, MountPath: "/usr/share/nginx/html"},
	{Key: "python", Label: "Python 3.12", Image: "python:3.12", Port: 8000, Protocol: ProtocolHTTP, MountPath: "/data"},
	{Key: "node", Label: "Node.js 22", Image: "node:22", Port: 3000, Protocol: ProtocolHTTP, MountPath: "/data"},
	{Key: "postgres", Label: "PostgreSQL 16", Image: "postgres:16", Port: 5432, Protocol: ProtocolTCP, MountPath: "/var/lib/postgresql/data", RequiredEnv: []string{"POSTGRES_PASSWORD"}},
	{Key: "jupyter", Label: "Jupyter Notebook", Image: "jupyter/minimal-notebook:latest", Port: 8888, Protocol: ProtocolHTTP, MountPath: "/home/jovyan"},
	{Key: "ubuntu", Label: "Ubuntu 24.04 (só terminal)", Image: "ubuntu:24.04", Port: 0, MountPath: "/data", Command: "sleep infinity"},
}

func Catalog() []CatalogImage { return catalog }

func CatalogByKey(key string) (CatalogImage, bool) {
	for _, c := range catalog {
		if c.Key == key {
			return c, true
		}
	}
	return CatalogImage{}, false
}
