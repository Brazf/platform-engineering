"use client";

import Link from "next/link";
import type { ReactNode } from "react";

// Estilos compartilhados dos formulários e consoles (VMs e containers usam os mesmos).
export const FORM = {
  label: "mb-1 block text-[12px] leading-[16px] font-bold tracking-[1.2px] text-[#555] uppercase",
  input:
    "h-10 w-full rounded-[4px] border border-[#8F8F8F] bg-white px-3 text-[15px] text-black placeholder:text-[#9A9A9A] focus:border-[#1351B4] focus:outline-none focus:ring-2 focus:ring-[#1351B4]/20",
  hint: "mt-1 text-[13px] leading-[18px] text-[#6B6B6B]",
  value: "flex h-10 items-center text-[15px] text-black",
  linkBtn: "text-[13px] font-semibold text-[#1351B4] hover:underline",
  error: "mt-6 rounded-[4px] border border-[#E52207] bg-[#FDF2F1] px-3 py-2 text-[14px] text-[#B21D38]",
  success: "mt-6 rounded-[4px] border border-[#168821] bg-[#E3F5E1] px-3 py-2 text-[14px] text-[#0B5B13]",
  primaryBtn:
    "flex h-[42px] items-center justify-center rounded-[21px] bg-[#1351B4] px-7 text-[16.8px] leading-none font-semibold text-white hover:bg-[#0F4499] disabled:opacity-60",
  dangerBtn:
    "flex h-[42px] items-center justify-center rounded-[21px] border border-[#B21D38] px-7 text-[16.8px] leading-none font-semibold text-[#B21D38] hover:bg-[#FBEDEF] disabled:opacity-60",
  outlineBtn:
    "flex h-[42px] items-center justify-center rounded-[21px] border border-[#1351B4] px-7 text-[16.8px] leading-none font-semibold text-[#1351B4] hover:bg-[#EEF3FB] disabled:opacity-60",
};

export const CONSOLE = {
  label: "text-[12px] leading-[16px] font-bold tracking-[1.2px] text-[#555] uppercase",
  value: "mt-[2px] text-[15px] leading-[20px] text-black break-all",
  panelHead: "flex h-10 shrink-0 items-center justify-between bg-[#E5E5E5] px-4 text-[14px] font-semibold text-black",
  panelState: "text-[12px] font-normal text-[#555]",
};

export const STATUS_DOT: Record<string, string> = {
  running: "#1FA433",
  pending: "#F5A623",
  creating: "#F5A623",
  stopped: "#8A8A8A",
  failed: "#FA1D1D",
  error: "#FA1D1D",
  unknown: "#FA1D1D",
};

/** Cartão branco do formulário: título, linha de contexto e seções. Cresce na altura se o pai esticar (ex.: grid auto-rows-fr). */
export function FormCard({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  return (
    <section className="mx-auto flex w-full max-w-[1100px] flex-col bg-white px-10 pt-7 pb-9 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
      <p className="text-[12px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">{title}</p>
      {subtitle && <p className="mt-1 text-[14px] text-[#6B6B6B]">{subtitle}</p>}
      <div className="mt-7 flex flex-1 flex-col gap-7">{children}</div>
    </section>
  );
}

/** Seção do formulário com título azul; campos em duas colunas em telas largas. */
export function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="border-t border-[#E3E3E3] pt-5 first:border-t-0 first:pt-0">
      <p className="text-[14px] leading-[16px] font-semibold text-[#1351B4]">{title}</p>
      <div className="mt-4 grid grid-cols-1 gap-x-10 gap-y-5 md:grid-cols-2">{children}</div>
    </div>
  );
}

