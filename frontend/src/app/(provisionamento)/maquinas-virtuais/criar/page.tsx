import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";

function SliderScale({ min, max, current }: { min: string; max: string; current: string }) {
  return (
    <div>
      <div className="flex h-[5px] w-[408px] rounded-[2px] bg-[#23A324]">
        <div className="h-[5px] w-[168px] bg-[#E08B1D]" />
        <div className="h-[5px] w-[112px] bg-[#FA1D1D]" />
      </div>
      <div className="relative mt-1 flex w-[408px] justify-between text-[9.72px] leading-none text-[#1A1A1A]">
        <span>{min}</span>
        <span>{max}</span>
        <span className="absolute left-[120px] top-4">{current}</span>
      </div>
      <div className="relative -mt-[14px] ml-[98px] h-0 w-0 border-l-[8px] border-r-[8px] border-t-[12px] border-l-transparent border-r-transparent border-t-black" />
    </div>
  );
}

export default function CriarMaquinaVirtualPage() {
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

        <section className="h-[484px] bg-white px-3 pt-2 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
          <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES DA MÁQUINA</p>

          <p className="mt-[10px] text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NOME DA MÁQUINA:</p>
          <input className="mt-1 h-7 w-[623px] border border-black bg-transparent px-2" />

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">SISTEMA OPERACIONAL:</p>
          <div className="mt-3 flex gap-6 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            <label className="flex items-center gap-2"><input type="radio" name="so" /> WINDOWS</label>
            <label className="flex items-center gap-2"><input type="radio" name="so" /> UBUNTU</label>
            <label className="flex items-center gap-2"><input type="radio" name="so" /> DEBIAN</label>
          </div>

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE NÚCLEOS (CPUS)</p>
          <div className="mt-3"><SliderScale min="0" max="24 CPUS" current="2" /></div>

          <p className="mt-5 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE MEMÓRIA (RAM)</p>
          <div className="mt-3"><SliderScale min="0" max="16000 MB" current="4500 mb" /></div>

          <p className="mt-5 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">QUANTIDADE DE ARMAZENAMENTO (GB)</p>
          <div className="mt-3"><SliderScale min="0" max="1 TB" current="300 GB" /></div>

          <p className="mt-6 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">REQUER GPU:</p>
          <div className="mt-3 flex gap-10 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]">
            <label className="flex items-center gap-2"><input type="radio" name="gpu" /> SIM</label>
            <label className="flex items-center gap-2"><input type="radio" name="gpu" /> NÃO</label>
          </div>

          <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">DESCRIÇÃO DA MÁQUINA:</p>
          <input className="mt-1 h-[34px] w-full border border-black bg-transparent px-2" />
        </section>

        <div className="flex h-[71px] items-center justify-center py-4">
          <Link href="/maquinas-virtuais" className="flex h-[39px] w-[86px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white">
            Criar
          </Link>
        </div>
      </main>
    </div>
  );
}
