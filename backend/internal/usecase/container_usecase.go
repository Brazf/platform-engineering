package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
)

var (
	ErrContainerNotFound  = errors.New("container não encontrado")
	ErrContainerNameTaken = errors.New("já existe um container com esse nome")
	ErrInvalidContainer   = errors.New("dados do container inválidos")
)

const (
	nodePortMin      = 30000
	nodePortMax      = 32767
	nodePortRetries  = 5
	defaultMountPath = "/data"
)

type CreateContainerRequest struct {
	Name         string
	Description  string
	Image        string
	CPUMillis    int
	MemoryMB     int
	Port         int
	Protocol     string
	Env          map[string]string
	Command      string
	PersistentGB int
	MountPath    string
	Role         string
}

// ContainerView é a entidade + campos calculados a partir do cluster e da config.
type ContainerView struct {
	*entity.Container
	Status          repository.ContainerStatus `json:"status"`
	ExternalURL     string                     `json:"external_url"`
	InternalAddress string                     `json:"internal_address"`
}

type ContainerUseCase struct {
	repo          repository.ContainerRepository
	runtime       repository.ContainerRuntime // nil → cluster indisponível
	ingressDomain string
	nodeIP        string
}

func NewContainerUseCase(repo repository.ContainerRepository, runtime repository.ContainerRuntime, ingressDomain, nodeIP string) *ContainerUseCase {
	return &ContainerUseCase{repo: repo, runtime: runtime, ingressDomain: ingressDomain, nodeIP: nodeIP}
}

func (uc *ContainerUseCase) Create(ctx context.Context, userID uuid.UUID, req CreateContainerRequest) (*ContainerView, error) {
	if uc.runtime == nil {
		return nil, repository.ErrRuntimeUnavailable
	}
	if err := uc.validate(req); err != nil {
		return nil, err
	}
	limits := LimitsFor(req.Role)
	if req.CPUMillis > limits.MaxCPU*1000 {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %d CPU(s) por container", ErrLimitExceeded, req.Role, limits.MaxCPU)
	}
	if req.MemoryMB > limits.MaxMemMB {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %dMB de RAM por container", ErrLimitExceeded, req.Role, limits.MaxMemMB)
	}
	count, err := uc.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao contar containers: %w", err)
	}
	if int(count) >= limits.MaxContainers {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %d containers", ErrLimitExceeded, req.Role, limits.MaxContainers)
	}
	used, err := uc.repo.SumPersistentGBByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao somar armazenamento: %w", err)
	}
	if used+req.PersistentGB > limits.MaxStorageGB {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %d GB de armazenamento persistente", ErrLimitExceeded, req.Role, limits.MaxStorageGB)
	}
	existing, err := uc.repo.FindByUserIDAndName(ctx, userID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar nome: %w", err)
	}
	if existing != nil {
		return nil, ErrContainerNameTaken
	}

	mount := req.MountPath
	if mount == "" {
		mount = defaultMountPath
	}
	env := entity.EnvMap{}
	for k, v := range req.Env {
		env[k] = v
	}
	c := &entity.Container{
		ID: uuid.New(), UserID: userID, Name: req.Name, Description: req.Description, Image: req.Image,
		CPUMillis: req.CPUMillis, MemoryMB: req.MemoryMB, Port: req.Port, Protocol: req.Protocol,
		Env: env, Command: req.Command, PersistentGB: req.PersistentGB, MountPath: mount,
		Namespace: entity.NamespaceFor(userID),
	}
	if req.Port == 0 {
		c.Protocol = entity.ProtocolHTTP
	}

	if err := uc.persistWithNodePort(ctx, c); err != nil {
		return nil, err
	}

	quota := repository.ResourceQuotaSpec{CPUMillis: limits.MaxCPU * 1000, MemoryMB: limits.MaxMemMB, Pods: limits.MaxContainers, StorageGB: limits.MaxStorageGB}
	if err := uc.runtime.EnsureNamespace(ctx, c.Namespace, quota); err != nil {
		uc.rollback(ctx, c)
		log.Printf("[container] EnsureNamespace %s falhou: %v", c.Namespace, err)
		return nil, errors.New("não foi possível preparar o ambiente do usuário")
	}
	if err := uc.runtime.Apply(ctx, c, uc.ingressDomain); err != nil {
		uc.rollback(ctx, c)
		log.Printf("[container] Apply %s/%s falhou: %v", c.Namespace, c.Name, err)
		return nil, errors.New("não foi possível criar o container")
	}
	return uc.view(ctx, c), nil
}

func (uc *ContainerUseCase) validate(req CreateContainerRequest) error {
	if !entity.ContainerNameRegexp.MatchString(req.Name) {
		return fmt.Errorf("%w: nome deve ter 1–40 caracteres minúsculos, dígitos ou hífen", ErrInvalidContainer)
	}
	if strings.ContainsAny(req.Image, " \t\n") || !strings.Contains(req.Image, ":") {
		return fmt.Errorf("%w: imagem deve ter o formato registry/nome:tag", ErrInvalidContainer)
	}
	if req.Port < 0 || req.Port > 65535 {
		return fmt.Errorf("%w: porta fora do intervalo", ErrInvalidContainer)
	}
	if req.Port > 0 && req.Protocol != entity.ProtocolHTTP && req.Protocol != entity.ProtocolTCP {
		return fmt.Errorf("%w: protocolo deve ser http ou tcp", ErrInvalidContainer)
	}
	if req.CPUMillis <= 0 || req.MemoryMB <= 0 || req.PersistentGB < 0 {
		return fmt.Errorf("%w: cpu, memória e armazenamento devem ser positivos", ErrInvalidContainer)
	}
	if req.MountPath != "" && !strings.HasPrefix(req.MountPath, "/") {
		return fmt.Errorf("%w: caminho de montagem deve ser absoluto", ErrInvalidContainer)
	}
	return nil
}