/** Grupo de opções exclusivas em forma de botões. */
export function Segmented<T extends string | boolean>({
  value, options, onChange, name,
}: { value: T; options: { v: T; label: string }[]; onChange: (v: T) => void; name: string }) {
  return (
    <div role="radiogroup" aria-label={name} className="inline-flex flex-wrap overflow-hidden rounded-[4px] border border-[#8F8F8F]">
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
              "h-10 px-5 text-[14px] font-semibold transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-[#1351B4]/40 " +
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

/** Rodapé com Cancelar + ação principal. */
export function FormActions({
  cancelHref, cancelLabel = "Cancelar", submitLabel, loadingLabel, loading, onSubmit,
}: { cancelHref: string; cancelLabel?: string; submitLabel: string; loadingLabel: string; loading: boolean; onSubmit: () => void }) {
  return (
    <div className="flex items-center justify-center gap-3 py-6">
      <Link href={cancelHref} className={FORM.dangerBtn}>{cancelLabel}</Link>
      <button type="button" onClick={onSubmit} disabled={loading} className={FORM.primaryBtn}>
        {loading ? loadingLabel : submitLabel}
      </button>
    </div>
  );
}

// ─── Listas e detalhes ────────────────────────────────────────────────────────

/** Selo de status com bolinha colorida. */
export function StatusBadge({ label, phase, message }: { label: string; phase: string; message?: string }) {
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-[#E3E3E3] bg-[#FAFAFA] px-3 py-1 text-[11px] font-bold tracking-[1px] text-[#555] uppercase" title={message}>
      <span className="h-[10px] w-[10px] rounded-full" style={{ backgroundColor: STATUS_DOT[phase] ?? STATUS_DOT.unknown }} />
      {label}
    </span>
  );
}

/** Par rótulo/valor usado em cartões de lista e de detalhe. */
export function Field({ label, children, className = "" }: { label: string; children: ReactNode; className?: string }) {
  return (
    <div className={className}>
      <p className={CONSOLE.label}>{label}</p>
      <div className={CONSOLE.value}>{children}</div>
    </div>
  );
}

/** Item clicável da lista de recursos. */
export function ResourceCard({
  href, name, description, status, fields, createdAt,
}: {
  href: string; name: string; description?: string;
  status: { label: string; phase: string; message?: string };
  fields: { label: string; value: ReactNode }[];
  createdAt: string;
}) {
  return (
    <Link
      href={href}
      className="block bg-white px-8 py-6 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)] transition-colors hover:bg-[#FCFCFC] focus:outline-none focus-visible:ring-2 focus-visible:ring-[#1351B4]/40"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-[20px] leading-none font-semibold text-black">{name}</p>
          {description && <p className="mt-2 text-[14px] leading-[18px] text-[#6B6B6B]">{description}</p>}
        </div>
        <StatusBadge {...status} />
      </div>
      <div className="mt-5 grid grid-cols-2 gap-x-8 gap-y-4 md:grid-cols-4">
        {fields.map((f) => (
          <Field key={f.label} label={f.label}>{f.value}</Field>
        ))}
        <Field label="Criado em">{new Date(createdAt).toLocaleDateString("pt-BR")}</Field>
      </div>
    </Link>
  );
}

/** Mensagem de carregando / vazio / erro fora dos cartões. */
export function PageNotice({ kind, children }: { kind: "info" | "error"; children: ReactNode }) {
  return (
    <p className={kind === "error" ? FORM.error + " mt-0" : "text-[15px] text-[#6B6B6B]"}>{children}</p>
  );
}

/** Cabeçalho do cartão de detalhe: nome + status. */
export function DetailHeader({ name, description, status }: { name: string; description?: string; status: { label: string; phase: string; message?: string } }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <h2 className="text-[24px] leading-none font-semibold text-black">{name}</h2>
        {description && <p className="mt-2 text-[14px] leading-[18px] text-[#6B6B6B]">{description}</p>}
      </div>
      <div className="flex flex-col items-end gap-1">
        <StatusBadge {...status} />
        {status.message && <p className="max-w-[360px] text-right text-[13px] leading-[18px] text-[#6B6B6B]">{status.message}</p>}
      </div>
    </div>
  );
}
