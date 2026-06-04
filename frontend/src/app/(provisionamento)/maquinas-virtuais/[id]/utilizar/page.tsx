"use client";

import { useEffect, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { InternalCompactHeader } from "@/components";
import "@xterm/xterm/css/xterm.css";

export default function UtilizarMaquinaVirtualPage() {
  const params = useParams();
  const id = params?.id as string;
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!terminalRef.current || !id) return;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let term: any = null;
    let resizeHandler: (() => void) | null = null;

    async function init() {
      const { Terminal } = await import("@xterm/xterm");
      const { FitAddon } = await import("@xterm/addon-fit");

      if (!terminalRef.current) return;

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
      // Usa o mesmo host/porta da página — em dev vai para :8080 diretamente,
      // em produção (Docker) passa pelo nginx na porta 80.
      const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
      const wsURL = `${wsProto}//${window.location.host}/api/vms/${id}/terminal${
        token ? `?token=${encodeURIComponent(token)}` : ""
      }`;

      const ws = new WebSocket(wsURL);
      wsRef.current = ws;
      ws.binaryType = "arraybuffer";

      ws.onopen = () => {
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
        term?.writeln("\r\n\r\nConexão encerrada.");
      };

      ws.onerror = () => {
        term?.writeln("\r\nErro na conexão WebSocket.");
      };

      term.onData((data: string) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
      });

      resizeHandler = () => fitAddon.fit();
      window.addEventListener("resize", resizeHandler);
    }

    init();

    return () => {
      wsRef.current?.close();
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

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link
            href={`/maquinas-virtuais/${id}`}
            className="flex h-[39px] w-[214px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38]"
          >
            Reiniciar Máquina
          </Link>
          <Link
            href="/maquinas-virtuais"
            className="flex h-[39px] w-[213px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Desligar Máquina
          </Link>
        </div>
      </main>
    </div>
  );
}
