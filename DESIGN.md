---
name: page-to-calibre
description: Bancada simples para preparar artigos e enviá-los à biblioteca Calibre.
colors:
  primary: '#09665f'
  primary-hover: '#064c47'
  primary-active: '#043b38'
  primary-disabled: '#4e716e'
  page: '#f4f7f8'
  ink: '#182f3b'
  muted: '#48616a'
  surface: '#fff'
  border: '#879ba4'
  placeholder: '#586e78'
  readonly: '#eaf0f1'
  focus: '#147c94'
  link: '#075c69'
  divider: '#c8d3d8'
  success: '#dcefe9'
  error-ink: '#9c2929'
  error-surface: '#fff0ee'
  invalid-border: '#a12c2c'
  invalid-surface: '#fff4f3'
  selection: '#c1e4de'
  selection-ink: '#102f34'
  scrollbar: '#849a9f'
typography:
  headline:
    fontFamily: system-ui, sans-serif
    fontSize: clamp(1.9rem, 4vw, 2.75rem)
    lineHeight: 1.2
    letterSpacing: -.025em
  title:
    fontFamily: system-ui, sans-serif
    fontSize: 1.35rem
    lineHeight: 1.3
  body:
    fontFamily: system-ui, sans-serif
    fontSize: 1rem
    lineHeight: 1.6
  intro:
    fontFamily: system-ui, sans-serif
    fontSize: 1.0625rem
    lineHeight: 1.6
  label:
    fontFamily: system-ui, sans-serif
    fontSize: 1rem
    fontWeight: 600
    lineHeight: 1.6
  hint:
    fontFamily: system-ui, sans-serif
    fontSize: .875rem
    lineHeight: 1.6
rounded:
  control: .4rem
spacing:
  half: .5rem
  compact: .75rem
  base: 1rem
  gutter: 1.25rem
  field: 1.5rem
  section: 2rem
components:
  button-primary:
    backgroundColor: '{colors.primary}'
    textColor: '{colors.surface}'
    typography: '{typography.label}'
    rounded: '{rounded.control}'
    padding: .7rem 1.25rem
  button-primary-hover:
    backgroundColor: '{colors.primary-hover}'
  button-primary-active:
    backgroundColor: '{colors.primary-active}'
  button-primary-disabled:
    backgroundColor: '{colors.primary-disabled}'
  input:
    backgroundColor: '{colors.surface}'
    textColor: '{colors.ink}'
    typography: '{typography.body}'
    rounded: '{rounded.control}'
    padding: .7rem .85rem
    width: 100%
  input-readonly:
    backgroundColor: '{colors.readonly}'
  input-invalid:
    backgroundColor: '{colors.invalid-surface}'
  result:
    backgroundColor: '{colors.success}'
    textColor: '{colors.ink}'
    rounded: '{rounded.control}'
    padding: 1rem
  error:
    backgroundColor: '{colors.error-surface}'
    textColor: '{colors.error-ink}'
    rounded: '{rounded.control}'
    padding: 1rem
---

# Design System: page-to-calibre

## Overview

**Creative North Star: "Bancada simples de envio"**

Uma interface pessoal de leitura com fundo claro frio, texto azul profundo e ação verde petróleo. A aparência é discreta, com espaços suficientes para distinguir campos, instruções e respostas da operação.

O sistema construído usa tipografia de sistema, controles retangulares de cantos discretos e agrupamento por espaçamento. A identificação do produto fica acima da tarefa; não há imagens de produto. Este documento registra a implementação local existente, sem afirmar implantação ou publicação.

**Key Characteristics:**

- Paleta fria com uma ação principal em verde petróleo.
- Hierarquia por tamanho, peso, espaçamento e fundos de estado.
- Controles nativos, foco visível e adaptação para uma coluna.

## Colors

A paleta combina neutros frios com verde petróleo para a ação. O frontmatter contém os valores normativos extraídos de `web/src/style.css`. As rampas tonais do sidecar são amostras OKLCH sintetizadas para visualização; não representam cores adicionais usadas na interface.

### Primary

- **Verde petróleo** (`primary`): botão principal, acento nativo e cursor de texto. Os tokens `primary-hover`, `primary-active` e `primary-disabled` distinguem interação e espera.
- **Azul de interação** (`link`, `focus`): links sublinhados e anel de foco, respectivamente.

### Neutral

