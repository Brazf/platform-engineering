# Design system do frontend (ProxLab)

Referência obrigatória para **qualquer tela interna nova**. É o padrão das telas de Máquina Virtual e
Container (`/maquinas-virtuais/*`, `/containers/*`), também usado em `/cadastrar-usuario` e `/meu-perfil`.
Não invente estilos: reutilize os componentes e constantes de `frontend/src/components/form-ui.tsx` e
`frontend/src/components/internal/internal-ui.tsx` (importe de `@/components`). Se faltar algo, acrescente
**lá**, como token/componente, e registre aqui.

Stack: Next.js (App Router, client components com `"use client"`), Tailwind v4 com valores arbitrários
(`text-[15px]`, `bg-[#1351B4]`), fonte Geist. Não há tema escuro. Visual inspirado no padrão gov.br.

## 1. Esqueleto de página

```tsx
<div className="min-h-screen bg-[#F8F8F8]">
  <InternalHeader />
  <main className="w-full px-10 pt-0 pb-8">
    <div className="h-10"><InternalBreadcrumb label="Título" /></div>
    <div className="flex h-[90px] items-start pb-8">            {/* + justify-between se tiver botão */}
      <h1 className="text-[41.8px] leading-none font-light text-black">Título</h1>
      {/* opcional: <Link href="..." className={FORM.primaryBtn}>Criar ...</Link> */}
    </div>
    {/* conteúdo, centralizado em max-w-[1100px] */}
    <div className="flex items-center justify-center py-6">
      <Link href="/pagina-inicial-usuario" className={FORM.outlineBtn}>Voltar</Link>
    </div>
  </main>
</div>
```

- `InternalHeader`: logo IFPE (link para a página inicial), à direita **notificações → Meu perfil → divisor → Sair**,
  e a linha "Plataforma de Provisionamento". Não duplicar o header em páginas.
- Conteúdo de lista/detalhe/form: `mx-auto max-w-[1100px]`.

## 2. Cores

| Uso | Valor |
|---|---|
| Fundo da página | `#F8F8F8` |
| Superfície (cartões, header) | `#FFFFFF` |
| Primária (botões, links, títulos de seção, foco) | `#1351B4`, hover `#0F4499`, fundo suave `#EEF3FB` |
| Texto principal / secundário / hint | `#000` · `#555` (labels) · `#6B6B6B` |
| Bordas de input / divisórias | `#8F8F8F` · `#E3E3E3` · divisor do header `#CCCCCC` |
| Perigo | texto/borda `#B21D38`, hover `#FBEDEF`; caixa de erro `#E52207` / `#FDF2F1` |
| Sucesso | caixa `#168821` / `#E3F5E1` / texto `#0B5B13` |
| Status (bolinha) | `STATUS_DOT`: verde `#1FA433`, amarelo `#F5A623`, cinza `#8A8A8A`, vermelho `#FA1D1D` |

Sombra de cartão: `shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]`. Sombra do header: `shadow-[0_1px_6px_0_rgba(0,0,0,0.16)]`.
Cantos: cartões retos (sem radius); inputs `rounded-[4px]`; botões pílula `rounded-[21px]`; ícones do header `rounded-full`.

## 3. Tipografia

| Elemento | Classe |
|---|---|
| H1 da página | `text-[41.8px] leading-none font-light` |
| Título do cartão (caixa alta) | `FormCard title` → `text-[12px] font-bold tracking-[1.5px]` |
| Título de seção | `Group title` → `text-[14px] font-semibold text-[#1351B4]` |
| Label de campo | `FORM.label` (12px, bold, uppercase, tracking 1.2px, `#555`) |
| Valor/Input | 15px preto |
| Hint | `FORM.hint` (13px `#6B6B6B`) |
| Nome em cartão de lista / detalhe | 20px / 24px `font-semibold` |

## 4. Componentes (`@/components`)

