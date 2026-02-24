import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";

type VmItem = {
  id: string;
  nome: string;
  descricao: string;
  dataCriacao: string;
};

const vms: VmItem[] = [
  {
    id: "maquina-de-teste",
    nome: "MÁQUINA DE TESTE",
    descricao: "CRIADA PARA REALIZAR DEPLOY DE UM APP DE TESTES",
    dataCriacao: "20/02/2014",
  },
  {
    id: "maquina-linux",
    nome: "MÁQUINA LINUX",
    descricao: "CRIADA PARA SUBIR UM SERVIDOR",
    dataCriacao: "10/02/2017",
  },
  {
    id: "microsoft-sqlserver",
    nome: "MICROSOFT SQLSERVER",
    descricao: "CRIADA PARA UM SUBIR UM BANCO SQLSERVER",
    dataCriacao: "01/01/2026",
  },
];

export default function MaquinasVirtuaisPage() {
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

        <div className="flex flex-col gap-4">
          {vms.map((vm) => (
            <Link
              key={vm.id}
              href={`/maquinas-virtuais/${vm.id}`}
              className="block h-[130px] bg-white px-3 py-[25px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]"
            >
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME: {vm.nome}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DESCRIÇÃO: {vm.descricao}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DATA DE CRIAÇÃO: {vm.dataCriacao}</p>
            </Link>
          ))}
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
