"use client";

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiCreateVM, getRole, type VMCredentials } from "@/lib/api";

// Componente visual decorativo mantido idêntico ao original.
// O valor (current) foi tornado editável sem mudar posição ou estilo.
function SliderField({
  min, max, value, onChange,
}: {
  min: string; max: string; value: number; onChange: (v: number) => void;
}) {
  return (
    <div>
      <div className="flex h-[5px] w-[408px] rounded-[2px] bg-[#23A324]">
        <div className="h-[5px] w-[168px] bg-[#E08B1D]" />
        <div className="h-[5px] w-[112px] bg-[#FA1D1D]" />
      </div>
      <div className="relative mt-1 flex w-[408px] justify-between text-[9.72px] leading-none text-[#1A1A1A]">
        <span>{min}</span>
        <span>{max}</span>
        <input
          type="number"
          value={value}
          min={0}
          onChange={e => onChange(Math.max(0, Number(e.target.value)))}
          className="absolute left-[120px] top-4 w-16 border-none bg-transparent text-center text-[9.72px] leading-none text-[#1A1A1A] outline-none"
        />
      </div>
      <div className="relative -mt-[14px] ml-[98px] h-0 w-0 border-l-[8px] border-r-[8px] border-t-[12px] border-l-transparent border-r-transparent border-t-black" />
    </div>
  );
}

const OS_OPTIONS = [
  { label: "WINDOWS", value: "windows-10" },
  { label: "UBUNTU",  value: "ubuntu-22.04" },
  { label: "DEBIAN",  value: "debian-11" },
] as const;

export default function CriarMaquinaVirtualPage() {
  const router = useRouter();
  const [name, setName]           = useState("");
  const [os, setOs]               = useState("ubuntu-22.04");
  const [cpu, setCpu]             = useState(2);
  const [memoryMb, setMemoryMb]   = useState(2048);
  const [diskGb, setDiskGb]       = useState(20);
  const [requireGpu, setRequireGpu] = useState(false);
  const [error, setError]         = useState("");
  const [loading, setLoading]     = useState(false);
  const [credentials, setCredentials] = useState<VMCredentials | null>(null);
  const [copied, setCopied]       = useState<"username" | "password" | null>(null);

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
      const role = getRole() ?? "aluno";
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

        <section className="h-[484px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES DA MÁQUINA</p>

          <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME DA MÁQUINA:</p>
          <input
            className="mt-1 h-7 w-[623px] border border-black bg-transparent px-2"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="Ex: minha-vm"
          />

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">SISTEMA OPERACIONAL:</p>
          <div className="mt-3 flex gap-6 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            {OS_OPTIONS.map(opt => (
              <label key={opt.value} className="flex items-center gap-2">
                <input
                  type="radio"
                  name="so"
                  checked={os === opt.value}
                  onChange={() => setOs(opt.value)}
                />
                {opt.label}
              </label>
            ))}
          </div>

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE NÚCLEOS (CPUS)</p>
          <div className="mt-3">
            <SliderField min="0" max="24 CPUS" value={cpu} onChange={setCpu} />
          </div>

          <p className="mt-5 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE MEMÓRIA (RAM)</p>
          <div className="mt-3">
            <SliderField min="0" max="16000 MB" value={memoryMb} onChange={setMemoryMb} />
          </div>

          <p className="mt-5 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE ARMAZENAMENTO (GB)</p>
          <div className="mt-3">
            <SliderField min="0" max="1 TB" value={diskGb} onChange={setDiskGb} />
          </div>

          <p className="mt-6 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">REQUER GPU:</p>
          <div className="mt-3 flex gap-10 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            <label className="flex items-center gap-2">
              <input type="radio" name="gpu" checked={requireGpu === true} onChange={() => setRequireGpu(true)} />
              SIM
            </label>
            <label className="flex items-center gap-2">
              <input type="radio" name="gpu" checked={requireGpu === false} onChange={() => setRequireGpu(false)} />
              NÃO
            </label>
          </div>

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DESCRIÇÃO DA MÁQUINA:</p>
          <input className="mt-1 h-[34px] w-full border border-black bg-transparent px-2" />
        </section>

        {error && (
          <p className="mt-3 text-[12px] leading-none text-[#B21D38]">{error}</p>
        )}

        <div className="flex h-[71px] items-center justify-center py-4">
          <button
            type="button"
            onClick={handleCriar}
            disabled={loading}
            className="flex h-[39px] w-[86px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white disabled:opacity-60"
          >
            {loading ? "..." : "Criar"}
          </button>
        </div>
      </main>

      {credentials && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" role="dialog" aria-modal="true">
          <div className="w-[480px] bg-white px-6 pt-5 pb-6 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">MÁQUINA CRIADA</p>
            <p className="mt-3 text-[12px] leading-[16px] text-[rgba(0,0,0,0.87)]">
              Anote as credenciais de acesso. Por segurança, a senha é exibida <strong>apenas uma vez</strong> e não pode ser recuperada depois.
            </p>

            <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">USUÁRIO:</p>
            <div className="mt-1 flex gap-2">
              <input readOnly className="h-7 flex-1 border border-black bg-transparent px-2 font-mono text-[13px]" value={credentials.username} />
              <button type="button" onClick={() => copiar("username", credentials.username)} className="h-7 w-[86px] border border-black text-[11.67px] font-bold tracking-[1.5px] uppercase">
                {copied === "username" ? "Copiado" : "Copiar"}
              </button>
            </div>

            <p className="mt-3 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">SENHA:</p>
            <div className="mt-1 flex gap-2">
              <input readOnly className="h-7 flex-1 border border-black bg-transparent px-2 font-mono text-[13px]" value={credentials.password} />
              <button type="button" onClick={() => copiar("password", credentials.password)} className="h-7 w-[86px] border border-black text-[11.67px] font-bold tracking-[1.5px] uppercase">
                {copied === "password" ? "Copiado" : "Copiar"}
              </button>
            </div>

            <div className="mt-6 flex justify-center">
              <button
                type="button"
                onClick={() => router.push("/maquinas-virtuais")}
                className="flex h-[39px] items-center justify-center rounded-[20px] bg-[#1351B4] px-6 text-[16.8px] leading-none font-semibold text-white"
              >
                Ir para minhas máquinas
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