**Formulários (`form-ui.tsx`)**
- `FormCard({ title, subtitle })` — cartão branco do formulário; título em CAIXA ALTA.
- `Group({ title })` — seção com título azul; grade de 2 colunas em `md`. Separação por borda `#E3E3E3`.
- `Segmented({ value, options, onChange, name })` — escolha exclusiva em botões (preferir a `<select>` para 2–4 opções).
- `FormActions({ cancelHref, submitLabel, loadingLabel, loading, onSubmit })` — rodapé centralizado: Cancelar (perigo) + ação principal.
- Constantes `FORM.*`: `label`, `input` (use também em `<select>`), `hint`, `value` (campo só leitura),
  `linkBtn`, `error`, `success`, `primaryBtn`, `outlineBtn`, `dangerBtn`.
- Campo padrão:
  ```tsx
  <div>
    <label className={FORM.label} htmlFor="x">Rótulo</label>
    <input id="x" className={FORM.input} ... />
    <p className={FORM.hint}>Explicação curta.</p>
  </div>
  ```
- Campo só leitura: `<p className={FORM.label}>E-mail</p><p className={FORM.value}>{valor}</p>`.
- Mensagens: `<p role="alert" className={FORM.error}>` / `<p role="status" className={FORM.success}>`.
- Mais de um formulário na mesma página (ex.: Meu perfil): um `FormCard` por formulário, empilhados com
  `grid auto-rows-fr gap-6` para **todos os cartões terem a mesma altura** (o menor estica até o maior);
  botão principal dentro do cartão em `mt-auto flex justify-end`, colado no rodapé.

**Listas e detalhes (`form-ui.tsx`)**
- `ResourceCard({ href, name, description, status, fields, createdAt })` — item de lista clicável.
- `StatusBadge({ label, phase })`, `Field({ label })`, `DetailHeader({ name, description, status })`.
- `PageNotice({ kind: "info" | "error" })` — carregando / vazio / erro fora dos cartões.
- `CONSOLE.*` — painéis de terminal/logs (cabeçalho cinza `#E5E5E5`).

**Estrutura (`internal-ui.tsx`)**
- `InternalHeader`, `InternalBreadcrumb({ label })`, `ActionCard({ icon, label, href, height })` (cards da página inicial).

**Modal**: overlay `fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4`, caixa
`w-full max-w-[520px] bg-white px-8 pt-6 pb-7` + sombra de cartão; título no estilo de `FormCard title`.

## 5. Botões

| Tipo | Uso |
|---|---|
| `FORM.primaryBtn` | ação principal (Criar, Salvar) |
| `FORM.outlineBtn` | secundária / Voltar / Copiar (compacto: `+ " h-10 px-5 text-[14px]"`) |
| `FORM.dangerBtn` | Cancelar, Excluir |
| `FORM.linkBtn` | ação textual dentro de cartão |

Estado de carregamento troca o texto (`"Salvando..."`) e usa `disabled` (opacidade 60%).

## 6. Ícones e assets

`frontend/public/assets/internal/*.svg|png`, via `next/image`. Ícones do header: círculo `h-10 w-10 rounded-full bg-[#F8F8F8]`,
SVG 12×14 em azul `#1351B4` (`icon-bell.svg`, `icon-profile.svg`). Ícones de campo em cinza `#888888` (`icon-user.svg`, `icon-email.svg`, `icon-id.svg`).

## 7. Comportamento

- Dados via `@/lib/api` (`request` já põe o Bearer; 401 limpa a sessão e volta para `/`). Por isso erros de
  regra de negócio **não** devem usar 401 no backend (ex.: senha atual errada → 403).
- Carregamento em `useEffect` com flag `cancelled`; papel do usuário via `getRole()` (`useSyncExternalStore` na página inicial).
- Validar no front antes de chamar a API (mensagens em português, curtas) e mostrar o `error` do backend quando vier.
- Textos em português do Brasil; datas `toLocaleDateString("pt-BR")`.
- Acessibilidade: `htmlFor`/`id` em todo input, `aria-label` em botões só com ícone, foco visível `focus-visible:ring-2 focus-visible:ring-[#1351B4]/40`.
