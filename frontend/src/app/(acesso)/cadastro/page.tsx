import Image from "next/image";
import Link from "next/link";

export default function CadastroPage() {
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

            <h1 className="pb-5 text-[29.03px] leading-[1.1] font-medium text-black">Cadastro</h1>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Nome completo</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-user.svg" alt="Ícone nome" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="text"
                  placeholder="Nome"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Email (discente)</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-email.svg" alt="Ícone email" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="email"
                  placeholder="Email"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Matrícula</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-cap.svg" alt="Ícone matrícula" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="text"
                  placeholder="Matrícula"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">CPF</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-cpf.svg" alt="Ícone CPF" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="text"
                  placeholder="CPF"
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
              </div>
            </div>

            <div className="mt-2 grid grid-cols-2 gap-3">
              <Link href="/primeiro-acesso" className="flex h-[39px] items-center justify-center rounded-[20px] border border-[#1351B4] text-[16.8px] leading-none font-semibold text-[#1351B4]">
                Voltar
              </Link>
              <button type="button" className="h-[39px] rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white">
                Continuar
              </button>
            </div>

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
