import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";

export default function ContainerDetalhePage() {
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

        <section className="h-[137px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES</p>
          <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME: CONTAINER DE DEPLOY</p>
          <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DESCRIÇÃO: CRIADA PARA REALIZAR DEPLOY DE UM APP DE TESTES</p>
          <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DATA DE CRIAÇÃO: 20/02/2014</p>
        </section>

        <section className="mt-5 h-[157px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">CONFIGURAÇÃO</p>
          <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">IMAGEM: NGINX:LATEST</p>
          <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">PORTAS: 3000:80</p>
          <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">VARIÁVEIS DE AMBIENTE: NODE_ENV=PROD</p>
          <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">ARMAZENAMENTO PERSISTENTE: SIM</p>
          <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">CONSOME GPU: NÃO</p>
        </section>

        <section className="mt-5 h-[114px]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#E52207]">ESTADO DO CONTAINER</p>
          <div className="mt-[11px] border-t border-[#CCCCCC]" />

          <div className="flex h-[39px] items-center justify-between border-b border-[#CCCCCC] px-4">
            <span className="text-[14px] leading-none text-[#1351B4]">EM EXECUÇÃO</span>
            <span className="h-7 w-7 rounded-full bg-[#1FA433]" />
          </div>

          <div className="flex h-[39px] items-center justify-between px-4">
            <span className="text-[14px] leading-none text-[#1351B4]">PARADO</span>
            <span className="h-7 w-7 rounded-full bg-[#FA1D1D]" />
          </div>
        </section>

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link
            href="/containers"
            className="flex h-[39px] w-[183px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38]"
          >
            Excluir Container
          </Link>
          <Link
            href="/containers/container-de-deploy/utilizar"
            className="flex h-[39px] w-[178px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Iniciar Container
          </Link>
        </div>
      </main>
    </div>
  );
}
