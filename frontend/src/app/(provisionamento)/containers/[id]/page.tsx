"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiDeleteContainer, apiGetContainer, STATUS_LABEL, type Container } from "@/lib/api";

const LINE = "mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase";
const TITLE = "text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]";
const DOT: Record<Container["status"]["phase"], string> = {
  running: "#1FA433",
  pending: "#F5A623",
  failed: "#FA1D1D",
  unknown: "#FA1D1D",
};

export default function ContainerDetalhePage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;
  const [c, setC] = useState<Container | null>(null);
  const [error, setError] = useState("");
  const [showEnv, setShowEnv] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!id) return;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;
    async function load() {
      try {
        const data = await apiGetContainer(id);
        if (cancelled) return;
        setC(data);
        setError("");
        if (data.status.phase === "pending") timer = setTimeout(load, 5000);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erro ao carregar container.");
      }
    }
    load();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [id]);

  async function handleDelete() {
    if (!confirm("Excluir este container? Os dados persistentes serão apagados.")) return;
    setDeleting(true);
    setError("");
    try {
      await apiDeleteContainer(id);
      router.push("/containers");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao excluir container.");
      setDeleting(false);
    }
  }

  const envKeys = c ? Object.keys(c.env ?? {}) : [];

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
        {!c && !error && <p className={LINE}>Carregando...</p>}

        {c && (
          <>
            <section className="bg-white px-3 pt-2 pb-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <p className={TITLE}>DETALHES</p>
              <p className={LINE}>NOME: {c.name}</p>
              <p className={LINE}>DESCRIÇÃO: {c.description || "—"}</p>
              <p className={LINE}>DATA DE CRIAÇÃO: {new Date(c.created_at).toLocaleDateString("pt-BR")}</p>
            </section>

            <section className="mt-5 bg-white px-3 pt-2 pb-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <p className={TITLE}>CONFIGURAÇÃO</p>
              <p className={LINE}>IMAGEM: {c.image}</p>
              <p className={LINE}>CPU: {c.cpu_millis / 1000} vCPU</p>
              <p className={LINE}>MEMÓRIA: {c.memory_mb >= 1024 ? `${c.memory_mb / 1024} GB` : `${c.memory_mb} MB`}</p>
              <p className={LINE}>PORTA: {c.port > 0 ? `${c.port} (${c.protocol})` : "não exposto"}</p>
              <p className={LINE}>ARMAZENAMENTO PERSISTENTE: {c.persistent_gb > 0 ? `${c.persistent_gb} GB em ${c.mount_path}` : "não"}</p>
              <p className={LINE}>COMANDO: {c.command || "—"}</p>
            </section>

            <section className="mt-5 bg-white px-3 pt-2 pb-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <p className={TITLE}>ACESSO</p>
              <p className={LINE}>ENDEREÇO EXTERNO: {c.external_url || "—"}</p>
              <p className={LINE}>ENDEREÇO INTERNO: {c.internal_address || "—"}</p>
            </section>

            <section className="mt-5 bg-white px-3 pt-2 pb-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <p className={TITLE}>VARIÁVEIS DE AMBIENTE</p>
              {envKeys.length === 0 && <p className={LINE}>—</p>}
              {envKeys.map((k) => (
                <p key={k} className={LINE}>{k}: {showEnv ? c.env[k] : "••••"}</p>
              ))}
              {envKeys.length > 0 && (
                <button type="button" className="mt-2 text-[11.67px] font-bold tracking-[1.5px] text-[#1351B4] uppercase" onClick={() => setShowEnv(!showEnv)}>
                  {showEnv ? "ocultar" : "mostrar"}
                </button>
              )}
            </section>

            <section className="mt-5">
              <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#E52207]">ESTADO DO CONTAINER</p>
              <div className="mt-[11px] border-t border-[#CCCCCC]" />
              <div className="flex h-[39px] items-center justify-between border-b border-[#CCCCCC] px-4">
                <span className="text-[14px] leading-none text-[#1351B4]">
                  {STATUS_LABEL[c.status.phase]}{c.status.message ? ` — ${c.status.message}` : ""}
                </span>
                <span className="h-7 w-7 rounded-full" style={{ backgroundColor: DOT[c.status.phase] }} />
              </div>
            </section>

            <div className="flex h-[71px] items-center justify-center gap-2 py-4">
              <button
                type="button"
                onClick={handleDelete}
                disabled={deleting}
                className="flex h-[39px] w-[183px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38] disabled:opacity-60"
              >
                {deleting ? "..." : "Excluir Container"}
              </button>
              <Link
                href={`/containers/${c.id}/utilizar`}
                className="flex h-[39px] w-[178px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
              >
                Utilizar Container
              </Link>
            </div>
          </>
        )}
      </main>
    </div>
  );
}
