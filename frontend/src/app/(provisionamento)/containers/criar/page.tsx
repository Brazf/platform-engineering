"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { FORM, FormActions, FormCard, Group, InternalBreadcrumb, InternalHeader, Segmented } from "@/components";
import { apiCreateContainer, apiGetCatalog, getRole, type CatalogImage } from "@/lib/api";

const CPU_OPTIONS = [500, 1000, 2000, 4000, 8000]; // millis
const MEM_OPTIONS = [256, 512, 1024, 2048, 4096, 8192, 16384];

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

        <FormCard
          title="NOVO CONTAINER"
          subtitle={<>Perfil <span className="font-semibold text-black">{role}</span>: até {maxCpu / 1000} vCPU, {maxMem / 1024} GB de memória e {maxGb} GB de armazenamento por container.</>}
        >
          <Group title="Identificação">
            <div>
              <label className={FORM.label} htmlFor="name">Nome</label>
              <input id="name" className={FORM.input} value={name} onChange={(e) => setName(e.target.value)} placeholder="meu-app" autoComplete="off" />
              <p className={FORM.hint}>Letras minúsculas, números e hífen. Vira o endereço interno do container.</p>
            </div>
            <div>
              <label className={FORM.label} htmlFor="desc">Descrição</label>
              <input id="desc" className={FORM.input} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Para que serve este container" />
            </div>
          </Group>

          <Group title="Imagem">
            <div>
              <label className={FORM.label} htmlFor="image">Catálogo</label>
              <select id="image" className={FORM.input} value={catalogKey} onChange={(e) => setCatalogKey(e.target.value)}>
                {catalog.map((c) => (
                  <option key={c.key} value={c.key}>{c.label} ({c.image})</option>
                ))}
                <option value="outra">Outra imagem...</option>
              </select>
            </div>
            {catalogKey === "outra" ? (
              <div>
                <label className={FORM.label} htmlFor="custom">Imagem</label>
                <input id="custom" className={FORM.input} placeholder="registry/imagem:tag" value={customImage} onChange={(e) => setCustomImage(e.target.value)} />
                <p className={FORM.hint}>Imagem pública do Docker Hub ou de outro registry, sempre com a tag.</p>
              </div>
            ) : (
              <div>
                <p className={FORM.label}>Volume padrão</p>
                <p className={FORM.value}>{selected?.mount_path ?? "—"}</p>
                <p className={FORM.hint}>Caminho onde o armazenamento persistente é montado.</p>
              </div>
            )}
          </Group>

          <Group title="Recursos">
            <div>
              <label className={FORM.label} htmlFor="cpu">CPU</label>
              <select id="cpu" className={FORM.input} value={cpu} onChange={(e) => setCpu(Number(e.target.value))}>
                {CPU_OPTIONS.filter((v) => v <= maxCpu).map((v) => (
                  <option key={v} value={v}>{v / 1000} vCPU</option>
                ))}
              </select>
            </div>
            <div>
              <label className={FORM.label} htmlFor="mem">Memória</label>
              <select id="mem" className={FORM.input} value={mem} onChange={(e) => setMem(Number(e.target.value))}>
                {MEM_OPTIONS.filter((v) => v <= maxMem).map((v) => (
                  <option key={v} value={v}>{v >= 1024 ? `${v / 1024} GB` : `${v} MB`}</option>
                ))}
              </select>
            </div>
          </Group>

          <Group title="Rede">
            <div>
              <p className={FORM.label}>Exposição</p>
              <Segmented name="exposição" value={expose} onChange={setExpose}
                options={[{ v: true, label: "Expor porta" }, { v: false, label: "Não expor" }]} />
              <p className={FORM.hint}>{expose ? "O container recebe um endereço de acesso." : "Acesso só pelo terminal."}</p>
            </div>
            {expose && (
              <div className="flex flex-col gap-5">
                <div>
                  <label className={FORM.label} htmlFor="port">Porta do serviço</label>
                  <input id="port" className={FORM.input + " max-w-[180px]"} type="number" min={1} max={65535} value={port} onChange={(e) => setPort(Number(e.target.value))} />
                </div>
                <div>
                  <p className={FORM.label}>Protocolo</p>
                  <Segmented name="protocolo" value={protocol} onChange={setProtocol}
                    options={[{ v: "http", label: "HTTP (site ou API)" }, { v: "tcp", label: "TCP (banco, outro)" }]} />
                </div>
              </div>
            )}
          </Group>

          <Group title="Variáveis de ambiente">
            <div className="md:col-span-2">
              {env.length === 0 && <p className={FORM.hint + " mt-0"}>Nenhuma variável. Adicione se a imagem precisar de configuração.</p>}
              <div className="flex flex-col gap-2">
                {env.map((e, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <input className={FORM.input + " md:w-2/5"} placeholder="NOME" value={e.k} readOnly={required.has(e.k)}
                      onChange={(ev) => setEnv(env.map((x, j) => (j === i ? { ...x, k: ev.target.value.toUpperCase() } : x)))} />
                    <input className={FORM.input + " flex-1"} placeholder={required.has(e.k) ? "obrigatória" : "valor"} value={e.v}
                      type={/PASS|SECRET|TOKEN/.test(e.k) ? "password" : "text"}
                      onChange={(ev) => setEnv(env.map((x, j) => (j === i ? { ...x, v: ev.target.value } : x)))} />
                    <button type="button" aria-label="Remover variável" disabled={required.has(e.k)}
                      className="h-10 w-10 shrink-0 rounded-[4px] text-[20px] leading-none text-[#B21D38] hover:bg-[#FBEDEF] disabled:opacity-30"
                      onClick={() => setEnv(env.filter((_, j) => j !== i))}>×</button>
                  </div>
                ))}
              </div>
              <button type="button" className={FORM.linkBtn + " mt-3"} onClick={() => setEnv([...env, { k: "", v: "" }])}>+ Adicionar variável</button>
            </div>
          </Group>

          <Group title="Armazenamento persistente">
            <div>
              <p className={FORM.label}>Manter dados entre reinícios</p>
              <Segmented name="armazenamento" value={persist} onChange={setPersist}
                options={[{ v: true, label: "Sim" }, { v: false, label: "Não" }]} />
            </div>
            {persist && (
              <div>
                <label className={FORM.label} htmlFor="gb">Tamanho</label>
                <div className="flex items-center gap-2">
                  <input id="gb" className={FORM.input + " max-w-[140px]"} type="number" min={1} max={maxGb} value={persistGb} onChange={(e) => setPersistGb(Number(e.target.value))} />
                  <span className="text-[15px] text-black">GB</span>
                </div>
                <p className={FORM.hint}>Até {maxGb} GB somando todos os seus containers.</p>
              </div>
            )}
          </Group>

          <div className="border-t border-[#E3E3E3] pt-5">
            <button type="button" className={FORM.linkBtn} onClick={() => setShowAdvanced(!showAdvanced)} aria-expanded={showAdvanced}>
              {showAdvanced ? "− Ocultar avançado" : "+ Avançado"}
            </button>
            {showAdvanced && (
              <div className="mt-4 md:w-1/2 md:pr-5">
                <label className={FORM.label} htmlFor="cmd">Comando de inicialização</label>
                <input id="cmd" className={FORM.input} value={command} onChange={(e) => setCommand(e.target.value)} placeholder="ex.: python app.py" />
                <p className={FORM.hint}>Substitui o comando padrão da imagem. Deixe vazio para usar o padrão.</p>
              </div>
            )}
          </div>

          {error && <p role="alert" className={FORM.error}>{error}</p>}
        </FormCard>

        <FormActions cancelHref="/containers" submitLabel="Criar container" loadingLabel="Criando..." loading={loading} onSubmit={handleCriar} />
      </main>
    </div>
  );
}
