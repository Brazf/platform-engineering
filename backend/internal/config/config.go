package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/proxlab/backend/internal/domain/entity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	// Aplicação
	AppEnv string // "development" | "production"

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// JWT
	JWTSecret      string
	JWTExpiryHours int

	// Proxmox
	ProxmoxHost               string
	ProxmoxNode               string
	ProxmoxTokenID            string
	ProxmoxTokenSecret        string
	ProxmoxGPUPCI             string // ex: "0000:01:00.0,pcie=1" — vazio desabilita passthrough
	ProxmoxUbuntuTemplateVMID int    // VMID do template cloud-init Ubuntu clonado para VMs Ubuntu

	// Terraform
	TerraformBin     string // caminho do binário terraform. Default: "terraform" (resolve via PATH)
	TerraformWorkdir string // raiz dos workspaces por VM

	// k3s / containers
	K3SNodeIP       string // IP fixo da VM do cluster em CIDR, ex: "10.0.0.50/24"
	K3SNodeGateway  string
	K3SNodePool     string // resource pool do Proxmox onde a VM do cluster nasce
	K3SNodeCPU      int
	K3SNodeMemoryMB int
	K3SNodeDiskGB   int
	IngressDomain   string // vazio → containers HTTP expostos por NodePort
	KubeconfigPath  string // default: <TerraformWorkdir>/k3s/kubeconfig

	// Server
	ServerPort string
}

func Load() (*Config, error) {
	// Ignora erro caso .env não exista (produção usa variáveis do sistema)
	_ = godotenv.Load()

	jwtExpiry, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	if err != nil {
		return nil, fmt.Errorf("JWT_EXPIRY_HOURS inválido: %w", err)
	}
	ubuntuTemplate, err := strconv.Atoi(getEnv("PROXMOX_UBUNTU_TEMPLATE_VMID", "9000"))
	if err != nil {
		return nil, fmt.Errorf("PROXMOX_UBUNTU_TEMPLATE_VMID inválido: %w", err)
	}

	k3sCPU, err := strconv.Atoi(getEnv("K3S_NODE_CPU", "8"))
	if err != nil {
		return nil, fmt.Errorf("K3S_NODE_CPU inválido: %w", err)
	}
	k3sMem, err := strconv.Atoi(getEnv("K3S_NODE_MEMORY_MB", "16384"))
	if err != nil {
		return nil, fmt.Errorf("K3S_NODE_MEMORY_MB inválido: %w", err)
	}
	k3sDisk, err := strconv.Atoi(getEnv("K3S_NODE_DISK_GB", "200"))
	if err != nil {
		return nil, fmt.Errorf("K3S_NODE_DISK_GB inválido: %w", err)
	}
	tfWorkdir := getEnv("TERRAFORM_WORKDIR", "/var/lib/proxlab/terraform")

	cfg := &Config{
		AppEnv:                    getEnv("APP_ENV", "development"),
		DBHost:                    getEnv("DB_HOST", "localhost"),
		DBPort:                    getEnv("DB_PORT", "5432"),
		DBUser:                    getEnv("DB_USER", "proxlab"),
		DBPassword:                getEnv("DB_PASSWORD", ""),
		DBName:                    getEnv("DB_NAME", "proxlab"),
		JWTSecret:                 getEnv("JWT_SECRET", ""),
		JWTExpiryHours:            jwtExpiry,
		ProxmoxHost:               getEnv("PROXMOX_HOST", ""),
		ProxmoxNode:               getEnv("PROXMOX_NODE", "pve"),
		ProxmoxTokenID:            getEnv("PROXMOX_TOKEN_ID", ""),
		ProxmoxTokenSecret:        getEnv("PROXMOX_TOKEN_SECRET", ""),
		ProxmoxGPUPCI:             getEnv("PROXMOX_GPU_PCI", ""),
		ProxmoxUbuntuTemplateVMID: ubuntuTemplate,
		TerraformBin:              getEnv("TERRAFORM_BIN", "terraform"),
		TerraformWorkdir:          tfWorkdir,
		K3SNodeIP:                 getEnv("K3S_NODE_IP", ""),
		K3SNodeGateway:            getEnv("K3S_NODE_GATEWAY", ""),
		K3SNodePool:               getEnv("K3S_NODE_POOL", "researchers"),
		K3SNodeCPU:                k3sCPU,
		K3SNodeMemoryMB:           k3sMem,
		K3SNodeDiskGB:             k3sDisk,
		IngressDomain:             getEnv("INGRESS_DOMAIN", ""),
		KubeconfigPath:            getEnv("KUBECONFIG_PATH", filepath.Join(tfWorkdir, "k3s", "kubeconfig")),
		ServerPort:                getEnv("SERVER_PORT", "8080"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ConnectDatabase abre a conexão GORM com PostgreSQL.
// Em desenvolvimento, executa auto-migrate das entidades de domínio.
func (c *Config) ConnectDatabase() (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(c.DSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}

	if c.AppEnv == "development" {
		if err := db.AutoMigrate(&entity.User{}, &entity.VM{}, &entity.Container{}); err != nil {
			return nil, fmt.Errorf("erro no auto-migrate: %w", err)
		}
		// GORM não gera índice parcial via tag.
		if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_containers_node_port ON containers(node_port) WHERE node_port > 0`).Error; err != nil {
			return nil, fmt.Errorf("erro ao criar índice de node_port: %w", err)
		}
	}

	return db, nil
}

// K3SNodeHost devolve o IP sem o prefixo CIDR ("10.0.0.50/24" → "10.0.0.50").
func (c *Config) K3SNodeHost() string {
	host, _, _ := strings.Cut(c.K3SNodeIP, "/")
	return host
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=America/Recife",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func (c *Config) validate() error {
	required := map[string]string{
		"JWT_SECRET":           c.JWTSecret,
		"PROXMOX_HOST":         c.ProxmoxHost,
		"PROXMOX_TOKEN_ID":     c.ProxmoxTokenID,
		"PROXMOX_TOKEN_SECRET": c.ProxmoxTokenSecret,
	}
	for key, val := range required {
		if val == "" {
			return fmt.Errorf("variável de ambiente obrigatória ausente: %s", key)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