- **Papel frio** (`page`): fundo da página; **branco** (`surface`): campos e texto do botão.
- **Tinta azul** (`ink`): texto principal; **ardósia** (`muted`): introdução, ajuda, origem e rodapé; `placeholder`: indicação de preenchimento.
- **Contornos frios** (`border`, `divider`, `scrollbar`): campo, separação da prévia e barra de rolagem. `readonly` distingue campos temporariamente somente para leitura.
- **Seleção suave** (`selection`, `selection-ink`): seleção de texto legível.

Os tokens `success`, `error-ink`, `error-surface`, `invalid-border` e `invalid-surface` são semânticos, não novos acentos de marca. O sucesso tem fundo verde suave; erros usam texto vermelho e fundo claro, com mensagem textual explícita.

## Typography

A família única é `system-ui, sans-serif`, conforme a identidade operacional implementada; não há fonte remota nem fonte de exibição própria. O tamanho-base acompanha o padrão do navegador. O corpo usa entrelinha confortável, e o título principal comprime entrelinha e espaçamento de letras. A escala é funcional, sem razão modular declarada.

### Hierarchy

- **Headline:** título principal fluido; peso de título nativo do navegador. O token registra apenas as propriedades explicitamente definidas.
- **Title:** título da prévia; peso nativo do navegador.
- **Body:** texto corrente e campos.
- **Intro:** apresentação ligeiramente maior, limitada a 55ch.
- **Label:** rótulos e ação com peso seminegrito.
- **Hint:** ajuda e origem, em tamanho reduzido. Marcadores opcionais e rodapé usam `.85rem`; o nome do produto usa `.9rem` e peso 650; o status usa peso 550.

## Layout

Uma coluna central com largura máxima externa de 58rem, incluindo padding pelo `border-box`; no desktop, a área útil máxima é 54rem. O padding vertical é `clamp(1.25rem, 5vw, 4rem)` e o horizontal é 2rem. O trecho do artigo é limitado a 70ch, e o rodapé a 66ch.

A linha de envio combina URL flexível e botão com intervalo compacto. Os metadados usam duas colunas iguais, com intervalo de 1.25rem; título e etiquetas ocupam a largura inteira. A prévia começa após uma separação e respiro de seção. Os campos usam o intervalo `field` entre grupos.

Até 38rem, a linha de envio empilha, o botão ocupa toda a largura, os metadados passam a uma coluna e o padding horizontal cai para 1.25rem. Textos longos de origem, links e trecho podem quebrar em qualquer ponto para evitar transbordamento.

## Elevation & Depth

A implementação não usa sombras. Profundidade e agrupamento surgem do contraste entre página, campos brancos e fundos de estado; a prévia usa uma linha divisória. O foco é um contorno externo de 3px com deslocamento de 3px, sem sombra.

## Shapes

Botões, campos e mensagens de resultado ou erro compartilham cantos discretamente arredondados pelo token `control`. Os campos têm borda contínua de 1px. Não há cartões decorativos: a prévia é uma seção aberta e os fundos coloridos identificam respostas da operação.

## Components

### Buttons

Ação compacta e legível: verde petróleo, texto branco, peso de rótulo, sem borda e altura mínima de 3rem. Hover e pressão escurecem o fundo; desabilitado muda para o verde atenuado e cursor de espera. O foco segue o anel global. A largura integral aplica-se ao breakpoint móvel.

### Inputs / Fields

Campos brancos com texto principal, borda fria e altura mínima de 3rem. Rótulo acima, orientação textual e indicação de obrigatoriedade ou opcionalidade. Somente leitura usa fundo próprio; `:user-invalid` altera borda e fundo após interação. O erro de campo também aparece em texto. O foco segue o anel global.

### Respostas da operação

O status textual tem peso intermediário e fica acima da prévia. O erro tem fundo e tinta semânticos; o resultado usa fundo verde suave e link sublinhado. Ambos usam padding `base` e cantos `control`. As regiões são anunciadas por semântica de status ou alerta quando aplicável.

### Prévia

Seção aberta com divisória, título, origem, metadados e trecho. Com movimento permitido, aparece por recorte de 180ms `ease-out`; o botão transiciona a cor de fundo em 140ms `ease-out`. As duas animações só existem sob `prefers-reduced-motion: no-preference`. Não há navegação, chips ou biblioteca de cartões na superfície atual.

## Do's and Don'ts

### Do:

- Do manter rótulos visíveis e mensagens de estado próximas da ação.
- Do preservar o foco visível e a preferência de redução de movimento.
- Do usar os tokens de estado para diferenciar leitura, erro e resultado.

### Don't:

- Don't retirar sublinhados dos links de resultado.
- Don't depender apenas da cor para explicar erros ou progresso.
- Don't acrescentar sombras aos padrões planos documentados.
