import Link from "next/link";
import { InternalCompactHeader } from "@/components";

export default function UtilizarContainerPage() {
  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalCompactHeader />

      <main className="w-full">
        <div className="flex gap-[6px] px-[7px] pt-[9px]">
          <section className="h-[641px] w-[321px] bg-white px-4 pt-[19px] shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <h1 className="text-center text-[16.8px] leading-none font-semibold text-black">CONTAINER DE DEPLOY</h1>
            <h2 className="mt-6 text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">RECURSOS DA MÁQUINA</h2>

            <div className="mt-6">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">CPU</p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]"><div className="h-4 w-[58px] rounded-[4px] bg-[#22B14C]" /></div>
            </div>

            <div className="mt-5">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">MEMÓRIA</p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]"><div className="h-4 w-[187px] rounded-[4px] bg-[#22B14C]" /></div>
            </div>

            <div className="mt-5">
              <p className="text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">ARMAZENAMENTO</p>
              <div className="mt-3 h-4 rounded-[4px] bg-[#CCCCCC]"><div className="h-4 w-[111px] rounded-[4px] bg-[#22B14C]" /></div>
            </div>

            <h2 className="mt-10 text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">VARIÁVEIS DE AMBIENTE</h2>
            <p className="mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">NODE_ENV: PROD</p>
            <p className="mt-2 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase">PORT: 3000</p>

            <h2 className="mt-8 text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">ENDPOINT</h2>
            <p className="mt-4 text-center text-[16.8px] leading-[23px] font-normal text-black">http://localhost:3000</p>
          </section>

          <section className="h-[641px] w-[937px] bg-black/90 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
            <div className="h-[45px] bg-[#E5E5E5] px-[17px] py-[11px] text-[33px] leading-[23px] text-black">Logs do Container</div>
            <div className="px-[22px] pt-5 text-white">
              <p className="text-[16.8px] leading-[normal]">[2026-02-14 23:53:01] INFO entrypoint.sh: Configuration complete; ready for start up</p>
              <p className="mt-2 text-[16.8px] leading-[normal]">[2026-02-14 23:53:02] READY nginx/1.25.3 started on 0.0.0.0:80</p>
              <p className="mt-2 text-[16.8px] leading-[normal]">[2026-02-14 23:53:10] GET / 200 OK - Mozilla/5.0 (Windows NT 10.0)</p>
            </div>
          </section>
        </div>

        <div className="flex h-[71px] items-center justify-center gap-2 py-4">
          <Link
            href="/containers/container-de-deploy"
            className="flex h-[39px] w-[231px] items-center justify-center rounded-[20px] border border-[#B21D38] text-[16.8px] leading-none font-semibold text-[#B21D38]"
          >
            Reiniciar Container
          </Link>
          <Link
            href="/containers"
            className="flex h-[39px] w-[230px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white"
          >
            Desligar Container
          </Link>
        </div>
      </main>
    </div>
  );
}