// persistWithNodePort insere o registro; quando precisa de NodePort, sorteia e
// tenta de novo em colisão do índice único ux_containers_node_port.
func (uc *ContainerUseCase) persistWithNodePort(ctx context.Context, c *entity.Container) error {
	needsNodePort := c.Port > 0 && (c.Protocol == entity.ProtocolTCP || uc.ingressDomain == "")
	if !needsNodePort {
		if err := uc.repo.Create(ctx, c); err != nil {
			return fmt.Errorf("erro ao salvar container: %w", err)
		}
		return nil
	}
	for i := 0; i < nodePortRetries; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(nodePortMax-nodePortMin+1))
		if err != nil {
			return fmt.Errorf("erro ao sortear porta: %w", err)
		}
		c.NodePort = nodePortMin + int(n.Int64())
		err = uc.repo.Create(ctx, c)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "ux_containers_node_port") {
			return fmt.Errorf("erro ao salvar container: %w", err)
		}
	}
	return errors.New("não foi possível alocar uma porta externa; tente novamente")
}

func (uc *ContainerUseCase) rollback(ctx context.Context, c *entity.Container) {
	if err := uc.repo.Delete(ctx, c.ID); err != nil {
		log.Printf("[container] rollback do registro %s falhou: %v", c.ID, err)
	}
}

func (uc *ContainerUseCase) List(ctx context.Context, userID uuid.UUID) ([]ContainerView, error) {
	items, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar containers: %w", err)
	}
	views := make([]ContainerView, 0, len(items))
	if len(items) == 0 {
		return views, nil
	}
	names := make([]string, len(items))
	for i := range items {
		names[i] = items[i].Name
	}
	statuses := uc.statuses(ctx, items[0].Namespace, names)
	for i := range items {
		c := &items[i]
		views = append(views, uc.buildView(c, statuses[c.Name]))
	}
	return views, nil
}

func (uc *ContainerUseCase) Get(ctx context.Context, userID, id uuid.UUID) (*ContainerView, error) {
	c, err := uc.FindOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return uc.view(ctx, c), nil
}

// Delete remove do cluster primeiro; se o cluster falhar o registro fica (sem órfão).
func (uc *ContainerUseCase) Delete(ctx context.Context, userID, id uuid.UUID) error {
	c, err := uc.FindOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	if uc.runtime == nil {
		return repository.ErrRuntimeUnavailable
	}
	if err := uc.runtime.Delete(ctx, c.Namespace, c.Name); err != nil {
		log.Printf("[container] Delete %s/%s no cluster falhou: %v", c.Namespace, c.Name, err)
		return repository.ErrRuntimeUnavailable
	}
	if err := uc.repo.Delete(ctx, c.ID); err != nil {
		return fmt.Errorf("erro ao remover container do banco: %w", err)
	}
	return nil
}

// FindOwned devolve o container se pertencer ao usuário; senão ErrContainerNotFound (não vaza existência).
func (uc *ContainerUseCase) FindOwned(ctx context.Context, userID, id uuid.UUID) (*entity.Container, error) {
	c, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar container: %w", err)
	}
	if c == nil || c.UserID != userID {
		return nil, ErrContainerNotFound
	}
	return c, nil
}

func (uc *ContainerUseCase) statuses(ctx context.Context, ns string, names []string) map[string]repository.ContainerStatus {
	if uc.runtime == nil {
		return nil
	}
	st, err := uc.runtime.Status(ctx, ns, names)
	if err != nil {
		log.Printf("[container] Status %s falhou: %v", ns, err)
		return nil
	}
	return st
}

func (uc *ContainerUseCase) view(ctx context.Context, c *entity.Container) *ContainerView {
	st := uc.statuses(ctx, c.Namespace, []string{c.Name})
	v := uc.buildView(c, st[c.Name])
	return &v
}

func (uc *ContainerUseCase) buildView(c *entity.Container, st repository.ContainerStatus) ContainerView {
	if st.Phase == "" {
		st = repository.ContainerStatus{Phase: repository.PhaseUnknown, Message: "cluster indisponível"}
	}
	v := ContainerView{Container: c, Status: st}
	if c.Port == 0 {
		return v
	}
	v.InternalAddress = fmt.Sprintf("%s:%d", c.Name, c.Port)
	switch {
	case c.NodePort > 0:
		v.ExternalURL = fmt.Sprintf("%s:%d", uc.nodeIP, c.NodePort)
	case uc.ingressDomain != "":
		v.ExternalURL = fmt.Sprintf("http://%s-%s.%s", c.Name, c.Namespace, uc.ingressDomain)
	}
	return v
}
