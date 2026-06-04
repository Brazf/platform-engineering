"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiListVMs, type VM } from "@/lib/api";

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString("pt-BR");
  } catch {
    return "-";
  }
}

export default function MaquinasVirtuaisPage() {
  const [vms, setVms] = useState<VM[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiListVMs()
      .then(setVms)
      .catch(err => setError(err instanceof Error ? err.message : "Erro ao carregar VMs."))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Máquina Virtual" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Máquina Virtual</h1>
        </div>

        {loading && (
          <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            CARREGANDO...
          </p>
        )}

        {error && (
          <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#B21D38]">
            {error.toUpperCase()}
          </p>
        )}

        <div className="flex flex-col gap-4">
          {vms.map((vm) => (
            <Link
              key={vm.id}
              href={`/maquinas-virtuais/${vm.id}`}
              className="block h-[130px] bg-white px-3 py-[25px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]"
            >
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME: {vm.name}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">SISTEMA: {vm.os_template}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DATA DE CRIAÇÃO: {formatDate(vm.created_at)}</p>
            </Link>
          ))}

          {!loading && !error && vms.length === 0 && (
            <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
              NENHUMA MÁQUINA VIRTUAL ENCONTRADA.
            </p>
          )}
        </div>

        <div className="flex h-[71px] items-center justify-center py-4">
          <Link
            href="/maquinas-virtuais/criar"
            className="flex h-[39px] w-[214px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Criar Máquina Virtual
          </Link>
        </div>
      </main>
    </div>
  );
}
