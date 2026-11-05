# Versionamento do documento do TCC

Regras para qualquer alteração em `documentos-tcc/` (texto `.tex`, `bibliography.bib`, imagens, PDF).
Valem para a IA e para quem editar à mão. Decididas pelo autor em 2026-11-05.

## Duas camadas independentes

| Camada | O que é | Quando |
|---|---|---|
| **Commit** | Registro de cada alteração | Sempre que o autor pedir; um commit por alteração coerente |
| **Versão (tag)** | Marco do documento, `tcc-vX.Y` | Só em mudanças grandes, e **só com confirmação do autor** |

Os commits andam em paralelo com as versões: o histórico de commits mostra o tamanho de cada alteração, e é ele que ajuda a decidir se já vale uma nova versão.

## Quando versionar (tag)

- **Versão maior (`vX.0`)**: reestruturação do documento (capítulos reorganizados, fundamentação reescrita, metodologia/implementação movidas, etc.).
- **Versão menor (`vX.Y`)**: acúmulo de alterações relevantes, por exemplo:
  - **mais de 5 imagens novas** de uma vez (mudança que justifica uma versão por tornar visível o que antes não existia);
  - uma seção/capítulo novo ou reescrito;
  - ajustes pedidos pelo professor aplicados em bloco.
- **Não versionar**: adicionar 1 a 5 imagens, corrigir texto/legenda, ajustar bibliografia, recompilar o PDF. Isso é só commit.

Em caso de dúvida, não criar a tag: perguntar.

## Fluxo da IA (obrigatório)

1. Ao alterar o documento, fazer a alteração e compilar para conferir.
2. **Commit** apenas se o autor pedir (mensagem `docs(tcc): ...`, só arquivos de `documentos-tcc/`).
3. Ao terminar um conjunto de mudanças, **avaliar o tamanho** (quantos arquivos, quantas imagens novas, se houve reestruturação) e **perguntar ao autor se deve criar uma nova versão**, sugerindo o número (`tcc-vX.Y`) e justificando em uma linha. Nunca criar a tag sem a resposta.
4. Criar a tag anotada no commit: `git tag -a tcc-vX.Y -m "<o que mudou>"`. Não dar push sem pedido.
5. Lembrar o autor de que o PDF da versão pode ser extraído com:
   `git show tcc-vX.Y:documentos-tcc/Modelo_TCC___Engenharia_de_Software___Fábio_Braz/tcc_ifpe_eng_sw.pdf > TCC_vX.Y.pdf`

## Compilação (biber)

O biber do MacTeX falha neste Mac (`extracting arm64 binary with lipo failed`). Usar o binário extraído:
`PATH=$HOME/.local/bin:$PATH latexmk -pdf -interaction=nonstopmode tcc_ifpe_eng_sw.tex`
(`~/.local/bin/biber` vem de `lipo /Library/TeX/texbin/biber -thin arm64 -output ~/.local/bin/biber`.)

## Histórico de versões

| Versão | Descrição |
|---|---|
| `tcc-v1.0` | Documento original (antes das recomendações do professor) |
| `tcc-v2.0` | Fundamentação reestruturada, metodologia/implementação separadas, experimentos como validações de fluxo, 11 diagramas e 10 prints adicionados |
