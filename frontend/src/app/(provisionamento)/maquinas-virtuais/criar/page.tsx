"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { FORM, FormActions, FormCard, Group, InternalBreadcrumb, InternalHeader, Segmented } from "@/components";
import { apiCreateVM, getRole, type VMCredentials } from "@/lib/api";

const OS_OPTIONS = [
  { v: "ubuntu-22.04", label: "Ubuntu" },
  { v: "debian-11", label: "Debian" },
  { v: "windows-10", label: "Windows" },
];
const CPU_OPTIONS = [1, 2, 4, 8];
const MEM_OPTIONS = [1024, 2048, 4096, 8192, 16384];
const DISK_OPTIONS = [10, 20, 40, 80, 160, 320];

export default function CriarMaquinaVirtualPage() {
  const router = useRouter();
  const [role, setRole] = useState("aluno");
  const maxCpu = role === "pesquisador" ? 8 : 2;
  const maxMem = role === "pesquisador" ? 16384 : 4096;

  const [name, setName] = useState("");
  const [os, setOs] = useState("ubuntu-22.04");
  const [cpu, setCpu] = useState(2);
  const [memoryMb, setMemoryMb] = useState(2048);
  const [diskGb, setDiskGb] = useState(20);
  const [requireGpu, setRequireGpu] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [credentials, setCredentials] = useState<VMCredentials | null>(null);
  const [copied, setCopied] = useState<"username" | "password" | null>(null);

  useEffect(() => { setRole(getRole() ?? "aluno"); }, []);

  async function copiar(campo: "username" | "password", valor: string) {
    try {
      await navigator.clipboard.writeText(valor);
      setCopied(campo);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      // clipboard indisponível (http sem TLS, permissão negada) — usuário copia manualmente
    }
  }

  async function handleCriar() {
    setError("");
    if (!name.trim()) {
      setError("Informe o nome da máquina.");
      return;
    }
    setLoading(true);
    try {
      const pool = role === "pesquisador" ? "researchers" : "students";
      const created = await apiCreateVM({
        name: name.trim(),
        cpu_cores: cpu,
        memory_mb: memoryMb,
        disk_gb: diskGb,
        os_template: os,
        pool,
        require_gpu: requireGpu,
      });
      if (created.credentials) {
        // senha só existe nesta resposta: mostrar antes de sair da tela
        setCredentials(created.credentials);
        return;
      }
      router.push("/maquinas-virtuais");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao criar VM.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Máquina Virtual" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Máquina Virtual</h1>
        </div>

        <FormCard
          title="NOVA MÁQUINA VIRTUAL"
          subtitle={<>Perfil <span className="font-semibold text-black">{role}</span>: até {maxCpu} núcleos e {maxMem / 1024} GB de memória por máquina.</>}
        >
          <Group title="Identificação">
            <div>
              <label className={FORM.label} htmlFor="name">Nome</label>
              <input id="name" className={FORM.input} value={name} onChange={(e) => setName(e.target.value)} placeholder="minha-vm" autoComplete="off" />
              <p className={FORM.hint}>Aparece na lista de máquinas e no Proxmox.</p>
            </div>
            <div>
              <p className={FORM.label}>Sistema operacional</p>
              <Segmented name="sistema operacional" value={os} onChange={setOs} options={OS_OPTIONS} />
              <p className={FORM.hint}>Ubuntu nasce pronto, com usuário e senha gerados. Os outros iniciam pela mídia de instalação.</p>
            </div>
          </Group>

          <Group title="Recursos">
            <div>
              <label className={FORM.label} htmlFor="cpu">Núcleos de CPU</label>
              <select id="cpu" className={FORM.input} value={cpu} onChange={(e) => setCpu(Number(e.target.value))}>
                {CPU_OPTIONS.filter((v) => v <= maxCpu).map((v) => (
                  <option key={v} value={v}>{v} {v === 1 ? "núcleo" : "núcleos"}</option>
                ))}
              </select>
            </div>
            <div>
              <label className={FORM.label} htmlFor="mem">Memória</label>
              <select id="mem" className={FORM.input} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))}>
                {MEM_OPTIONS.filter((v) => v <= maxMem).map((v) => (
                  <option key={v} value={v}>{v / 1024} GB</option>
                ))}
              </select>
            </div>
            <div>
              <label className={FORM.label} htmlFor="disk">Disco</label>
              <select id="disk" className={FORM.input} value={diskGb} onChange={(e) => setDiskGb(Number(e.target.value))}>
                {DISK_OPTIONS.map((v) => (
                  <option key={v} value={v}>{v} GB</option>
                ))}
              </select>
            </div>
            <div>
              <p className={FORM.label}>GPU</p>
              <Segmented name="gpu" value={requireGpu} onChange={setRequireGpu}
                options={[{ v: false, label: "Não precisa" }, { v: true, label: "Requer GPU" }]} />
              <p className={FORM.hint}>Disponível para o perfil pesquisador, uma máquina por vez.</p>
            </div>
          </Group>

          {error && <p role="alert" className={FORM.error}>{error}</p>}
        </FormCard>

        <FormActions cancelHref="/maquinas-virtuais" submitLabel="Criar máquina" loadingLabel="Criando..." loading={loading} onSubmit={handleCriar} />
      </main>

      {credentials && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" role="dialog" aria-modal="true">
          <div className="w-full max-w-[520px] bg-white px-8 pt-6 pb-7 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <p className="text-[12px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">MÁQUINA CRIADA</p>
            <p className="mt-2 text-[14px] leading-[20px] text-[#333]">
              Anote as credenciais de acesso. Por segurança, a senha é exibida <strong>apenas uma vez</strong> e não pode ser recuperada depois.
            </p>

            <div className="mt-5 flex flex-col gap-4">
              {(["username", "password"] as const).map((campo) => (
                <div key={campo}>
                  <p className={FORM.label}>{campo === "username" ? "Usuário" : "Senha"}</p>
                  <div className="flex gap-2">
                    <input readOnly className={FORM.input + " font-mono"} value={credentials[campo]} />
                    <button type="button" onClick={() => copiar(campo, credentials[campo])} className={FORM.outlineBtn + " h-10 shrink-0 px-5 text-[14px]"}>
                      {copied === campo ? "Copiado" : "Copiar"}
                    </button>
                  </div>
                </div>
              ))}
            </div>

            <div className="mt-7 flex justify-center">
              <button type="button" onClick={() => router.push("/maquinas-virtuais")} className={FORM.primaryBtn}>
                Ir para minhas máquinas
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
