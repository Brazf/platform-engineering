"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { InternalCompactHeader } from "@/components";
import { apiGetContainer, containerWsURL, type Container } from "@/lib/api";
import "@xterm/xterm/css/xterm.css";

const LINE = "text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase";
const H2 = "text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]";

export default function UtilizarContainerPage() {
  const params = useParams();
  const id = params?.id as string;
  const terminalRef = useRef<HTMLDivElement>(null);
  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [c, setC] = useState<Container | null>(null);
  const [lines, setLines] = useState<string[]>([]);

  useEffect(() => {
    if (!id) return;
    apiGetContainer(id).then(setC).catch(() => setC(null));
  }, [id]);

  // Terminal (exec no pod), mesmo ciclo de reconexão do terminal de VM.
  useEffect(() => {
    if (!terminalRef.current || !id) return;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let term: any = null;
    let resizeHandler: (() => void) | null = null;
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
        theme: {
          background: "#111111",
          foreground: "#f8f8f8",
          cursor: "#00E5FF",
          selectionBackground: "#1351B4",
        },
        scrollback: 1000,
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
        const ws = new WebSocket(wsURL);
        wsRef.current = ws;
        ws.binaryType = "arraybuffer";

        ws.onopen = () => {
          attempt = 0;
          term?.writeln("Conectando ao terminal do container...");
          fitAddon.fit();
          ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
        };

        ws.onmessage = (evt: MessageEvent) => {
          if (typeof evt.data === "string") {
            term?.write(evt.data);
          } else {
            term?.write(new Uint8Array(evt.data as ArrayBuffer));
          }
        };

        ws.onclose = () => {
          if (cancelled) return;
          if (attempt >= MAX_RETRIES) {
            term?.writeln("\r\n\r\nConexão encerrada.");
            return;
          }
          attempt += 1;
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

      resizeHandler = () => fitAddon.fit();
      window.addEventListener("resize", resizeHandler);
    }

    init();

    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      wsRef.current?.close();
      wsRef.current = null;
      term?.dispose();
      if (resizeHandler) window.removeEventListener("resize", resizeHandler);
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
      ws = new WebSocket(containerWsURL(id, "logs"));
      ws.onopen = () => {
        attempt = 0;
      };
      ws.onmessage = (evt: MessageEvent) => {
        setLines((l) => [...l.slice(-999), String(evt.data)]);
      };
      ws.onclose = () => {
        if (cancelled || attempt >= 60) return;
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

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalCompactHeader />

      <main className="w-full">
        <div className="flex gap-[6px] px-[7px] pt-[9px]">
          <section className="h-[641px] w-[321px] overflow-y-auto bg-white px-4 pt-[19px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <h1 className="text-center text-[16.8px] leading-none font-semibold text-black">{c?.name?.toUpperCase() ?? "CONTAINER"}</h1>
            <h2 className={"mt-6 " + H2}>RECURSOS DO CONTAINER</h2>

            <p className={"mt-6 " + LINE}>CPU: {c ? `${c.cpu_millis / 1000} vCPU` : "—"}</p>
            <p className={"mt-2 " + LINE}>MEMÓRIA: {c ? (c.memory_mb >= 1024 ? `${c.memory_mb / 1024} GB` : `${c.memory_mb} MB`) : "—"}</p>
            <p className={"mt-2 " + LINE}>ARMAZENAMENTO: {c ? (c.persistent_gb > 0 ? `${c.persistent_gb} GB` : "não") : "—"}</p>

            <h2 className={"mt-10 " + H2}>VARIÁVEIS DE AMBIENTE</h2>
            {envKeys.length === 0 && <p className={"mt-4 " + LINE}>—</p>}
            {envKeys.map((k, i) => (
              <p key={k} className={(i === 0 ? "mt-4 " : "mt-2 ") + LINE}>{k}: ••••</p>
            ))}

            <h2 className={"mt-8 " + H2}>ENDPOINT</h2>
            <p className="mt-4 text-center text-[16.8px] leading-[23px] font-normal break-all text-black">{c?.external_url || "—"}</p>
          </section>

          <section className="h-[641px] w-[937px] bg-black/90 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <div className="h-[45px] bg-[#E5E5E5] px-[17px] py-[11px] text-[16.8px] leading-[23px] text-black">Terminal</div>
            <div ref={terminalRef} className="h-[275px] overflow-hidden bg-[#111111]" style={{ padding: "4px" }} />
            <div className="h-[45px] bg-[#E5E5E5] px-[17px] py-[11px] text-[16.8px] leading-[23px] text-black">Logs do Container</div>
            <div ref={logRef} className="h-[276px] overflow-y-auto px-[22px] pt-2 text-white">
              {lines.map((l, i) => (
                <p key={i} className="text-[13px] leading-[normal] whitespace-pre-wrap break-all">{l}</p>
              ))}
            </div>
          </section>
        </div>

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link
            href={`/containers/${id}`}
            className="flex h-[39px] w-[230px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Voltar
          </Link>
        </div>
      </main>
    </div>
  );
}
