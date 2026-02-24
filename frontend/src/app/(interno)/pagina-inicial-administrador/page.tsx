import { ActionCard, InternalBreadcrumb, InternalHeader } from "@/components";

export default function PaginaInicialAdministradorPage() {
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

        <div className="grid grid-cols-3 gap-5">
          <ActionCard icon="desktop" label="Máquina Virtual" href="/maquinas-virtuais" height="134" />
          <ActionCard icon="container" label="Container" href="/containers" height="134" />
          <ActionCard icon="userPlus" label="Cadastrar novo usuário" href="/cadastrar-usuario" height="134" />
        </div>
      </main>
    </div>
  );
}
