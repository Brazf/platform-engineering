"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { FORM, FormCard, Group, InternalBreadcrumb, InternalHeader, PageNotice } from "@/components";
import { apiChangePassword, apiGetProfile, apiUpdateProfile, type Profile } from "@/lib/api";

export default function MeuPerfilPage() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loadError, setLoadError] = useState("");

  const [name, setName] = useState("");
  const [nameMsg, setNameMsg] = useState<{ kind: "error" | "success"; text: string } | null>(null);
  const [savingName, setSavingName] = useState(false);

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [passMsg, setPassMsg] = useState<{ kind: "error" | "success"; text: string } | null>(null);
  const [savingPass, setSavingPass] = useState(false);

  useEffect(() => {
    let cancelled = false;
    apiGetProfile()
      .then((p) => {
        if (cancelled) return;
        setProfile(p);
        setName(p.name);
      })
      .catch((err) => {
        if (!cancelled) setLoadError(err instanceof Error ? err.message : "Erro ao carregar o perfil.");
      });
    return () => { cancelled = true; };
  }, []);

  async function handleSalvarNome() {
    setNameMsg(null);
    if (name.trim().length > 120) {
      setNameMsg({ kind: "error", text: "O nome pode ter no máximo 120 caracteres." });
      return;
    }
    setSavingName(true);
    try {
      const p = await apiUpdateProfile(name.trim());
      setProfile(p);
      setName(p.name);
      setNameMsg({ kind: "success", text: "Nome atualizado." });
    } catch (err) {
      setNameMsg({ kind: "error", text: err instanceof Error ? err.message : "Erro ao salvar o nome." });
    } finally {
      setSavingName(false);
    }
  }

  async function handleTrocarSenha() {
    setPassMsg(null);
    if (!current) {
      setPassMsg({ kind: "error", text: "Informe a senha atual." });
      return;
    }
    if (next.length < 8) {
      setPassMsg({ kind: "error", text: "A nova senha precisa ter pelo menos 8 caracteres." });
      return;
    }
    if (next !== confirm) {
      setPassMsg({ kind: "error", text: "As senhas não conferem." });
      return;
    }
    setSavingPass(true);
    try {
      await apiChangePassword(current, next);
      setCurrent("");
      setNext("");
      setConfirm("");
      setPassMsg({ kind: "success", text: "Senha alterada. Use a nova senha no próximo login." });
    } catch (err) {
      setPassMsg({ kind: "error", text: err instanceof Error ? err.message : "Erro ao trocar a senha." });
    } finally {
      setSavingPass(false);
    }
  }

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Meu perfil" />
        </div>

        <div className="flex h-[90px] items-start pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Meu perfil</h1>
        </div>

        {loadError && <div className="mx-auto max-w-[1100px]"><PageNotice kind="error">{loadError}</PageNotice></div>}
        {!profile && !loadError && <div className="mx-auto max-w-[1100px]"><PageNotice kind="info">Carregando...</PageNotice></div>}

        {profile && (
          <div className="grid auto-rows-fr gap-6">{/* cartões com a mesma altura */}
            <FormCard title="DADOS DA CONTA" subtitle="O e-mail é o seu login e não pode ser alterado.">
              <Group title="Identificação">
                <div>
                  <label className={FORM.label} htmlFor="name">Nome</label>
                  <input id="name" className={FORM.input} value={name} onChange={(e) => setName(e.target.value)} placeholder="Seu nome" autoComplete="name" maxLength={120} />
                  <p className={FORM.hint}>Como você aparece para os pesquisadores.</p>
                </div>
                <div>
                  <p className={FORM.label}>E-mail</p>
                  <p className={FORM.value}>{profile.email}</p>
                </div>
                <div>
                  <p className={FORM.label}>Perfil</p>
                  <p className={FORM.value}>{profile.role}</p>
                </div>
                <div>
                  <p className={FORM.label}>Conta criada em</p>
                  <p className={FORM.value}>{new Date(profile.created_at).toLocaleDateString("pt-BR")}</p>
                </div>
              </Group>

              {nameMsg && <p role={nameMsg.kind === "error" ? "alert" : "status"} className={nameMsg.kind === "error" ? FORM.error : FORM.success}>{nameMsg.text}</p>}

              <div className="mt-auto flex justify-end">
                <button type="button" onClick={handleSalvarNome} disabled={savingName || name.trim() === profile.name} className={FORM.primaryBtn}>
                  {savingName ? "Salvando..." : "Salvar nome"}
                </button>
              </div>
            </FormCard>

            <FormCard title="ALTERAR SENHA" subtitle="Por segurança, confirme a senha atual antes de definir a nova.">
              <Group title="Senha">
                <div>
                  <label className={FORM.label} htmlFor="current">Senha atual</label>
                  <input id="current" type="password" className={FORM.input} value={current} onChange={(e) => setCurrent(e.target.value)} autoComplete="current-password" />
                </div>
                <div className="hidden md:block" />
                <div>
                  <label className={FORM.label} htmlFor="next">Nova senha</label>
                  <input id="next" type="password" className={FORM.input} value={next} onChange={(e) => setNext(e.target.value)} placeholder="mínimo 8 caracteres" autoComplete="new-password" />
                </div>
                <div>
                  <label className={FORM.label} htmlFor="confirm">Confirmar nova senha</label>
                  <input id="confirm" type="password" className={FORM.input} value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />
                </div>
              </Group>

              {passMsg && <p role={passMsg.kind === "error" ? "alert" : "status"} className={passMsg.kind === "error" ? FORM.error : FORM.success}>{passMsg.text}</p>}

              <div className="mt-auto flex justify-end">
                <button type="button" onClick={handleTrocarSenha} disabled={savingPass} className={FORM.primaryBtn}>
                  {savingPass ? "Alterando..." : "Alterar senha"}
                </button>
              </div>
            </FormCard>
          </div>
        )}

        <div className="flex items-center justify-center py-6">
          <Link href="/pagina-inicial-usuario" className={FORM.outlineBtn}>Voltar</Link>
        </div>
      </main>
    </div>
  );
}
