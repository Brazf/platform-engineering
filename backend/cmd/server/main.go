package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/handler"
	"github.com/proxlab/backend/internal/middleware"
	"github.com/proxlab/backend/internal/proxmox"
	"github.com/proxlab/backend/internal/repository"
	"github.com/proxlab/backend/internal/terraform"
	"github.com/proxlab/backend/internal/usecase"
	wshandler "github.com/proxlab/backend/internal/websocket"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// 1. Configuração
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("erro ao carregar configuração: %v", err)
	}

	// 2. Banco de dados (auto-migrate em desenvolvimento)
	db, err := cfg.ConnectDatabase()
	if err != nil {
		log.Fatalf("%v", err)
	}

	// 3. Dependências (repositórios → use cases → handlers)
	userRepo := repository.NewUserRepository(db)
	vmRepo := repository.NewVMRepository(db)

	proxmoxClient := proxmox.NewProxmoxClient(cfg)
	provisioner, err := terraform.NewRunner(cfg)
	if err != nil {
		log.Fatalf("erro ao iniciar terraform runner: %v", err)
	}

	authUC := usecase.NewAuthUseCase(userRepo, cfg.JWTSecret, cfg.JWTExpiryHours)
	vmUC := usecase.NewVMUseCase(vmRepo, proxmoxClient, provisioner, cfg.ProxmoxNode, cfg.ProxmoxUbuntuTemplateVMID)

	authH := handler.NewAuthHandler(authUC)
	vmH := handler.NewVMHandler(vmUC)
	terminalH := wshandler.NewTerminalHandler(
		proxmoxClient, vmRepo,
		proxmoxClient.WsBase(),
		proxmoxClient.Node(),
		proxmoxClient.AuthHeader(),
	)

	// 4. Roteamento
	r := gin.Default()
	r.Use(middleware.CORS())

	// Documentação (Swagger UI servindo docs/swagger.yaml escrito à mão)
	r.StaticFile("/docs/swagger.yaml", "./docs/swagger.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/swagger.yaml")))

	public := r.Group("/api/auth")
	{
		public.POST("/login", authH.Login)
		public.POST("/register", authH.Register)
	}

	protected := r.Group("/api")
	protected.Use(middleware.Auth(cfg.JWTSecret))
	{
		vms := protected.Group("/vms")
		{
			vms.POST("", vmH.Create)
			vms.GET("", vmH.List)
			vms.GET("/:id", vmH.Get)
			vms.POST("/:id/start", vmH.Start)
			vms.POST("/:id/stop", vmH.Stop)
			vms.DELETE("/:id", vmH.Delete)
			vms.GET("/:id/terminal", terminalH.HandleTerminal)
		}
	}

	// 5. Start
	log.Printf("servidor iniciado na porta %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("erro ao iniciar servidor: %v", err)
	}
}
