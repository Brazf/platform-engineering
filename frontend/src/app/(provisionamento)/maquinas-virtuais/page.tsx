"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { FORM, InternalBreadcrumb, InternalHeader, PageNotice, ResourceCard } from "@/components";
import { apiListVMs, VM_STATUS_LABEL, type VM } from "@/lib/api";

export default function MaquinasVirtuaisPage() {
  const [vms, setVms] = useState<VM[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;
    async function load() {
      try {
        const data = await apiListVMs();
        if (cancelled) return;
        setVms(data);
        setError("");
        if (data.some((v) => v.status === "creating")) timer = setTimeout(load, 5000);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erro ao carregar VMs.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, []);

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Máquina Virtual" />
        </div>

        <div className="flex h-[90px] items-start justify-between pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Máquina Virtual</h1>
          <Link href="/maquinas-virtuais/criar" className={FORM.primaryBtn}>Criar máquina virtual</Link>
        </div>

        <div className="mx-auto flex max-w-[1100px] flex-col gap-4">
          {error && <PageNotice kind="error">{error}</PageNotice>}
          {loading && <PageNotice kind="info">Carregando...</PageNotice>}
          {!loading && !error && vms.length === 0 && (
            <PageNotice kind="info">Você ainda não tem máquinas virtuais. Crie a primeira pelo botão acima.</PageNotice>
          )}

          {vms.map((vm) => (
            <ResourceCard
              key={vm.id}
              href={`/maquinas-virtuais/${vm.id}`}
              name={vm.name}
              status={{ label: VM_STATUS_LABEL[vm.status], phase: vm.status }}
              createdAt={vm.created_at}
              fields={[
                { label: "Sistema", value: vm.os_template },
                { label: "Recursos", value: `${vm.cpu_cores} ${vm.cpu_cores === 1 ? "núcleo" : "núcleos"}, ${vm.memory_mb / 1024} GB` },
                { label: "Disco", value: `${vm.disk_gb} GB${vm.require_gpu ? ", com GPU" : ""}` },
              ]}
            />
          ))}
        </div>

        <div className="flex items-center justify-center py-6">
          <Link href="/pagina-inicial-usuario" className={FORM.outlineBtn}>Voltar</Link>
        </div>
      </main>
    </div>
  );
}
