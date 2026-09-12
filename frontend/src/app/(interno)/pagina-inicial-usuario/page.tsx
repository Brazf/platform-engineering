"use client";

import { useEffect, useState } from "react";
import { ActionCard, InternalBreadcrumb, InternalHeader } from "@/components";
import { getRole } from "@/lib/api";

export default function PaginaInicialUsuarioPage() {
  const [role, setRole] = useState<string | null>(null);
  useEffect(() => { setRole(getRole()); }, []);

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Página inicial" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Página inicial</h1>
        </div>

        <div className="flex gap-5">
          <ActionCard icon="desktop" label="Máquina Virtual" href="/maquinas-virtuais" height="121" />
          <ActionCard icon="container" label="Container" href="/containers" height="121" />
          {/* Só pesquisadores cadastram usuários (alunos concluem o cadastro no primeiro acesso). */}
          {role === "pesquisador" && (
            <ActionCard icon="userPlus" label="Cadastrar novo usuário" href="/cadastrar-usuario" height="121" />
          )}
        </div>
      </main>
    </div>
  );
}
