import Image from "next/image";
import Link from "next/link";

export default function CodigoRecuperacaoPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-white px-4 py-10 md:px-10 md:py-20">
      <div className="grid w-full max-w-[1280px] items-center gap-10 md:grid-cols-[736px_464px] md:gap-10">
        <section className="hidden md:block" aria-hidden>
          <Image src="/file.png" alt="Sprint Report - Gestão de TCC" width={736} height={414} className="h-[414px] w-[736px] object-contain" priority />
        </section>

        <section className="w-full md:w-[464px]">
          <div className="flex flex-col gap-5">
            <div className="h-[39px]">
              <Link href="/esqueceu-senha" className="text-[16.8px] leading-none font-semibold text-[#1351B4]">
                Voltar
              </Link>
            </div>

            <h1 className="pb-5 text-[42px] leading-[1.1] font-medium text-black">Código de recuperação</h1>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Código</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border-2 border-[#155BCB] bg-[#FCFCFC] pl-4 pr-1">
                <Image src="/assets/access/icon-lock.svg" alt="Ícone código" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="text"
                  placeholder="Código"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
              </div>
              <div className="flex h-6 items-center gap-1 rounded-[4px] bg-[#155BCB] px-2 text-white">
                <span className="text-[14px] font-semibold leading-none">ⓘ</span>
                <span className="text-[14px] leading-none">Digite o código recebido no seu email</span>
              </div>
            </div>

            <Link href="/nova-senha" className="mt-2 flex h-[39px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white">
              Continuar
            </Link>

            <div className="mt-8 flex h-[56px] items-center justify-between">
              <div className="flex h-[56px] w-[166px] items-center">
                <Image
                  src="/assets/access/pngteste.png"
                  alt="Instituto Federal Pernambuco Campus Belo Jardim"
                  width={166}
                  height={56}
                  className="h-14 w-auto"
                />
              </div>

              <Link href="#" className="pb-1 text-[11.67px] leading-none font-semibold text-[#1351B4] underline">
                Precisa de Ajuda?
              </Link>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
