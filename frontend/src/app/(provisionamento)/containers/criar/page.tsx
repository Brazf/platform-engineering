"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiCreateContainer, apiGetCatalog, getRole, type CatalogImage } from "@/lib/api";

const LABEL = "mb-1 block text-[11px] leading-[16px] font-bold tracking-[1.2px] text-[#555] uppercase";
const INPUT =
  "h-9 w-full rounded-[4px] border border-[#8F8F8F] bg-white px-3 text-[14px] text-black placeholder:text-[#9A9A9A] focus:border-[#1351B4] focus:outline-none focus:ring-2 focus:ring-[#1351B4]/20";
const HINT = "mt-1 text-[12px] leading-[16px] text-[#6B6B6B]";
const GROUP_TITLE = "text-[13px] leading-[16px] font-semibold text-[#1351B4]";
const LINK_BTN = "text-[12px] font-semibold text-[#1351B4] hover:underline";

const CPU_OPTIONS = [500, 1000, 2000, 4000, 8000]; // millis
const MEM_OPTIONS = [256, 512, 1024, 2048, 4096, 8192, 16384];

function Segmented<T extends string | boolean>({
  value, options, onChange, name,
}: { value: T; options: { v: T; label: string }[]; onChange: (v: T) => void; name: string }) {
  return (
    <div role="radiogroup" aria-label={name} className="inline-flex overflow-hidden rounded-[4px] border border-[#8F8F8F]">
      {options.map((o, i) => {
        const active = o.v === value;
        return (
          <button
            key={String(o.v)}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.v)}
            className={
              "h-9 px-4 text-[13px] font-semibold transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-[#1351B4]/40 " +
              (i > 0 ? "border-l border-[#8F8F8F] " : "") +
              (active ? "bg-[#1351B4] text-white" : "bg-white text-[#1351B4] hover:bg-[#EEF3FB]")
            }
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="border-t border-[#E3E3E3] pt-4 first:border-t-0 first:pt-0">
      <p className={GROUP_TITLE}>{title}</p>
      <div className="mt-3 grid grid-cols-1 gap-x-8 gap-y-4 md:grid-cols-2">{children}</div>
    </div>
  );
}

export default function CriarContainerPage() {
  const router = useRouter();
  const [role, setRole] = useState("aluno");
  const maxCpu = role === "pesquisador" ? 8000 : 2000;
  const maxMem = role === "pesquisador" ? 16384 : 4096;
  const maxGb = role === "pesquisador" ? 100 : 20;

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

  const required = new Set(selected?.required_env ?? []);

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

        <section className="mx-auto max-w-[960px] bg-white px-8 pt-6 pb-8 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">NOVO CONTAINER</p>
          <p className="mt-1 mb-6 text-[13px] text-[#6B6B6B]">
            Perfil <span className="font-semibold text-black">{role}</span>: até {maxCpu / 1000} vCPU, {maxMem / 1024} GB de memória e {maxGb} GB de armazenamento por container.
          </p>

          <div className="flex flex-col gap-6">
            <Group title="Identificação">
              <div>
                <label className={LABEL} htmlFor="name">Nome</label>
                <input id="name" className={INPUT} value={name} onChange={(e) => setName(e.target.value)} placeholder="meu-app" autoComplete="off" />
                <p className={HINT}>Letras minúsculas, números e hífen. Vira o endereço interno do container.</p>
              </div>
              <div>
                <label className={LABEL} htmlFor="desc">Descrição</label>
                <input id="desc" className={INPUT} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Para que serve este container" />
              </div>
            </Group>

            <Group title="Imagem">
              <div>
                <label className={LABEL} htmlFor="image">Catálogo</label>
                <select id="image" className={INPUT} value={catalogKey} onChange={(e) => setCatalogKey(e.target.value)}>
                  {catalog.map((c) => (
                    <option key={c.key} value={c.key}>{c.label} ({c.image})</option>
                  ))}
                  <option value="outra">Outra imagem...</option>
                </select>
              </div>
              {catalogKey === "outra" ? (
                <div>
                  <label className={LABEL} htmlFor="custom">Imagem</label>
                  <input id="custom" className={INPUT} placeholder="registry/imagem:tag" value={customImage} onChange={(e) => setCustomImage(e.target.value)} />
                  <p className={HINT}>Imagem pública do Docker Hub ou de outro registry, sempre com a tag.</p>
                </div>
              ) : (
                <div>
                  <label className={LABEL}>Volume padrão</label>
                  <p className="flex h-9 items-center text-[14px] text-black">{selected?.mount_path ?? "—"}</p>
                  <p className={HINT}>Caminho onde o armazenamento persistente é montado.</p>
                </div>
              )}
            </Group>

            <Group title="Recursos">
              <div>
                <label className={LABEL} htmlFor="cpu">CPU</label>
                <select id="cpu" className={INPUT} value={cpu} onChange={(e) => setCpu(Number(e.target.value))}>
                  {CPU_OPTIONS.filter((v) => v <= maxCpu).map((v) => (
                    <option key={v} value={v}>{v / 1000} vCPU</option>
                  ))}
                </select>
              </div>
              <div>
                <label className={LABEL} htmlFor="mem">Memória</label>
                <select id="mem" className={INPUT} value={mem} onChange={(e) => setMem(Number(e.target.value))}>
                  {MEM_OPTIONS.filter((v) => v <= maxMem).map((v) => (
                    <option key={v} value={v}>{v >= 1024 ? `${v / 1024} GB` : `${v} MB`}</option>
                  ))}
                </select>
              </div>
            </Group>

            <Group title="Rede">
              <div>
                <label className={LABEL}>Exposição</label>
                <Segmented
                  name="exposição"
                  value={expose}
                  onChange={setExpose}
                  options={[{ v: true, label: "Expor porta" }, { v: false, label: "Não expor" }]}
                />
                <p className={HINT}>{expose ? "O container recebe um endereço de acesso." : "Acesso só pelo terminal."}</p>
              </div>
              {expose && (
                <div className="flex flex-col gap-4">
                  <div>
                    <label className={LABEL} htmlFor="port">Porta do serviço</label>
                    <input id="port" className={INPUT + " max-w-[160px]"} type="number" min={1} max={65535} value={port} onChange={(e) => setPort(Number(e.target.value))} />
                  </div>
                  <div>
                    <label className={LABEL}>Protocolo</label>
                    <Segmented
                      name="protocolo"
                      value={protocol}
                      onChange={setProtocol}
                      options={[{ v: "http", label: "HTTP (site ou API)" }, { v: "tcp", label: "TCP (banco, outro)" }]}
                    />
                  </div>
                </div>
              )}
            </Group>

            <Group title="Variáveis de ambiente">
              <div className="md:col-span-2">
                {env.length === 0 && <p className={HINT + " mt-0"}>Nenhuma variável. Adicione se a imagem precisar de configuração.</p>}
                <div className="flex flex-col gap-2">
                  {env.map((e, i) => (
                    <div key={i} className="flex items-center gap-2">
                      <input
                        className={INPUT + " md:w-2/5"}
                        placeholder="NOME"
                        value={e.k}
                        readOnly={required.has(e.k)}
                        onChange={(ev) => setEnv(env.map((x, j) => (j === i ? { ...x, k: ev.target.value.toUpperCase() } : x)))}
                      />
                      <input
                        className={INPUT + " flex-1"}
                        placeholder={required.has(e.k) ? "obrigatória" : "valor"}
                        value={e.v}
                        type={/PASS|SECRET|TOKEN/.test(e.k) ? "password" : "text"}
                        onChange={(ev) => setEnv(env.map((x, j) => (j === i ? { ...x, v: ev.target.value } : x)))}
                      />
                      <button
                        type="button"
                        aria-label="Remover variável"
                        disabled={required.has(e.k)}
                        className="h-9 w-9 shrink-0 rounded-[4px] text-[18px] leading-none text-[#B21D38] hover:bg-[#FBEDEF] disabled:opacity-30"
                        onClick={() => setEnv(env.filter((_, j) => j !== i))}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
                <button type="button" className={LINK_BTN + " mt-3"} onClick={() => setEnv([...env, { k: "", v: "" }])}>
                  + Adicionar variável
                </button>
              </div>
            </Group>

            <Group title="Armazenamento persistente">
              <div>
                <label className={LABEL}>Manter dados entre reinícios</label>
                <Segmented
                  name="armazenamento"
                  value={persist}
                  onChange={setPersist}
                  options={[{ v: true, label: "Sim" }, { v: false, label: "Não" }]}
                />
              </div>
              {persist && (
                <div>
                  <label className={LABEL} htmlFor="gb">Tamanho</label>
                  <div className="flex items-center gap-2">
                    <input id="gb" className={INPUT + " max-w-[120px]"} type="number" min={1} max={maxGb} value={persistGb} onChange={(e) => setPersistGb(Number(e.target.value))} />
                    <span className="text-[14px] text-black">GB</span>
                  </div>
                  <p className={HINT}>Até {maxGb} GB somando todos os seus containers.</p>
                </div>
              )}
            </Group>

            <div className="border-t border-[#E3E3E3] pt-4">
              <button type="button" className={LINK_BTN} onClick={() => setShowAdvanced(!showAdvanced)} aria-expanded={showAdvanced}>
                {showAdvanced ? "− Ocultar avançado" : "+ Avançado"}
              </button>
              {showAdvanced && (
                <div className="mt-3 md:w-1/2 md:pr-4">
                  <label className={LABEL} htmlFor="cmd">Comando de inicialização</label>
                  <input id="cmd" className={INPUT} value={command} onChange={(e) => setCommand(e.target.value)} placeholder="ex.: python app.py" />
                  <p className={HINT}>Substitui o comando padrão da imagem. Deixe vazio para usar o padrão.</p>
                </div>
              )}
            </div>
          </div>

          {error && (
            <p role="alert" className="mt-6 rounded-[4px] border border-[#E52207] bg-[#FDF2F1] px-3 py-2 text-[13px] text-[#B21D38]">{error}</p>
          )}
        </section>

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link href="/containers" className="flex h-[39px] w-[110px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38]">
            Cancelar
          </Link>
          <button type="button" onClick={handleCriar} disabled={loading} className="flex h-[39px] w-[160px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white disabled:opacity-60">
            {loading ? "Criando..." : "Criar container"}
          </button>
        </div>
      </main>
    </div>
  );
}
