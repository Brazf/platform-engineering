"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiGetVM, apiStartVM, apiStopVM, apiDeleteVM, type VM } from "@/lib/api";

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString("pt-BR");
  } catch {
    return "-";
  }
}

export default function MaquinaVirtualDetalhePage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;

  const [vm, setVm]           = useState<VM | null>(null);
  const [error, setError]     = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing]   = useState(false);

  useEffect(() => {
    if (!id) return;
    apiGetVM(id)
      .then(setVm)
      .catch(err => setError(err instanceof Error ? err.message : "Erro ao carregar VM."))
      .finally(() => setLoading(false));
  }, [id]);

  async function handleStart() {
    if (!vm) return;
    setActing(true);
    setError("");
    try {
      await apiStartVM(id);
      router.push(`/maquinas-virtuais/${id}/utilizar`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao iniciar VM.");
      setActing(false);
    }
  }

  async function handleStop() {
    if (!vm) return;
    setActing(true);
    setError("");
    try {
      await apiStopVM(id);
      setVm(v => v ? { ...v, status: "stopped" } : v);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao parar VM.");
    } finally {
      setActing(false);
    }
  }

  async function handleDelete() {
    if (!vm) return;
    setActing(true);
    setError("");
    try {
      await apiDeleteVM(id);
      router.push("/maquinas-virtuais");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao excluir VM.");
      setActing(false);
    }
  }

  const isRunning  = vm?.status === "running";
  const isCreating = vm?.status === "creating";

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

        {loading && (
          <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">CARREGANDO...</p>
        )}

        {!loading && vm && (
          <>
            <section className="h-[137px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES</p>
              <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME: {vm.name}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">SISTEMA: {vm.os_template}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DATA DE CRIAÇÃO: {formatDate(vm.created_at)}</p>
            </section>

            <section className="mt-5 h-[157px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">CONFIGURAÇÃO</p>
              <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">SISTEMA OPERACIONAL: {vm.os_template.toUpperCase()}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE NÚCLEOS (CPUS): {vm.cpu_cores}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE MEMÓRIA: {vm.memory_mb} MB</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE ARMAZENAMENTO: {vm.disk_gb} GB</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">CONSOME GPU: NÃO</p>
            </section>

            <section className="mt-5 h-[114px]">
              <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#E52207]">ESTADO DA MÁQUINA</p>
              <div className="mt-[11px] border-t border-[#CCCCCC]" />

              <div className="flex h-[39px] items-center justify-between border-b border-[#CCCCCC] px-4">
                <span className="text-[14px] leading-none text-[#1351B4]">MÁQUINA LIGADA</span>
                <span className={`h-7 w-7 rounded-full ${isRunning ? "bg-[#1FA433]" : "bg-[#CCCCCC]"}`} />
              </div>

              <div className="flex h-[39px] items-center justify-between px-4">
                <span className="text-[14px] leading-none text-[#1351B4]">MÁQUINA DESLIGADA</span>
                <span className={`h-7 w-7 rounded-full ${!isRunning && !isCreating ? "bg-[#FA1D1D]" : "bg-[#CCCCCC]"}`} />
              </div>
            </section>

            {error && (
              <p className="mt-3 text-[12px] leading-none text-[#B21D38]">{error}</p>
            )}

            <div className="flex h-[71px] items-center justify-center gap-2 py-4">
              {/* Botão esquerdo (vermelho): Excluir quando parada / Parar quando rodando */}
              {isRunning ? (
                <button
                  type="button"
                  onClick={handleStop}
                  disabled={acting}
                  className="flex h-[39px] w-[230px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38] disabled:opacity-60"
                >
                  {acting ? "Parando..." : "Parar Máquina Virtual"}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={handleDelete}
                  disabled={acting}
                  className="flex h-[39px] w-[230px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38] disabled:opacity-60"
                >
                  {acting ? "Excluindo..." : "Excluir Máquina Virtual"}
                </button>
              )}

              {/* Botão direito (azul): Acessar terminal quando rodando / Iniciar quando parada */}
              {isRunning ? (
                <Link
                  href={`/maquinas-virtuais/${id}/utilizar`}
                  className="flex h-[39px] w-[225px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
                >
                  Acessar Terminal
                </Link>
              ) : (
                <button
                  type="button"
                  onClick={handleStart}
                  disabled={acting || isCreating}
                  className="flex h-[39px] w-[225px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white disabled:opacity-60"
                >
                  {acting ? "Iniciando..." : isCreating ? "Criando VM..." : "Iniciar Máquina Virtual"}
                </button>
              )}
            </div>
          </>
        )}

        {!loading && !vm && !error && (
          <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">VM NÃO ENCONTRADA.</p>
        )}

        {!loading && error && (
          <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#B21D38]">{error.toUpperCase()}</p>
        )}
      </main>
    </div>
  );
}
