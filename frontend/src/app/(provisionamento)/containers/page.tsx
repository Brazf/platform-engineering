"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiListContainers, STATUS_LABEL, type Container } from "@/lib/api";

const LINE = "text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase";

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

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Container</h1>
        </div>

        {error && <p className="text-[13px] text-[#E52207]">{error}</p>}
        {loading && <p className={LINE}>Carregando...</p>}
        {!loading && !error && items.length === 0 && <p className={LINE}>Nenhum container criado.</p>}

        <div className="flex flex-col gap-4">
          {items.map((c) => (
            <Link
              key={c.id}
              href={`/containers/${c.id}`}
              className="block bg-white px-3 py-[25px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]"
            >
              <p className={LINE}>NOME: {c.name}</p>
              <p className={"mt-2 " + LINE}>IMAGEM: {c.image}</p>
              <p className={"mt-2 " + LINE}>STATUS: {STATUS_LABEL[c.status.phase]}</p>
              <p className={"mt-2 " + LINE}>ENDEREÇO: {c.external_url || "—"}</p>
              <p className={"mt-2 " + LINE}>DATA DE CRIAÇÃO: {new Date(c.created_at).toLocaleDateString("pt-BR")}</p>
            </Link>
          ))}
        </div>

        <div className="flex h-[71px] items-center justify-center py-4">
          <Link
            href="/containers/criar"
            className="flex h-[39px] w-[214px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Criar Container
          </Link>
        </div>
      </main>
    </div>
  );
}
