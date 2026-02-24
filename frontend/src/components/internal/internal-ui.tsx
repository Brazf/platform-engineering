import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";

type IconName = "bars" | "home" | "chevronRight" | "caretDown";

function Icon({ name, className = "h-4 w-4" }: { name: IconName; className?: string }) {
  if (name === "bars") {
    return (
      <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden>
        <path d="M4 6h16v2H4V6Zm0 5h16v2H4v-2Zm0 5h16v2H4v-2Z" />
      </svg>
    );
  }

  if (name === "home") {
    return (
      <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden>
        <path d="M12 3 3 10h2v9h6v-6h2v6h6v-9h2L12 3Z" />
      </svg>
    );
  }

  if (name === "caretDown") {
    return (
      <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden>
        <path d="m12 15-6-7h12l-6 7Z" />
      </svg>
    );
  }

  return (
    <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden>
      <path d="m9 6 6 6-6 6" />
    </svg>
  );
}

export function InternalHeader() {
  return (
    <header className="h-[116px] bg-white pl-[40px] pr-[40px] py-[10px] shadow-[0_1px_6px_0_rgba(0,0,0,0.16)]">
      <div className="flex h-10 items-center justify-between">
        <Link href="/pagina-inicial-usuario" aria-label="Ir para a página inicial">
          <Image
            src="/assets/access/pngteste.png"
            alt="Instituto Federal Pernambuco Campus Belo Jardim"
            width={118}
            height={40}
            className="h-10 w-auto"
          />
        </Link>

        <div className="flex items-center gap-2">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-[#F8F8F8]">
            <Image src="/assets/internal/icon-bell.svg" alt="Notificações" width={12} height={14} className="h-[14px] w-[12px]" />
          </span>
          <span className="h-8 w-px bg-[#CCCCCC]" />
          <Link href="/" className="flex h-[39px] w-[79px] items-center justify-center rounded-[20px] text-[16.8px] leading-none font-semibold text-[#1351B4]">
            Sair
          </Link>
        </div>
      </div>

      <div className="mt-4 flex h-10 items-center gap-4">
        <button type="button" className="flex h-10 w-10 items-center justify-center rounded-[16px] text-[#1351B4]" aria-label="Menu">
          <Icon name="bars" className="h-4 w-4" />
        </button>
        <h2 className="text-[24.19px] leading-none font-normal text-black">Plataforma de Provisionamento</h2>
      </div>
    </header>
  );
}

export function InternalCompactHeader() {
  return (
    <header className="h-[60px] bg-white pl-[40px] pr-[40px] py-[10px] shadow-[0_1px_6px_0_rgba(0,0,0,0.16)]">
      <div className="flex h-10 items-center">
        <Link href="/pagina-inicial-usuario" aria-label="Ir para a página inicial">
          <Image
            src="/assets/access/pngteste.png"
            alt="Instituto Federal Pernambuco Campus Belo Jardim"
            width={118}
            height={40}
            className="h-10 w-auto"
          />
        </Link>
      </div>
    </header>
  );
}

export function InternalBreadcrumb({ label }: { label: string }) {
  return (
    <div className="flex h-10 items-center gap-2 text-[11.67px] leading-none text-[#333333]">
      <Link href="/pagina-inicial-usuario" aria-label="Ir para a página inicial" className="text-[#1351B4]">
        <Icon name="home" className="h-[14px] w-[14px]" />
      </Link>
      <span className="text-[#CCCCCC]">
        <Icon name="chevronRight" className="h-[12px] w-[12px]" />
      </span>
      <span>{label}</span>
    </div>
  );
}

export function ActionCard({ icon, label, href, height }: { icon: "desktop" | "container" | "userPlus"; label: string; href?: string; height: "121" | "134" }) {
  const iconNode =
    icon === "desktop" ? (
      <Image src="/assets/internal/icon-computador.png" alt="Máquina Virtual" width={27} height={27} className="h-[27px] w-[27px]" />
    ) : icon === "container" ? (
      <Image src="/assets/internal/icon-container.png" alt="Container" width={30} height={30} className="h-[30px] w-[30px]" />
    ) : (
      <Image src="/assets/internal/icon-user-plus.svg" alt="Cadastrar usuário" width={30} height={24} className="h-6 w-[30px]" />
    );

  const content = (
    <div className={`flex w-full flex-col items-center justify-center gap-2 bg-white px-4 py-8 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)] ${height === "121" ? "h-[121px]" : "h-[134px]"}`}>
      <span>{iconNode}</span>
      <span className="w-[120px] text-center text-[14px] leading-normal text-[#1351B4]">{label}</span>
    </div>
  );

  if (href) {
    return (
      <Link href={href} className="block w-full">
        {content}
      </Link>
    );
  }

  return content;
}

export function InputWithIcon({ label, placeholder, icon, rightSlot }: { label: string; placeholder: string; icon: "user" | "email" | "id"; rightSlot?: ReactNode }) {
  const iconPath = icon === "user" ? "/assets/internal/icon-user.svg" : icon === "email" ? "/assets/internal/icon-email.svg" : "/assets/internal/icon-id.svg";

  return (
    <div className="flex h-[63px] flex-col">
      <label className="pb-1 text-[14px] leading-none font-semibold text-black">{label}</label>
      <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
        <span className="text-[#888888]">
          <Image src={iconPath} alt="Ícone" width={14} height={16} className="h-4 w-[14px]" />
        </span>
        <input type="text" placeholder={placeholder} className="h-full w-full bg-transparent py-1 text-[14px] italic outline-none placeholder:text-black" />
        {rightSlot}
      </div>
    </div>
  );
}

export function SelectWithIcon({ label }: { label: string }) {
  return (
    <div className="flex h-[64px] flex-col">
      <label className="pb-1 text-[14px] leading-none font-semibold text-black">{label}</label>
      <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
        <span className="text-[#888888]">
          <Image src="/assets/internal/icon-id.svg" alt="Ícone perfil" width={14} height={16} className="h-4 w-[14px]" />
        </span>
        <select className="h-full w-full bg-transparent py-1 text-[14px] italic text-black outline-none">
          <option>Selecione um perfil</option>
          <option>Administrador</option>
          <option>Usuário</option>
        </select>
        <span className="flex h-8 w-8 items-center justify-center text-[#1351B4]">
          <Icon name="caretDown" className="h-4 w-4" />
        </span>
      </div>
    </div>
  );
}
