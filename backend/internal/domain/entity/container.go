package entity

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// ContainerNameRegexp: minúsculo, alfanumérico e hífen, 1–40 chars, sem hífen nas pontas.
var ContainerNameRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

const (
	ProtocolHTTP = "http"
	ProtocolTCP  = "tcp"
)

// EnvMap é persistido como jsonb.
type EnvMap map[string]string

func (m EnvMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *EnvMap) Scan(src any) error {
	if src == nil {
		*m = EnvMap{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("EnvMap: tipo inesperado no Scan")
	}
	return json.Unmarshal(b, m)
}

// Container é a definição persistida. Status NÃO é coluna: vem do cluster.
type Container struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"                                          json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_containers_user_name" json:"user_id"`
	Name         string    `gorm:"size:40;not null;uniqueIndex:ux_containers_user_name"          json:"name"`
	Description  string    `gorm:"default:''"                                                    json:"description"`
	Image        string    `gorm:"not null"                                                      json:"image"`
	CPUMillis    int       `gorm:"column:cpu_millis;not null"                                    json:"cpu_millis"`
	MemoryMB     int       `gorm:"column:memory_mb;not null"                                     json:"memory_mb"`
	Port         int       `gorm:"not null;default:0"                                            json:"port"`
	Protocol     string    `gorm:"size:4;not null;default:'http'"                                json:"protocol"`
	NodePort     int       `gorm:"column:node_port;not null;default:0"                           json:"node_port"`
	Env          EnvMap    `gorm:"type:jsonb;not null;default:'{}'"                              json:"env"`
	Command      string    `gorm:"default:''"                                                    json:"command"`
	PersistentGB int       `gorm:"column:persistent_gb;not null;default:0"                       json:"persistent_gb"`
	MountPath    string    `gorm:"column:mount_path;default:''"                                  json:"mount_path"`
	Namespace    string    `gorm:"size:10;not null"                                              json:"namespace"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NamespaceFor deriva o namespace k8s do usuário: "u-" + 8 primeiros chars do UUID.
func NamespaceFor(userID uuid.UUID) string {
	return "u-" + userID.String()[:8]
}
