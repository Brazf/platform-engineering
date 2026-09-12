"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { FORM, InternalBreadcrumb, InternalHeader, PageNotice, ResourceCard } from "@/components";
import { apiListContainers, STATUS_LABEL, type Container } from "@/lib/api";

export default function ContainersPage() {
  const [items, setItems] = useState<Container[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;
    async function load() {
      try {
        const data = await apiListContainers();
        if (cancelled) return;
        setItems(data);
        setError("");
        if (data.some((c) => c.status.phase === "pending")) timer = setTimeout(load, 5000);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erro ao listar containers.");
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
          <InternalBreadcrumb label="Container" />
        </div>

        <div className="flex h-[90px] items-start justify-between pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Container</h1>
          <Link href="/containers/criar" className={FORM.primaryBtn}>Criar container</Link>
        </div>

        <div className="mx-auto flex max-w-[1100px] flex-col gap-4">
          {error && <PageNotice kind="error">{error}</PageNotice>}
          {loading && <PageNotice kind="info">Carregando...</PageNotice>}
          {!loading && !error && items.length === 0 && (
            <PageNotice kind="info">Você ainda não tem containers. Crie o primeiro pelo botão acima.</PageNotice>
          )}

          {items.map((c) => (
            <ResourceCard
              key={c.id}
              href={`/containers/${c.id}`}
              name={c.name}
              description={c.description}
              status={{ label: STATUS_LABEL[c.status.phase], phase: c.status.phase, message: c.status.message }}
              createdAt={c.created_at}
              fields={[
                { label: "Imagem", value: c.image },
                { label: "Recursos", value: `${c.cpu_millis / 1000} vCPU, ${c.memory_mb >= 1024 ? `${c.memory_mb / 1024} GB` : `${c.memory_mb} MB`}` },
                { label: "Endereço", value: c.external_url || "não exposto" },
              ]}
            />
          ))}
        </div>
      </main>
    </div>
  );
}
