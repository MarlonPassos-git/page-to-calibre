# page-to-calibre

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Backend Go; HTML/CSS/TypeScript sem framework. Decisão confirmada no plano executado.

## Users

Leitor que guarda artigos públicos na biblioteca Calibre existente e lê pelo KOReader.

## Product Purpose

Preparar automaticamente uma URL, permitir revisão de metadados e importar o EPUB sem download/upload manual. Fechar a aba não cancela um envio aceito.

## Operating Context

Instância pessoal em rede Tailscale privada, mesma origem para página e API. Calibre separado e acesso de escrita próprio da integração.

## Capabilities and Constraints

Página única em português; URL e botão “Enviar ao Calibre” lado a lado em desktop; prévia automática, título/autor/data/etiquetas editáveis; sem histórico ou catálogo próprio. Sem JavaScript nos sites de origem, login, paywall ou extensão. Respostas antigas nunca sobrescrevem outra URL. Sucesso exige confirmação do EPUB no Calibre. O estado de implantação será documentado separadamente.

## Accessibility & Inclusion

Teclado, foco visível, leitor de tela, redução de movimento e uso em celular. Validação após interação.
