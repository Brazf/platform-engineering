import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";

export default function CriarContainerPage() {
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

        <section className="h-[484px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES DO CONTAINER</p>

          <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME DA CONTAINER:</p>
          <input className="mt-1 h-7 w-[623px] border border-black bg-transparent px-2" />

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">IMAGEM DO CONTAINER</p>
          <input className="mt-1 h-[34px] w-full border border-black bg-transparent px-2" />

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">ARMAZENAMENTO PERSISTENTE:</p>
          <div className="mt-3 flex gap-10 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            <label className="flex items-center gap-2"><input type="radio" name="persist" /> SIM</label>
            <label className="flex items-center gap-2"><input type="radio" name="persist" /> NÃO</label>
          </div>

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">REQUER GPU:</p>
          <div className="mt-3 flex gap-10 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            <label className="flex items-center gap-2"><input type="radio" name="gpu" /> SIM</label>
            <label className="flex items-center gap-2"><input type="radio" name="gpu" /> NÃO</label>
          </div>

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DESCRIÇÃO DO CONTAINER:</p>
          <input className="mt-1 h-[34px] w-full border border-black bg-transparent px-2" />

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DEFINIÇÃO DE PORTAS:</p>
          <input className="mt-1 h-[34px] w-full border border-black bg-transparent px-2" />
        </section>

        <div className="flex h-[71px] items-center justify-center py-4">
          <Link href="/containers" className="flex h-[39px] w-[86px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white">
            Criar
          </Link>
        </div>
      </main>
    </div>
  );
}
