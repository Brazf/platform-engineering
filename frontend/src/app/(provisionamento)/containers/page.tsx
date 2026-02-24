import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";

type ContainerItem = {
  id: string;
  nome: string;
  descricao: string;
  dataCriacao: string;
};

const containers: ContainerItem[] = [
  {
    id: "container-de-deploy",
    nome: "CONTAINER DE DEPLOY",
    descricao: "CRIADA PARA REALIZAR DEPLOY DE UM APP DE TESTES",
    dataCriacao: "20/02/2014",
  },
  {
    id: "container-de-nginx",
    nome: "CONTAINER DE NGINX",
    descricao: "CRIADA PARA SUBIR UM SERVIDOR",
    dataCriacao: "10/02/2017",
  },
  {
    id: "container-de-postgresql",
    nome: "CONTAINER DE POSTGRESQL",
    descricao: "CRIADA PARA UM SUBIR UM BANCO POSTGRESQL",
    dataCriacao: "01/01/2026",
  },
];

export default function ContainersPage() {
  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Container" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Container</h1>
        </div>

        <div className="flex flex-col gap-4">
          {containers.map((container) => (
            <Link
              key={container.id}
              href={`/containers/${container.id}`}
              className="block h-[130px] bg-white px-3 py-[25px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]"
            >
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME: {container.nome}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DESCRIÇÃO: {container.descricao}</p>
              <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DATA DE CRIAÇÃO: {container.dataCriacao}</p>
            </Link>
          ))}
        </div>
      </main>
    </div>
  );
}
