"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiCreateContainer, apiGetCatalog, getRole, type CatalogImage } from "@/lib/api";

const LABEL = "mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase";
const INPUT = "mt-1 h-[34px] w-full border border-black bg-transparent px-2";
const RADIO_ROW = "mt-3 flex gap-10 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]";

const CPU_OPTIONS = [500, 1000, 2000, 4000, 8000]; // millis
const MEM_OPTIONS = [256, 512, 1024, 2048, 4096, 8192, 16384];

export default function CriarContainerPage() {
  const router = useRouter();
  const [role, setRole] = useState("aluno");
  const maxCpu = role === "pesquisador" ? 8000 : 2000;
  const maxMem = role === "pesquisador" ? 16384 : 4096;

  const [catalog, setCatalog] = useState<CatalogImage[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [catalogKey, setCatalogKey] = useState("nginx");
  const [customImage, setCustomImage] = useState("");
  const [cpu, setCpu] = useState(500);
  const [mem, setMem] = useState(512);
  const [expose, setExpose] = useState(true);
  const [port, setPort] = useState(80);
  const [protocol, setProtocol] = useState<"http" | "tcp">("http");
  const [env, setEnv] = useState<{ k: string; v: string }[]>([]);
  const [persist, setPersist] = useState(false);
  const [persistGb, setPersistGb] = useState(5);
  const [command, setCommand] = useState("");
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setRole(getRole() ?? "aluno");
    apiGetCatalog().then(setCatalog).catch(() => setCatalog([]));
  }, []);

  const selected = catalog.find((c) => c.key === catalogKey);

  // Ao trocar de item do catálogo, pré-preenche porta/protocolo/comando/env obrigatórias.
  useEffect(() => {
    if (!selected) return;
    setExpose(selected.port > 0);
    setPort(selected.port || 80);
    setProtocol(selected.protocol === "tcp" ? "tcp" : "http");
    setCommand(selected.command ?? "");
    setEnv((selected.required_env ?? []).map((k) => ({ k, v: "" })));
  }, [selected?.key]); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleCriar() {
    setError("");
    const image = catalogKey === "outra" ? customImage.trim() : selected?.image ?? "";
    if (!/^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$/.test(name)) {
      setError("Nome: 1–40 caracteres, letras minúsculas, números e hífen.");
      return;
    }
    if (!image.includes(":")) {
      setError("Informe a imagem com tag, ex.: nginx:1.27.");
      return;
    }
    const missing = env.find((e) => e.k && !e.v && selected?.required_env?.includes(e.k));
    if (missing) {
      setError(`Preencha a variável ${missing.k}.`);
      return;
    }
    setLoading(true);
    try {
      await apiCreateContainer({
        name,
        description,
        image,
        cpu_millis: cpu,
        memory_mb: mem,
        port: expose ? port : 0,
        protocol,
        env: Object.fromEntries(env.filter((e) => e.k).map((e) => [e.k, e.v])),
        command: command || undefined,
        persistent_gb: persist ? persistGb : 0,
        mount_path: catalogKey === "outra" ? undefined : selected?.mount_path,
      });
      router.push("/containers");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao criar container.");
    } finally {
      setLoading(false);
    }
  }

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

        <section className="bg-white px-3 pt-2 pb-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES DO CONTAINER</p>

          <p className={LABEL}>NOME DO CONTAINER:</p>
          <input className="mt-1 h-7 w-[623px] border border-black bg-transparent px-2" value={name} onChange={(e) => setName(e.target.value)} />

          <p className={LABEL}>IMAGEM DO CONTAINER</p>
          <select className={INPUT} value={catalogKey} onChange={(e) => setCatalogKey(e.target.value)}>
            {catalog.map((c) => (
              <option key={c.key} value={c.key}>{c.label} ({c.image})</option>
            ))}
            <option value="outra">Outra imagem...</option>
          </select>
          {catalogKey === "outra" && (
            <input className={INPUT} placeholder="registry/imagem:tag" value={customImage} onChange={(e) => setCustomImage(e.target.value)} />
          )}

          <p className={LABEL}>CPU:</p>
          <select className={INPUT} value={cpu} onChange={(e) => setCpu(Number(e.target.value))}>
            {CPU_OPTIONS.filter((v) => v <= maxCpu).map((v) => (
              <option key={v} value={v}>{v / 1000} vCPU</option>
            ))}
          </select>
          <p className={LABEL}>MEMÓRIA:</p>
          <select className={INPUT} value={mem} onChange={(e) => setMem(Number(e.target.value))}>
            {MEM_OPTIONS.filter((v) => v <= maxMem).map((v) => (
              <option key={v} value={v}>{v >= 1024 ? `${v / 1024} GB` : `${v} MB`}</option>
            ))}
          </select>

          <p className={LABEL}>PORTA E PROTOCOLO:</p>
          <div className={RADIO_ROW}>
            <label className="flex items-center gap-2"><input type="checkbox" checked={!expose} onChange={(e) => setExpose(!e.target.checked)} /> NÃO EXPOR</label>
          </div>
          {expose && (
            <>
              <input className={INPUT} type="number" min={1} max={65535} value={port} onChange={(e) => setPort(Number(e.target.value))} />
              <div className={RADIO_ROW}>
                <label className="flex items-center gap-2"><input type="radio" name="proto" checked={protocol === "http"} onChange={() => setProtocol("http")} /> HTTP (SITE/API)</label>
                <label className="flex items-center gap-2"><input type="radio" name="proto" checked={protocol === "tcp"} onChange={() => setProtocol("tcp")} /> TCP (BANCO, OUTRO SERVIÇO)</label>
              </div>
            </>
          )}

          <p className={LABEL}>VARIÁVEIS DE AMBIENTE:</p>
          {env.map((e, i) => (
            <div key={i} className="mt-1 flex gap-2">
              <input className="h-[34px] w-1/3 border border-black bg-transparent px-2" placeholder="CHAVE" value={e.k}
                onChange={(ev) => setEnv(env.map((x, j) => (j === i ? { ...x, k: ev.target.value } : x)))} />
              <input className="h-[34px] flex-1 border border-black bg-transparent px-2" placeholder="valor" value={e.v}
                onChange={(ev) => setEnv(env.map((x, j) => (j === i ? { ...x, v: ev.target.value } : x)))} />
              <button type="button" className="h-[34px] px-2 text-[#B21D38]" onClick={() => setEnv(env.filter((_, j) => j !== i))}>remover</button>
            </div>
          ))}
          <button type="button" className="mt-2 text-[11.67px] font-bold tracking-[1.5px] text-[#1351B4] uppercase" onClick={() => setEnv([...env, { k: "", v: "" }])}>+ adicionar variável</button>

          <p className={LABEL}>ARMAZENAMENTO PERSISTENTE:</p>
          <div className={RADIO_ROW}>
            <label className="flex items-center gap-2"><input type="radio" name="persist" checked={persist} onChange={() => setPersist(true)} /> SIM</label>
            <label className="flex items-center gap-2"><input type="radio" name="persist" checked={!persist} onChange={() => setPersist(false)} /> NÃO</label>
          </div>
          {persist && (
            <input className={INPUT} type="number" min={1} max={role === "pesquisador" ? 100 : 20} value={persistGb} onChange={(e) => setPersistGb(Number(e.target.value))} placeholder="Tamanho em GB" />
          )}

          <button type="button" className={LABEL + " text-[#1351B4]"} onClick={() => setShowAdvanced(!showAdvanced)}>AVANÇADO {showAdvanced ? "▲" : "▼"}</button>
          {showAdvanced && (
            <>
              <p className={LABEL}>COMANDO (OPCIONAL):</p>
              <input className={INPUT} value={command} onChange={(e) => setCommand(e.target.value)} placeholder="ex.: python app.py" />
            </>
          )}

          <p className={LABEL}>DESCRIÇÃO DO CONTAINER:</p>
          <input className={INPUT} value={description} onChange={(e) => setDescription(e.target.value)} />

          {error && <p className="mt-3 text-[13px] text-[#E52207]">{error}</p>}
        </section>

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link href="/containers" className="flex h-[39px] w-[86px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38]">
            Voltar
          </Link>
          <button type="button" onClick={handleCriar} disabled={loading} className="flex h-[39px] w-[86px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white disabled:opacity-60">
            {loading ? "..." : "Criar"}
          </button>
        </div>
      </main>
    </div>
  );
}
