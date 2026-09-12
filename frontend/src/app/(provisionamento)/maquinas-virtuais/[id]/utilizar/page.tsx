"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { CONSOLE, FORM, InternalCompactHeader, STATUS_DOT } from "@/components";
import { apiGetVM, apiRebootVM, apiStopVM, VM_STATUS_LABEL, type VM } from "@/lib/api";
import "@xterm/xterm/css/xterm.css";

type ConnState = "connecting" | "open" | "closed";
const STATE_LABEL: Record<ConnState, string> = { connecting: "conectando...", open: "conectado", closed: "encerrado" };

export default function UtilizarMaquinaVirtualPage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [vm, setVm] = useState<VM | null>(null);
  const [termState, setTermState] = useState<ConnState>("connecting");
  const [acting, setActing] = useState<"reboot" | "stop" | null>(null);
  const [actionError, setActionError] = useState("");

  useEffect(() => {
    if (!id) return;
    apiGetVM(id).then(setVm).catch(() => setVm(null));
  }, [id]);

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
    let observer: ResizeObserver | null = null;
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
        theme: { background: "#111111", foreground: "#f8f8f8", cursor: "#00E5FF", selectionBackground: "#1351B4" },
        scrollback: 2000,
      });
      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(terminalRef.current);
      fitAddon.fit();

      const token = typeof window !== "undefined" ? localStorage.getItem("token") : null;
      // Dev: define NEXT_PUBLIC_API_BASE=http://localhost:8080 em .env.local
      // Docker (nginx proxy): API_BASE vazio → usa window.location.host na porta 80
      const apiBase = process.env.NEXT_PUBLIC_API_BASE ?? "";
      const wsBase = apiBase
        ? apiBase.replace(/^http/, "ws")
        : `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`;
      const wsURL = `${wsBase}/api/vms/${id}/terminal${token ? `?token=${encodeURIComponent(token)}` : ""}`;

      // Reconecta automaticamente: em reboot/desligamento o QEMU encerra, o console
      // serial cai e o WebSocket fecha; ao religar, a próxima tentativa volta a funcionar.
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
          term?.writeln("Conectando ao terminal da VM...");
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
        ws.onerror = () => { /* onclose cuida da reconexão */ };
      }

      term.onData((data: string) => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
      });

      connect();
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

  return (
    <div className="flex h-screen flex-col bg-[#F8F8F8]">
      <InternalCompactHeader />

      <main className="flex min-h-0 flex-1 flex-col gap-3 p-3 lg:flex-row">
        <aside className="shrink-0 overflow-y-auto bg-white px-5 py-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)] lg:w-[320px]">
          <div className="flex items-center justify-between gap-2">
            <h1 className="truncate text-[20px] leading-none font-semibold text-black">{vm?.name ?? "Máquina virtual"}</h1>
            {vm && (
              <span className="flex shrink-0 items-center gap-2 text-[11px] font-bold tracking-[1px] text-[#555] uppercase">
                {VM_STATUS_LABEL[vm.status]}
                <span className="h-3 w-3 rounded-full" style={{ backgroundColor: STATUS_DOT[vm.status] }} />
              </span>
            )}
          </div>

          <div className="mt-5 grid grid-cols-2 gap-x-4 gap-y-4 lg:grid-cols-1">
            <div><p className={CONSOLE.label}>Sistema</p><p className={CONSOLE.value}>{vm?.os_template ?? "—"}</p></div>
            <div><p className={CONSOLE.label}>Recursos</p><p className={CONSOLE.value}>{vm ? `${vm.cpu_cores} ${vm.cpu_cores === 1 ? "núcleo" : "núcleos"}, ${vm.memory_mb / 1024} GB` : "—"}</p></div>
            <div><p className={CONSOLE.label}>Disco</p><p className={CONSOLE.value}>{vm ? `${vm.disk_gb} GB` : "—"}</p></div>
            <div><p className={CONSOLE.label}>GPU</p><p className={CONSOLE.value}>{vm ? (vm.require_gpu ? "sim" : "não") : "—"}</p></div>
            <div><p className={CONSOLE.label}>ID no Proxmox</p><p className={CONSOLE.value}>{vm?.proxmox_vm_id ?? "—"}</p></div>
          </div>

          <div className="mt-6 flex flex-col gap-2">
            <button type="button" onClick={handleReboot} disabled={acting !== null} className={FORM.outlineBtn + " h-10 text-[15px]"}>
              {acting === "reboot" ? "Reiniciando..." : "Reiniciar máquina"}
            </button>
            <button type="button" onClick={handleStop} disabled={acting !== null} className={FORM.dangerBtn + " h-10 text-[15px]"}>
              {acting === "stop" ? "Desligando..." : "Desligar máquina"}
            </button>
            {actionError && <p className="text-[13px] leading-[18px] text-[#B21D38]">{actionError}</p>}
            <Link href={`/maquinas-virtuais/${id}`} className={FORM.linkBtn + " mt-2 text-center"}>Voltar aos detalhes</Link>
          </div>
        </aside>

        <section className="flex min-h-[300px] min-w-0 flex-1 flex-col overflow-hidden bg-[#111111] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <div className={CONSOLE.panelHead}><span>Terminal de acesso remoto</span><span className={CONSOLE.panelState}>{STATE_LABEL[termState]}</span></div>
          <div ref={terminalRef} className="min-h-0 flex-1 p-1" />
        </section>
      </main>
    </div>
  );
}
