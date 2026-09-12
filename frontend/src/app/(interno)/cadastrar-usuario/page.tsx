"use client";

import { useState } from "react";
import Link from "next/link";
import { FORM, FormActions, FormCard, Group, InternalBreadcrumb, InternalHeader, Segmented } from "@/components";
import { apiRegister } from "@/lib/api";

export default function CadastrarUsuarioPage() {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"aluno" | "pesquisador">("aluno");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [created, setCreated] = useState<{ email: string; role: string } | null>(null);
  const [loading, setLoading] = useState(false);

  function reset() {
    setEmail("");
    setRole("aluno");
    setPassword("");
    setConfirm("");
    setCreated(null);
    setError("");
  }

  async function handleSalvar() {
    setError("");
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      setError("Informe um e-mail válido.");
      return;
    }
    if (password.length < 8) {
      setError("A senha provisória precisa ter pelo menos 8 caracteres.");
      return;
    }
    if (password !== confirm) {
      setError("As senhas não conferem.");
      return;
    }
    setLoading(true);
    try {
      const u = await apiRegister(email.trim().toLowerCase(), password, role);
      setCreated({ email: u.email, role: u.role });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao cadastrar usuário.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Novo usuário" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Novo usuário</h1>
        </div>

        {created ? (
          <FormCard title="USUÁRIO CADASTRADO" subtitle="A conta já pode entrar na plataforma com a senha provisória informada.">
            <Group title="Resumo">
              <div>
                <p className={FORM.label}>E-mail</p>
                <p className={FORM.value}>{created.email}</p>
              </div>
              <div>
                <p className={FORM.label}>Perfil</p>
                <p className={FORM.value}>{created.role}</p>
              </div>
            </Group>
            <div className="flex flex-wrap items-center gap-3">
              <button type="button" onClick={reset} className={FORM.primaryBtn}>Cadastrar outro</button>
              <Link href="/pagina-inicial-usuario" className={FORM.outlineBtn}>Voltar à página inicial</Link>
            </div>
          </FormCard>
        ) : (
          <>
            <FormCard
              title="INFORMAÇÕES DO USUÁRIO"
              subtitle="Cadastre alunos e pesquisadores do seu grupo. Compartilhe a senha provisória com a pessoa; ela poderá trocá-la depois."
            >
              <Group title="Conta">
                <div>
                  <label className={FORM.label} htmlFor="email">E-mail</label>
                  <input id="email" type="email" className={FORM.input} value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nome@ifpe.edu.br" autoComplete="off" />
                  <p className={FORM.hint}>É o login da pessoa na plataforma.</p>
                </div>
                <div>
                  <p className={FORM.label}>Perfil</p>
                  <Segmented name="perfil" value={role} onChange={setRole}
                    options={[{ v: "aluno", label: "Aluno" }, { v: "pesquisador", label: "Pesquisador" }]} />
                  <p className={FORM.hint}>{role === "aluno" ? "Limites menores de CPU, memória e armazenamento." : "Limites maiores e pode cadastrar usuários."}</p>
                </div>
              </Group>

              <Group title="Senha provisória">
                <div>
                  <label className={FORM.label} htmlFor="pass">Senha</label>
                  <input id="pass" type="password" className={FORM.input} value={password} onChange={(e) => setPassword(e.target.value)} placeholder="mínimo 8 caracteres" autoComplete="new-password" />
                </div>
                <div>
                  <label className={FORM.label} htmlFor="confirm">Confirmar senha</label>
                  <input id="confirm" type="password" className={FORM.input} value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />
                </div>
              </Group>

              {error && <p role="alert" className={FORM.error}>{error}</p>}
            </FormCard>

            <FormActions cancelHref="/pagina-inicial-usuario" submitLabel="Salvar usuário" loadingLabel="Salvando..." loading={loading} onSubmit={handleSalvar} />
          </>
        )}
      </main>
    </div>
  );
}
