"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { FORM, InternalBreadcrumb, InternalHeader, ListCard, Modal, PageNotice, Segmented, Tag } from "@/components";
import { apiDeleteUser, apiGetProfile, apiListUsers, apiUpdateUserEmail, type UserSummary } from "@/lib/api";

type Filtro = "todos" | "aluno" | "pesquisador";

const SMALL = " h-10 px-5 text-[14px]";

function plural(n: number, um: string, varios: string) {
  return `${n} ${n === 1 ? um : varios}`;
}

export default function UsuariosPage() {
  const [users, setUsers] = useState<UserSummary[]>([]);
  const [meId, setMeId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [filtro, setFiltro] = useState<Filtro>("todos");

  const [editing, setEditing] = useState<UserSummary | null>(null);
  const [newEmail, setNewEmail] = useState("");
  const [editError, setEditError] = useState("");
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState<UserSummary | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const [removing, setRemoving] = useState(false);

  const load = useCallback(async () => {
    try {
      setUsers(await apiListUsers());
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao carregar usuários.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    Promise.all([apiListUsers(), apiGetProfile()])
      .then(([list, me]) => {
        if (cancelled) return;
        setUsers(list);
        setMeId(me.id);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erro ao carregar usuários.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, []);

  function abrirEdicao(u: UserSummary) {
    setNotice("");
    setEditing(u);
    setNewEmail(u.email);
    setEditError("");
  }

  async function salvarEmail() {
    if (!editing) return;
    setEditError("");
    const email = newEmail.trim().toLowerCase();
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setEditError("Informe um e-mail válido.");
      return;
    }
    setSaving(true);
    try {
      await apiUpdateUserEmail(editing.id, email);
      setEditing(null);
      setNotice(`E-mail alterado para ${email}. A pessoa passa a entrar com esse endereço.`);
      await load();
    } catch (err) {
      setEditError(err instanceof Error ? err.message : "Erro ao alterar o e-mail.");
    } finally {
      setSaving(false);
    }
  }

  function abrirExclusao(u: UserSummary) {
    setNotice("");
    setDeleting(u);
    setDeleteError("");
  }

  async function confirmarExclusao() {
    if (!deleting) return;
    setDeleteError("");
    setRemoving(true);
    try {
      await apiDeleteUser(deleting.id);
      setNotice(`Usuário ${deleting.email} excluído, com todas as máquinas e containers dele.`);
      setDeleting(null);
      await load();
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : "Erro ao excluir o usuário.");
      await load(); // parte dos recursos pode ter saído: atualiza as contagens
    } finally {
      setRemoving(false);
    }
  }

  const visiveis = filtro === "todos" ? users : users.filter((u) => u.role === filtro);
  const deletingNow = deleting ? users.find((u) => u.id === deleting.id) ?? deleting : null;

  return (
    <div className="min-h-screen bg-[#F8F8F8]">
      <InternalHeader />

      <main className="w-full px-10 pt-0 pb-8">
        <div className="h-10">
          <InternalBreadcrumb label="Usuários cadastrados" />
        </div>

        <div className="flex h-[90px] items-start justify-between pb-8">
          <h1 className="text-[41.8px] leading-none font-light text-black">Usuários cadastrados</h1>
          <Link href="/cadastrar-usuario" className={FORM.primaryBtn}>Cadastrar novo usuário</Link>
        </div>

        <div className="mx-auto flex max-w-[1100px] flex-col gap-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Segmented name="filtro de perfil" value={filtro} onChange={setFiltro}
              options={[{ v: "todos", label: "Todos" }, { v: "aluno", label: "Alunos" }, { v: "pesquisador", label: "Pesquisadores" }]} />
            {!loading && <p className="text-[14px] text-[#6B6B6B]">{plural(visiveis.length, "usuário", "usuários")}</p>}
          </div>

          {notice && <p role="status" className={FORM.success + " mt-0"}>{notice}</p>}
          {error && <PageNotice kind="error">{error}</PageNotice>}
          {loading && <PageNotice kind="info">Carregando...</PageNotice>}
          {!loading && !error && visiveis.length === 0 && <PageNotice kind="info">Nenhum usuário neste filtro.</PageNotice>}

          {visiveis.map((u) => {
            const souEu = u.id === meId;
            return (
              <ListCard
                key={u.id}
                name={u.name || u.email}
                description={u.name ? u.email : souEu ? "Você" : undefined}
                badge={<Tag>{u.role}{souEu ? " · você" : ""}</Tag>}
                fields={[
                  { label: "Máquinas virtuais", value: u.vm_count },
                  { label: "Containers", value: u.container_count },
                  { label: "Criado em", value: new Date(u.created_at).toLocaleDateString("pt-BR") },
                ]}
                actions={
                  <>
                    <button type="button" onClick={() => abrirEdicao(u)} className={FORM.outlineBtn + SMALL}>Alterar e-mail</button>
                    {!souEu && (
                      <button type="button" onClick={() => abrirExclusao(u)} className={FORM.dangerBtn + SMALL}>Excluir</button>
                    )}
                  </>
                }
              />
            );
          })}
        </div>

        <div className="flex items-center justify-center py-6">
          <Link href="/pagina-inicial-usuario" className={FORM.outlineBtn}>Voltar</Link>
        </div>
      </main>

      {editing && (
        <Modal
          title="ALTERAR E-MAIL"
          actions={
            <>
              <button type="button" onClick={() => setEditing(null)} disabled={saving} className={FORM.dangerBtn}>Cancelar</button>
              <button type="button" onClick={salvarEmail} disabled={saving} className={FORM.primaryBtn}>{saving ? "Salvando..." : "Salvar"}</button>
            </>
          }
        >
          <p>O e-mail é o login da pessoa. Avise-a da mudança: a senha continua a mesma.</p>
          <div className="mt-5">
            <label className={FORM.label} htmlFor="new-email">Novo e-mail</label>
            <input id="new-email" type="email" className={FORM.input} value={newEmail} onChange={(e) => setNewEmail(e.target.value)} autoComplete="off" autoFocus
              onKeyDown={(e) => { if (e.key === "Enter") salvarEmail(); }} />
          </div>
          {editError && <p role="alert" className={FORM.error + " mt-4"}>{editError}</p>}
        </Modal>
      )}

      {deletingNow && (
        <Modal
          title="EXCLUIR USUÁRIO"
          actions={
            <>
              <button type="button" onClick={() => setDeleting(null)} disabled={removing} className={FORM.outlineBtn}>Cancelar</button>
              <button type="button" onClick={confirmarExclusao} disabled={removing} className={FORM.dangerBtn}>{removing ? "Excluindo..." : "Excluir tudo"}</button>
            </>
          }
        >
          <p>
            A conta <strong>{deletingNow.email}</strong> será excluída junto com{" "}
            <strong>{plural(deletingNow.vm_count, "máquina virtual", "máquinas virtuais")}</strong> e{" "}
            <strong>{plural(deletingNow.container_count, "container", "containers")}</strong>.
          </p>
          <p className="mt-2">Os dados dessas máquinas e containers serão perdidos. Essa ação <strong>não pode ser desfeita</strong>.</p>
          {removing && <p className="mt-2 text-[#6B6B6B]">Apagando os recursos, isso pode levar alguns minutos...</p>}
          {deleteError && <p role="alert" className={FORM.error + " mt-4"}>{deleteError}</p>}
        </Modal>
      )}
    </div>
  );
}
