// Base URL configurável via variável de ambiente.
// Dev: NEXT_PUBLIC_API_BASE=http://localhost:8080 em .env.local
// Docker (nginx proxy): deixar em branco → URLs relativas
const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("token");
}

export function saveAuth(token: string, role: string): void {
  localStorage.setItem("token", token);
  localStorage.setItem("role", role);
}

export function clearAuth(): void {
  localStorage.removeItem("token");
  localStorage.removeItem("role");
}

export function getRole(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("role");
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(init.headers as Record<string, string>),
  };

  const res = await fetch(API_BASE + path, { ...init, headers });

  if (res.status === 401) {
    clearAuth();
    if (typeof window !== "undefined") window.location.href = "/";
    throw new Error("Não autorizado");
  }

  if (!res.ok) {
    let msg = `Erro ${res.status}`;
    try {
      const body = await res.json();
      msg = (body as { error?: string }).error ?? msg;
    } catch { /* ignore */ }
    throw new Error(msg);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

export async function apiLogin(email: string, password: string) {
  return request<{ token: string; user: { id: string; email: string; role: string } }>(
    "/api/auth/login",
    { method: "POST", body: JSON.stringify({ email, password }) }
  );
}

/** Cadastro de usuário: exige login com perfil pesquisador (POST /api/users). */
export async function apiRegister(email: string, password: string, role: string) {
  return request<{ id: string; email: string; role: string }>(
    "/api/users",
    { method: "POST", body: JSON.stringify({ email, password, role }) }
  );
}

// ─── Meu perfil ───────────────────────────────────────────────────────────────

export interface Profile {
  id: string;
  email: string;
  name: string;
  role: "aluno" | "pesquisador";
  created_at: string;
}

export async function apiGetProfile(): Promise<Profile> {
  return request<Profile>("/api/me");
}

/** Só o nome é alterável; e-mail e perfil são fixos. */
export async function apiUpdateProfile(name: string): Promise<Profile> {
  return request<Profile>("/api/me", { method: "PATCH", body: JSON.stringify({ name }) });
}

/** 403 = senha atual incorreta (a mensagem do backend vem no Error). */
export async function apiChangePassword(currentPassword: string, newPassword: string): Promise<void> {
  return request<void>("/api/me/password", {
    method: "PUT",
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
}

// ─── VMs ──────────────────────────────────────────────────────────────────────

export type VMStatus = "creating" | "running" | "stopped" | "error";

export const VM_STATUS_LABEL: Record<VMStatus, string> = {
  creating: "CRIANDO", running: "LIGADA", stopped: "DESLIGADA", error: "ERRO",
};

export interface VM {
  id: string;
  proxmox_vm_id: number;
  name: string;
  cpu_cores: number;
  memory_mb: number;
  disk_gb: number;
  os_template: string;
  node: string;
  pool: string;
  tags: string;
  require_gpu: boolean;
  status: VMStatus;
  created_at: string;
  updated_at: string;
}

export async function apiListVMs(): Promise<VM[]> {
  return request<VM[]>("/api/vms");
}

export async function apiGetVM(id: string): Promise<VM> {
  return request<VM>(`/api/vms/${id}`);
}

/** Credenciais cloud-init geradas para VMs Ubuntu. Retornadas uma única vez na criação. */
export interface VMCredentials {
  username: string;
  password: string;
}

export async function apiCreateVM(data: {
  name: string;
  cpu_cores: number;
  memory_mb: number;
  disk_gb: number;
  os_template: string;
  pool?: string;
  require_gpu?: boolean;
}): Promise<{
  id: string;
  proxmox_vm_id: number;
  name: string;
  status: VMStatus;
  websocket_url: string;
  credentials?: VMCredentials;
}> {
  return request("/api/vms", { method: "POST", body: JSON.stringify(data) });
}

export async function apiStartVM(id: string): Promise<void> {
  return request<void>(`/api/vms/${id}/start`, { method: "POST" });
}

export async function apiStopVM(id: string): Promise<void> {
  return request<void>(`/api/vms/${id}/stop`, { method: "POST" });
}

export async function apiRebootVM(id: string): Promise<void> {
  return request<void>(`/api/vms/${id}/reboot`, { method: "POST" });
}

export async function apiDeleteVM(id: string): Promise<void> {
  return request<void>(`/api/vms/${id}`, { method: "DELETE" });
}

// ─── Containers ───────────────────────────────────────────────────────────────

export interface CatalogImage {
  key: string;
  label: string;
  image: string;
  port: number;
  protocol: "http" | "tcp" | "";
  mount_path: string;
  command: string;
  required_env: string[] | null;
}

export interface ContainerStatus {
  phase: "pending" | "running" | "failed" | "unknown";
  message: string;
}

export interface Container {
  id: string;
  name: string;
  description: string;
  image: string;
  cpu_millis: number;
  memory_mb: number;
  port: number;
  protocol: "http" | "tcp";
  node_port: number;
  env: Record<string, string>;
  command: string;
  persistent_gb: number;
  mount_path: string;
  namespace: string;
  created_at: string;
  status: ContainerStatus;
  external_url: string;
  internal_address: string;
}

export interface CreateContainerInput {
  name: string;
  description?: string;
  image: string;
  cpu_millis: number;
  memory_mb: number;
  port: number;
  protocol: "http" | "tcp";
  env: Record<string, string>;
  command?: string;
  persistent_gb: number;
  mount_path?: string;
}

export const STATUS_LABEL: Record<ContainerStatus["phase"], string> = {
  pending: "INICIANDO", running: "EM EXECUÇÃO", failed: "FALHOU", unknown: "DESCONHECIDO",
};

export async function apiGetCatalog(): Promise<CatalogImage[]> {
  return request<CatalogImage[]>("/api/containers/catalog");
}
export async function apiListContainers(): Promise<Container[]> {
  return request<Container[]>("/api/containers");
}
export async function apiGetContainer(id: string): Promise<Container> {
  return request<Container>(`/api/containers/${id}`);
}
export async function apiCreateContainer(data: CreateContainerInput): Promise<Container> {
  return request<Container>("/api/containers", { method: "POST", body: JSON.stringify(data) });
}
export async function apiDeleteContainer(id: string): Promise<void> {
  return request<void>(`/api/containers/${id}`, { method: "DELETE" });
}

/** URL do WebSocket de terminal/logs com ?token=, mesma regra do terminal de VM. */
export function containerWsURL(id: string, kind: "terminal" | "logs"): string {
  const token = getToken();
  const wsBase = API_BASE
    ? API_BASE.replace(/^http/, "ws")
    : `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`;
  return `${wsBase}/api/containers/${id}/${kind}${token ? `?token=${encodeURIComponent(token)}` : ""}`;
}
