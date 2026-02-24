import Image from "next/image";
import Link from "next/link";

export default function NovaSenhaPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-white px-4 py-10 md:px-10 md:py-20">
      <div className="grid w-full max-w-[1280px] items-center gap-10 md:grid-cols-[736px_464px] md:gap-10">
        <section className="hidden md:block" aria-hidden>
          <Image src="/file.png" alt="Sprint Report - Gestão de TCC" width={736} height={414} className="h-[414px] w-[736px] object-contain" priority />
        </section>

        <section className="w-full md:w-[464px]">
          <div className="flex flex-col gap-5">
            <div className="h-[39px]">
              <Link href="/" className="text-[16.8px] leading-none font-semibold text-[#1351B4]">
                Voltar para Login
              </Link>
            </div>

            <h1 className="pb-5 text-[42px] leading-[1.1] font-medium text-black">Nova senha</h1>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Senha</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-lock.svg" alt="Ícone senha" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="password"
                  placeholder="Senha"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
                <button type="button" aria-label="Mostrar senha" className="flex h-8 w-8 items-center justify-center text-[#1351B4]">
                  <svg viewBox="0 0 24 24" className="h-4 w-4 fill-current">
                    <path d="M12 5c-5.2 0-9.3 4.2-10.8 6.1a1.4 1.4 0 0 0 0 1.8C2.7 14.8 6.8 19 12 19s9.3-4.2 10.8-6.1a1.4 1.4 0 0 0 0-1.8C21.3 9.2 17.2 5 12 5Zm0 11a5 5 0 1 1 0-10 5 5 0 0 1 0 10Zm0-2.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z" />
                  </svg>
                </button>
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Confirmar senha</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-lock.svg" alt="Ícone confirmar senha" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="password"
                  placeholder="Senha"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
                <button type="button" aria-label="Mostrar senha" className="flex h-8 w-8 items-center justify-center text-[#1351B4]">
                  <svg viewBox="0 0 24 24" className="h-4 w-4 fill-current">
                    <path d="M12 5c-5.2 0-9.3 4.2-10.8 6.1a1.4 1.4 0 0 0 0 1.8C2.7 14.8 6.8 19 12 19s9.3-4.2 10.8-6.1a1.4 1.4 0 0 0 0-1.8C21.3 9.2 17.2 5 12 5Zm0 11a5 5 0 1 1 0-10 5 5 0 0 1 0 10Zm0-2.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z" />
                  </svg>
                </button>
              </div>
            </div>

            <Link href="/" className="mt-2 flex h-[39px] items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white">
              Finalizar
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
