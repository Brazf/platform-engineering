import Link from "next/link";
import { InternalCompactHeader } from "@/components";

export default function UtilizarMaquinaVirtualPage() {
  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalCompactHeader />

      <main className="w-full">
        <div className="flex px-[7px] pt-[9px]">
          <section className="h-[641px] w-[321px] bg-white px-4 pt-[19px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <h1 className="text-center text-[16.8px] leading-none font-semibold text-black">MÁQUINA DE TESTE</h1>
            <h2 className="mt-6 text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">UTILIZAÇÃO</h2>

            <div className="mt-6">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">CPU (2 NÚCLEOS)</p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]">
                <div className="h-4 w-[58px] rounded-[4px] bg-[#22B14C]" />
              </div>
            </div>

            <div className="mt-5">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">MEMÓRIA (4GB)</p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]">
                <div className="h-4 w-[187px] rounded-[4px] bg-[#22B14C]" />
              </div>
            </div>

            <div className="mt-5">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">ARMAZENAMENTO (64GB)</p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]">
                <div className="h-4 w-[111px] rounded-[4px] bg-[#22B14C]" />
              </div>
            </div>
          </section>

          <section className="h-[641px] w-[937px] bg-black/90 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <div className="h-[45px] bg-[#E5E5E5] px-[14px] py-[11px] text-[16.8px] leading-none text-black">Terminal de Acesso Remoto - Windows PowerShell</div>
            <div className="px-[17px] pt-5 text-white">
              <p className="text-[16.8px] leading-none">Windows PowerShell</p>
              <p className="mt-6 text-[16.8px] leading-none">Copyright (C) Microsoft Corporation. All rights reserverd</p>
              <p className="mt-16 text-[16.8px] leading-none text-[#00E5FF]">PS C:\Users\Admin&gt; _</p>
            </div>
          </section>
        </div>

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link
            href="/maquinas-virtuais/maquina-de-teste"
            className="flex h-[39px] w-[214px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38]"
          >
            Reiniciar Máquina
          </Link>
          <Link
            href="/maquinas-virtuais"
            className="flex h-[39px] w-[213px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Desligar Máquina
          </Link>
        </div>
      </main>
    </div>
  );
}
