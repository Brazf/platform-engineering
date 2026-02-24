import Link from "next/link";
import { InputWithIcon, InternalBreadcrumb, InternalHeader, SelectWithIcon } from "@/components";

export default function CadastrarUsuarioPage() {
  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-3">
        <div className="h-10">
          <InternalBreadcrumb label="Novo usuário" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Novo usuário</h1>
        </div>

        <div className="flex h-[67px] items-end pb-4 pt-7">
          <h2 className="text-[16.8px] font-extrabold uppercase text-black">INFORMAÇÕES DO USUÁRIO</h2>
        </div>

        <div className="grid h-[135px] grid-cols-2 gap-2">
          <InputWithIcon label="Nome" placeholder="Nome" icon="user" />
          <SelectWithIcon label="Perfil de usuário" />
          <InputWithIcon label="Email" placeholder="Email" icon="email" />
          <InputWithIcon label="CPF" placeholder="CPF" icon="id" />
        </div>

        <div className="flex h-[71px] items-center justify-end gap-2 py-4">
          <Link
            href="/pagina-inicial-administrador"
            className="flex h-[39px] w-[120px] items-center justify-center rounded-[20px] border border-[#1351B4] text-[16.8px] leading-none font-semibold text-[#1351B4]"
          >
            Cancelar
          </Link>
          <button type="button" className="h-[39px] w-[160px] rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white">
            Salvar usuário
          </button>
        </div>
      </main>
    </div>
  );
}
