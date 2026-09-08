"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { InternalCompactHeader } from "@/components";
import { apiRebootVM, apiStopVM } from "@/lib/api";
import "@xterm/xterm/css/xterm.css";

export default function UtilizarMaquinaVirtualPage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [acting, setActing] = useState<"reboot" | "stop" | null>(null);
  const [actionError, setActionError] = useState("");

  async function handleReboot() {
    setActionError("");
    setActing("reboot");
    try {
      await apiRebootVM(id);
      // força a queda da sessão atual; o ciclo de reconexão reanexa ao console
      // quando a VM voltar, mesmo se o Proxmox demorar a fechar o socket antigo
      wsRef.current?.close();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Erro ao reiniciar a máquina.");
    } finally {
      setActing(null);
    }
  }

  async function handleStop() {
    setActionError("");
    setActing("stop");
    try {
      await apiStopVM(id);
      router.push("/maquinas-virtuais");
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Erro ao desligar a máquina.");
      setActing(null);
    }
  }

  useEffect(() => {
    if (!terminalRef.current || !id) return;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let term: any = null;
    let resizeHandler: (() => void) | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    // O cleanup pode rodar antes dos imports dinâmicos terminarem (React em dev
    // monta o efeito duas vezes); sem esta flag a primeira conexão vazava aberta.
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

      const token =
        typeof window !== "undefined" ? localStorage.getItem("token") : null;

      // Dev: define NEXT_PUBLIC_API_BASE=http://localhost:8080 em .env.local
      // Docker (nginx proxy): API_BASE vazio → usa window.location.host na porta 80
      const apiBase = process.env.NEXT_PUBLIC_API_BASE ?? "";
      const wsBase = apiBase
        ? apiBase.replace(/^http/, "ws")
        : `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`;

      const wsURL = `${wsBase}/api/vms/${id}/terminal${
        token ? `?token=${encodeURIComponent(token)}` : ""
      }`;

      // Reconecta automaticamente: em reboot/desligamento o QEMU encerra, o console
      // serial cai e o WebSocket fecha; ao religar, a próxima tentativa volta a funcionar.
      const MAX_RETRIES = 60;
      let attempt = 0;

      function connect() {
        if (cancelled) return;
        const ws = new WebSocket(wsURL);
        wsRef.current = ws;
        ws.binaryType = "arraybuffer";

        ws.onopen = () => {
          attempt = 0;
          term?.writeln("Conectando ao terminal da VM...");
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
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
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

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalCompactHeader />

      <main className="w-full">
        <div className="flex px-[7px] pt-[9px]">
          <section className="h-[641px] w-[321px] bg-white px-4 pt-[19px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <h1 className="text-center text-[16.8px] leading-none font-semibold text-black">
              MÁQUINA DE TESTE
            </h1>
            <h2 className="mt-6 text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">
              UTILIZAÇÃO
            </h2>

            <div className="mt-6">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">
                CPU (2 NÚCLEOS)
              </p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]">
                <div className="h-4 w-[58px] rounded-[4px] bg-[#22B14C]" />
              </div>
            </div>

            <div className="mt-5">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">
                MEMÓRIA (4GB)
              </p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]">
                <div className="h-4 w-[187px] rounded-[4px] bg-[#22B14C]" />
              </div>
            </div>

            <div className="mt-5">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">
                ARMAZENAMENTO (64GB)
              </p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]">
                <div className="h-4 w-[111px] rounded-[4px] bg-[#22B14C]" />
              </div>
            </div>
          </section>

          <section className="h-[641px] w-[937px] bg-black/90 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <div className="h-[45px] bg-[#E5E5E5] px-[14px] py-[11px] text-[16.8px] leading-none text-black">
              Terminal de Acesso Remoto
            </div>
            <div
              ref={terminalRef}
              className="h-[596px] overflow-hidden"
              style={{ padding: "4px" }}
            />
          </section>
        </div>

        {actionError && (
          <p className="mt-3 text-center text-[12px] leading-none text-[#B21D38]">{actionError}</p>
        )}

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <button
            type="button"
            onClick={handleReboot}
            disabled={acting !== null}
            className="flex h-[39px] w-[214px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38] disabled:opacity-60"
          >
            {acting === "reboot" ? "..." : "Reiniciar Máquina"}
          </button>
          <button
            type="button"
            onClick={handleStop}
            disabled={acting !== null}
            className="flex h-[39px] w-[213px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white disabled:opacity-60"
          >
            {acting === "stop" ? "..." : "Desligar Máquina"}
          </button>
        </div>
      </main>
    </div>
  );
}
