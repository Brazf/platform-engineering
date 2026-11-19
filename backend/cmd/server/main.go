package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/proxlab/backend/internal/config"
	domainrepo "github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/handler"
	"github.com/proxlab/backend/internal/kube"
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
	containerRepo := repository.NewContainerRepository(db)

	proxmoxClient := proxmox.NewProxmoxClient(cfg)
	provisioner, err := terraform.NewRunner(cfg)
	if err != nil {
		log.Fatalf("erro ao iniciar terraform runner: %v", err)
	}
	var containerRuntime domainrepo.ContainerRuntime
	if rt, err := kube.New(cfg.KubeconfigPath); err != nil {
		log.Printf("[kube] cluster de containers indisponível (%v); rotas de containers responderão 503", err)
	} else {
		containerRuntime = rt
	}

	authUC := usecase.NewAuthUseCase(userRepo, cfg.JWTSecret, cfg.JWTExpiryHours)
	profileUC := usecase.NewProfileUseCase(userRepo)
	vmUC := usecase.NewVMUseCase(vmRepo, proxmoxClient, provisioner, cfg.ProxmoxNode, cfg.ProxmoxUbuntuTemplateVMID,
		map[string]string{"aluno": cfg.ProxmoxPoolAluno, "pesquisador": cfg.ProxmoxPoolPesquisador},
		cfg.ProxmoxGPUMappings)
	containerUC := usecase.NewContainerUseCase(containerRepo, containerRuntime, cfg.IngressDomain, cfg.K3SNodeHost())
	userAdminUC := usecase.NewUserAdminUseCase(userRepo, vmRepo, containerRepo, vmUC, containerUC)

	authH := handler.NewAuthHandler(authUC)
	profileH := handler.NewProfileHandler(profileUC)
	userAdminH := handler.NewUserAdminHandler(userAdminUC)
	vmH := handler.NewVMHandler(vmUC)
	containerH := handler.NewContainerHandler(containerUC)
	containerTermH := wshandler.NewContainerTerminalHandler(containerUC, containerRuntime)
	containerLogsH := wshandler.NewContainerLogsHandler(containerUC, containerRuntime)
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
	}

	protected := r.Group("/api")
	protected.Use(middleware.Auth(cfg.JWTSecret))
	{
		// Usuários: só pesquisadores cadastram (alunos concluem no primeiro acesso), listam,
		// trocam e-mail e excluem (a exclusão apaga as VMs e containers da pessoa).
		users := protected.Group("/users", middleware.RequireRole("pesquisador"))
		{
			users.POST("", authH.Register)
			users.GET("", userAdminH.List)
			users.PATCH("/:id", userAdminH.UpdateEmail)
			users.DELETE("/:id", userAdminH.Delete)
		}

		// Meu perfil: qualquer usuário logado lê/edita os próprios dados (e-mail é fixo).
		protected.GET("/me", profileH.Get)
		protected.PATCH("/me", profileH.Update)
		protected.PUT("/me/password", profileH.ChangePassword)

		vms := protected.Group("/vms")
		{
			vms.POST("", vmH.Create)
			vms.GET("", vmH.List)
			vms.GET("/:id", vmH.Get)
			vms.POST("/:id/start", vmH.Start)
			vms.POST("/:id/stop", vmH.Stop)
			vms.POST("/:id/reboot", vmH.Reboot)
			vms.DELETE("/:id", vmH.Delete)
			vms.GET("/:id/terminal", terminalH.HandleTerminal)
		}
		containers := protected.Group("/containers")
		{
			containers.GET("/catalog", containerH.Catalog)
			containers.POST("", containerH.Create)
			containers.GET("", containerH.List)
			containers.GET("/:id", containerH.Get)
			containers.DELETE("/:id", containerH.Delete)
			containers.GET("/:id/terminal", containerTermH.Handle)
			containers.GET("/:id/logs", containerLogsH.Handle)
		}
	}

	// 5. Start
	log.Printf("servidor iniciado na porta %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("erro ao iniciar servidor: %v", err)
	}
}
