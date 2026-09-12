"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { DetailHeader, FORM, Field, Group, InternalBreadcrumb, InternalHeader, PageNotice } from "@/components";
import { apiDeleteContainer, apiGetContainer, STATUS_LABEL, type Container } from "@/lib/api";

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

        <div className="mx-auto max-w-[1100px]">
          {error && <PageNotice kind="error">{error}</PageNotice>}
          {!c && !error && <PageNotice kind="info">Carregando...</PageNotice>}

          {c && (
            <section className="bg-white px-10 pt-7 pb-9 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <DetailHeader
                name={c.name}
                description={c.description}
                status={{ label: STATUS_LABEL[c.status.phase], phase: c.status.phase, message: c.status.message }}
              />

              <div className="mt-7 flex flex-col gap-7">
                <Group title="Configuração">
                  <Field label="Imagem">{c.image}</Field>
                  <Field label="Recursos">{c.cpu_millis / 1000} vCPU, {c.memory_mb >= 1024 ? `${c.memory_mb / 1024} GB` : `${c.memory_mb} MB`}</Field>
                  <Field label="Porta">{c.port > 0 ? `${c.port} (${c.protocol.toUpperCase()})` : "não exposto"}</Field>
                  <Field label="Armazenamento persistente">{c.persistent_gb > 0 ? `${c.persistent_gb} GB em ${c.mount_path}` : "não"}</Field>
                  <Field label="Comando">{c.command || "padrão da imagem"}</Field>
                  <Field label="Criado em">{new Date(c.created_at).toLocaleString("pt-BR")}</Field>
                </Group>

                <Group title="Acesso">
                  <Field label="Endereço externo">{c.external_url || "não exposto"}</Field>
                  <Field label="Endereço interno">{c.internal_address || "—"}</Field>
                </Group>

                <Group title="Variáveis de ambiente">
                  <div className="md:col-span-2">
                    {envKeys.length === 0 ? (
                      <p className={FORM.value}>nenhuma</p>
                    ) : (
                      <ul className="flex flex-col gap-1">
                        {envKeys.map((k) => (
                          <li key={k} className="text-[15px] leading-[22px] text-black break-all">
                            <span className="font-mono">{k}</span>
                            <span className="text-[#9A9A9A]"> = </span>
                            <span className="font-mono">{showEnv ? c.env[k] : "••••••"}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                    {envKeys.length > 0 && (
                      <button type="button" className={FORM.linkBtn + " mt-3"} onClick={() => setShowEnv(!showEnv)}>
                        {showEnv ? "Ocultar valores" : "Mostrar valores"}
                      </button>
                    )}
                  </div>
                </Group>
              </div>
            </section>
          )}

          {c && (
            <div className="flex flex-wrap items-center justify-center gap-3 py-6">
              <Link href="/containers" className={FORM.outlineBtn}>Voltar</Link>
              <button type="button" onClick={handleDelete} disabled={deleting} className={FORM.dangerBtn}>
                {deleting ? "Excluindo..." : "Excluir container"}
              </button>
              <Link href={`/containers/${c.id}/utilizar`} className={FORM.primaryBtn}>Abrir terminal e logs</Link>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
