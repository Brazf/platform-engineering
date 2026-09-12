"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { InternalCompactHeader } from "@/components";
import { apiGetContainer, containerWsURL, STATUS_LABEL, type Container } from "@/lib/api";
import "@xterm/xterm/css/xterm.css";

const LABEL = "text-[11px] leading-[16px] font-bold tracking-[1.2px] text-[#555] uppercase";
const VALUE = "mt-[2px] text-[14px] leading-[20px] text-black break-all";
const PANEL_HEAD = "flex h-10 shrink-0 items-center justify-between bg-[#E5E5E5] px-4 text-[14px] font-semibold text-black";
const DOT: Record<Container["status"]["phase"], string> = {
  running: "#1FA433", pending: "#F5A623", failed: "#FA1D1D", unknown: "#FA1D1D",
};

export default function UtilizarContainerPage() {
  const params = useParams();
  const id = params?.id as string;
  const terminalRef = useRef<HTMLDivElement>(null);
  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [c, setC] = useState<Container | null>(null);
  const [lines, setLines] = useState<string[]>([]);
  const [termState, setTermState] = useState<"connecting" | "open" | "closed">("connecting");
  const [logState, setLogState] = useState<"connecting" | "open" | "closed">("connecting");

  useEffect(() => {
    if (!id) return;
    apiGetContainer(id).then(setC).catch(() => setC(null));
  }, [id]);

  // Terminal (exec no pod), mesmo ciclo de reconexão do terminal de VM.
  useEffect(() => {
    if (!terminalRef.current || !id) return;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let term: any = null;
    let observer: ResizeObserver | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;

    async function init() {
      const { Terminal } = await import("@xterm/xterm");
      const { FitAddon } = await import("@xterm/addon-fit");

      if (cancelled || !terminalRef.current) return;

      term = new Terminal({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        theme: { background: "#111111", foreground: "#f8f8f8", cursor: "#00E5FF", selectionBackground: "#1351B4" },
        scrollback: 2000,
      });

      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(terminalRef.current);
      fitAddon.fit();

      const wsURL = containerWsURL(id, "terminal");
      const MAX_RETRIES = 60;
      let attempt = 0;

      function connect() {
        if (cancelled) return;
        setTermState("connecting");
        const ws = new WebSocket(wsURL);
        wsRef.current = ws;
        ws.binaryType = "arraybuffer";

        ws.onopen = () => {
          attempt = 0;
          setTermState("open");
          fitAddon.fit();
          ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
        };

        ws.onmessage = (evt: MessageEvent) => {
          if (typeof evt.data === "string") term?.write(evt.data);
          else term?.write(new Uint8Array(evt.data as ArrayBuffer));
        };

        ws.onclose = () => {
          if (cancelled) return;
          if (attempt >= MAX_RETRIES) {
            setTermState("closed");
            term?.writeln("\r\n\r\nConexão encerrada.");
            return;
          }
          attempt += 1;
          setTermState("connecting");
          term?.writeln(`\r\n\r\nConexão perdida. Reconectando (${attempt})...`);
          reconnectTimer = setTimeout(connect, 3000);
        };

        ws.onerror = () => {
          // onclose cuida da reconexão
        };
      }

      term.onData((data: string) => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
      });

      term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", cols, rows }));
      });

      connect();

      // Reajusta o grid do xterm sempre que o painel muda de tamanho (janela ou layout).
      observer = new ResizeObserver(() => fitAddon.fit());
      observer.observe(terminalRef.current);
    }

    init();

    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      wsRef.current?.close();
      wsRef.current = null;
      observer?.disconnect();
      term?.dispose();
    };
  }, [id]);

  // Logs em tempo real (só leitura).
  useEffect(() => {
    if (!id) return;
    let ws: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;
    let attempt = 0;

    function connect() {
      if (cancelled) return;
      setLogState("connecting");
      ws = new WebSocket(containerWsURL(id, "logs"));
      ws.onopen = () => { attempt = 0; setLogState("open"); };
      ws.onmessage = (evt: MessageEvent) => {
        setLines((l) => [...l.slice(-1999), String(evt.data)]);
      };
      ws.onclose = () => {
        if (cancelled) return;
        if (attempt >= 60) { setLogState("closed"); return; }
        attempt += 1;
        timer = setTimeout(connect, 3000);
      };
    }
    connect();

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      ws?.close();
    };
  }, [id]);

  useEffect(() => {
    logRef.current?.scrollTo(0, logRef.current.scrollHeight);
  }, [lines]);

  const envKeys = c ? Object.keys(c.env ?? {}) : [];
  const stateLabel = { connecting: "conectando...", open: "conectado", closed: "encerrado" };

  return (
    <div className="flex h-screen flex-col bg-[#F8F8F8]">
      <InternalCompactHeader />

      <main className="flex min-h-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-1 flex-col gap-3 p-3 lg:flex-row">
          {/* Painel lateral */}
          <aside className="shrink-0 overflow-y-auto bg-white px-5 py-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)] lg:w-[300px]">
            <div className="flex items-center justify-between gap-2">
              <h1 className="truncate text-[18px] leading-none font-semibold text-black">{c?.name ?? "Container"}</h1>
              {c && (
                <span className="flex shrink-0 items-center gap-2 text-[11px] font-bold tracking-[1px] text-[#555] uppercase">
                  {STATUS_LABEL[c.status.phase]}
                  <span className="h-3 w-3 rounded-full" style={{ backgroundColor: DOT[c.status.phase] }} />
                </span>
              )}
            </div>
            {c?.description && <p className="mt-1 text-[13px] leading-[18px] text-[#6B6B6B]">{c.description}</p>}

            <div className="mt-5 grid grid-cols-2 gap-x-4 gap-y-3 lg:grid-cols-1">
              <div>
                <p className={LABEL}>Imagem</p>
                <p className={VALUE}>{c?.image ?? "—"}</p>
              </div>
              <div>
                <p className={LABEL}>Recursos</p>
                <p className={VALUE}>
                  {c ? `${c.cpu_millis / 1000} vCPU · ${c.memory_mb >= 1024 ? `${c.memory_mb / 1024} GB` : `${c.memory_mb} MB`}` : "—"}
                </p>
              </div>
              <div>
                <p className={LABEL}>Armazenamento</p>
                <p className={VALUE}>{c ? (c.persistent_gb > 0 ? `${c.persistent_gb} GB em ${c.mount_path}` : "sem volume") : "—"}</p>
              </div>
              <div>
                <p className={LABEL}>Endereço externo</p>
                <p className={VALUE}>{c?.external_url || "não exposto"}</p>
              </div>
              <div>
                <p className={LABEL}>Endereço interno</p>
                <p className={VALUE}>{c?.internal_address || "—"}</p>
              </div>
              <div className="col-span-2 lg:col-span-1">
                <p className={LABEL}>Variáveis de ambiente</p>
                {envKeys.length === 0 ? (
                  <p className={VALUE}>nenhuma</p>
                ) : (
                  <ul className="mt-[2px] flex flex-col gap-[2px]">
                    {envKeys.map((k) => (
                      <li key={k} className="text-[13px] leading-[18px] text-black break-all">
                        <span className="font-mono">{k}</span> <span className="text-[#9A9A9A]">••••</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>

            <Link
              href={`/containers/${id}`}
              className="mt-6 flex h-[36px] items-center justify-center rounded-[20px] border border-[#1351B4] text-[14px] font-semibold text-[#1351B4] hover:bg-[#EEF3FB]"
            >
              Voltar aos detalhes
            </Link>
          </aside>

          {/* Terminal + logs */}
          <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
            <section className="flex min-h-[260px] flex-[3] flex-col overflow-hidden bg-[#111111] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <div className={PANEL_HEAD}>
                <span>Terminal</span>
                <span className="text-[12px] font-normal text-[#555]">{stateLabel[termState]}</span>
              </div>
              <div ref={terminalRef} className="min-h-0 flex-1 p-1" />
            </section>

            <section className="flex min-h-[160px] flex-[2] flex-col overflow-hidden bg-[#1A1A1A] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
              <div className={PANEL_HEAD}>
                <span>Logs</span>
                <span className="text-[12px] font-normal text-[#555]">{stateLabel[logState]}</span>
              </div>
              <div ref={logRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-2 font-mono text-[12.5px] leading-[18px] text-[#E6E6E6]">
                {lines.length === 0 && <p className="text-[#8A8A8A]">Sem saída ainda. As linhas aparecem aqui assim que o container escrever algo.</p>}
                {lines.map((l, i) => (
                  <p key={i} className="whitespace-pre-wrap break-all">{l}</p>
                ))}
              </div>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}
