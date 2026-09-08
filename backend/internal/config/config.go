package config

import (
	"fmt"
	"os"
	"strconv"

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
	ProxmoxHost        string
	ProxmoxNode        string
	ProxmoxTokenID     string
	ProxmoxTokenSecret string
	ProxmoxGPUPCI      string // ex: "0000:01:00.0,pcie=1" — vazio desabilita passthrough
	ProxmoxUbuntuTemplateVMID int // VMID do template cloud-init Ubuntu clonado para VMs Ubuntu

	// Terraform
	TerraformBin     string // caminho do binário terraform. Default: "terraform" (resolve via PATH)
	TerraformWorkdir string // raiz dos workspaces por VM

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

	cfg := &Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "proxlab"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "proxlab"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpiryHours:     jwtExpiry,
		ProxmoxHost:        getEnv("PROXMOX_HOST", ""),
		ProxmoxNode:        getEnv("PROXMOX_NODE", "pve"),
		ProxmoxTokenID:     getEnv("PROXMOX_TOKEN_ID", ""),
		ProxmoxTokenSecret: getEnv("PROXMOX_TOKEN_SECRET", ""),
		ProxmoxGPUPCI:      getEnv("PROXMOX_GPU_PCI", ""),
		ProxmoxUbuntuTemplateVMID: ubuntuTemplate,
		TerraformBin:       getEnv("TERRAFORM_BIN", "terraform"),
		TerraformWorkdir:   getEnv("TERRAFORM_WORKDIR", "/var/lib/proxlab/terraform"),
		ServerPort:         getEnv("SERVER_PORT", "8080"),
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
		if err := db.AutoMigrate(&entity.User{}, &entity.VM{}); err != nil {
			return nil, fmt.Errorf("erro no auto-migrate: %w", err)
		}
	}

	return db, nil
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
