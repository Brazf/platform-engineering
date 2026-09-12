"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { DetailHeader, FORM, Field, Group, InternalBreadcrumb, InternalHeader, PageNotice } from "@/components";
import { apiDeleteVM, apiGetVM, apiStartVM, apiStopVM, VM_STATUS_LABEL, type VM } from "@/lib/api";

export default function MaquinaVirtualDetalhePage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;

  const [vm, setVm] = useState<VM | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);

  useEffect(() => {
    if (!id) return;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;
    async function load() {
      try {
        const data = await apiGetVM(id);
        if (cancelled) return;
        setVm(data);
        if (data.status === "creating") timer = setTimeout(load, 5000);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erro ao carregar VM.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
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
      setVm((v) => (v ? { ...v, status: "stopped" } : v));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao parar VM.");
    } finally {
      setActing(false);
    }
  }

  async function handleDelete() {
    if (!vm) return;
    if (!confirm("Excluir esta máquina virtual? O disco será apagado.")) return;
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

  const isRunning = vm?.status === "running";
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

        <div className="mx-auto max-w-[1100px]">
          {error && <PageNotice kind="error">{error}</PageNotice>}
          {loading && <PageNotice kind="info">Carregando...</PageNotice>}
          {!loading && !vm && !error && <PageNotice kind="info">Máquina virtual não encontrada.</PageNotice>}

          {vm && (
            <section className="bg-white px-10 pt-7 pb-9 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <DetailHeader name={vm.name} status={{ label: VM_STATUS_LABEL[vm.status], phase: vm.status }} />

              <div className="mt-7 flex flex-col gap-7">
                <Group title="Configuração">
                  <Field label="Sistema operacional">{vm.os_template}</Field>
                  <Field label="Núcleos de CPU">{vm.cpu_cores}</Field>
                  <Field label="Memória">{vm.memory_mb / 1024} GB</Field>
                  <Field label="Disco">{vm.disk_gb} GB</Field>
                  <Field label="GPU">{vm.require_gpu ? "sim" : "não"}</Field>
                  <Field label="Criado em">{new Date(vm.created_at).toLocaleString("pt-BR")}</Field>
                </Group>

                <Group title="Proxmox">
                  <Field label="ID da VM">{vm.proxmox_vm_id}</Field>
                  <Field label="Nó">{vm.node}</Field>
                  {vm.pool && <Field label="Pool">{vm.pool}</Field>}
                  {vm.tags && <Field label="Tags">{vm.tags.split(";").join(", ")}</Field>}
                </Group>
              </div>
            </section>
          )}

          {vm && (
            <div className="flex flex-wrap items-center justify-center gap-3 py-6">
              <Link href="/maquinas-virtuais" className={FORM.outlineBtn}>Voltar</Link>
              {isRunning ? (
                <button type="button" onClick={handleStop} disabled={acting} className={FORM.dangerBtn}>
                  {acting ? "Parando..." : "Desligar máquina"}
                </button>
              ) : (
                <button type="button" onClick={handleDelete} disabled={acting || isCreating} className={FORM.dangerBtn}>
                  {acting ? "Excluindo..." : "Excluir máquina"}
                </button>
              )}
              {isRunning ? (
                <Link href={`/maquinas-virtuais/${id}/utilizar`} className={FORM.primaryBtn}>Abrir terminal</Link>
              ) : (
                <button type="button" onClick={handleStart} disabled={acting || isCreating} className={FORM.primaryBtn}>
                  {acting ? "Ligando..." : isCreating ? "Criando..." : "Ligar máquina"}
                </button>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
