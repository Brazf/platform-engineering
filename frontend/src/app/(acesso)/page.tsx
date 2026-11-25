"use client";

import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { PasswordToggle } from "@/components";
import { apiLogin, saveAuth } from "@/lib/api";

export default function Home() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleLogin() {
    setError("");
    if (!email || !password) {
      setError("Preencha email e senha.");
      return;
    }
    setLoading(true);
    try {
      const res = await apiLogin(email, password);
      saveAuth(res.token, res.user.role);
      router.push("/pagina-inicial-usuario");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao entrar.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-white px-4 py-10 md:px-10 md:py-20">
      <div className="grid w-full max-w-[1280px] items-center gap-10 md:grid-cols-[736px_464px] md:gap-10">
        <section className="hidden md:block" aria-hidden>
          <Image src="/file.png" alt="Sprint Report - Gestão de TCC" width={736} height={414} className="h-[414px] w-[736px] object-contain" priority />
        </section>

        <section className="w-full md:w-[464px]">
          <div className="flex flex-col gap-5">
            <h1 className="pb-5 text-[29.03px] leading-[1.1] font-medium text-black/20">Acesso à Plataforma de Provisionamento</h1>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Email</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-user.svg" alt="Ícone usuário" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type="email"
                  placeholder="Email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  onKeyDown={e => e.key === "Enter" && handleLogin()}
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-[14px] leading-none font-semibold text-black">Senha</label>
              <div className="flex h-10 items-center gap-2 rounded-[4px] border border-[#888888] bg-white pl-4 pr-1">
                <Image src="/assets/access/icon-lock.svg" alt="Ícone senha" width={14} height={16} className="h-4 w-[14px]" />
                <input
                  type={showPassword ? "text" : "password"}
                  placeholder="Senha"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  onKeyDown={e => e.key === "Enter" && handleLogin()}
                  className="h-full w-full bg-transparent text-[14px] italic outline-none placeholder:text-black"
                />
                <PasswordToggle visible={showPassword} onToggle={() => setShowPassword(v => !v)} />
              </div>
            </div>

            {error && (
              <p className="text-[12px] leading-none text-[#B21D38]">{error}</p>
            )}

            <div className="flex items-center justify-between">
              <label className="flex items-center gap-2 text-[14px] text-black">
                <span className="flex h-6 w-6 items-center justify-center rounded-full border border-[#2670E8] bg-[#2670E8]/20">
                  <span className="h-3.5 w-3.5 rounded-full bg-[#2670E8]" />
                </span>
                Manter-me conetado
              </label>
              <Link href="/esqueceu-senha" className="text-[11.67px] leading-none font-semibold text-[#1351B4] underline">
                Esqueceu a senha?
              </Link>
            </div>

            <button
              type="button"
              onClick={handleLogin}
              disabled={loading}
              className="flex h-[39px] w-full items-center justify-center rounded-[20px] bg-[#1351B4] text-[16.8px] leading-none font-semibold text-white disabled:opacity-60"
            >
              {loading ? "Entrando..." : "Entrar"}
            </button>

            <div className="flex justify-center pt-1">
              <Link href="/primeiro-acesso" className="text-[16.8px] leading-none font-semibold text-[#1351B4]">
                Primeiro Acesso?
              </Link>
            </div>

            <div className="mt-8 flex h-[56px]  justify-between items-center">
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
