# page-to-calibre

> **PoC encerrada e repositório arquivado em 05/11/2026.** O projeto foi um experimento de captura de páginas e importação EPUB no Calibre. O fluxo de artigos passou para o [Wallabag](https://wallabag.org/). Código e documentação ficam preservados para estudo e reprodução; não há manutenção ativa. [Resultados e limitações da PoC](docs/poc-results.md).

Transforma uma URL pública em um EPUB e envia ao Content server do Calibre. A prévia automática permite revisar título, autor, data e etiquetas. A fila fica no SQLite: fechar a aba não cancela uma importação aceita, e o mesmo navegador pode recuperar seu resultado.

Implementação validada localmente com Calibre 9.15.0, incluindo testes integrados de importação e deduplicação. A instalação precisa de um Content server configurado; siga o procedimento abaixo.

1. Cole o endereço do artigo.
2. Confira a prévia e ajuste os metadados.
3. Clique em **Enviar ao Calibre**. O sucesso só aparece depois de confirmar o item e o formato EPUB.

Reenviar uma origem já identificada abre o item existente, sem substituir seu conteúdo ou metadados. Uma escrita interrompida é reconciliada pelo identificador de origem. Se a confirmação não for possível, o resultado permanece **incerto** e exige verificação antes de reenviar.

A primeira versão não executa JavaScript dos sites, acessa contas de origem ou contorna paywalls. A extração pode falhar ou perder partes de páginas complexas. Imagens indisponíveis geram avisos, com o texto preservado. O aplicativo permanece disponível; não há desligamento automático em repouso.

- [Instalação, configuração e recuperação](docs/installation.md)
- [Contrato HTTP](docs/api.md)
- [Desenvolvimento e verificações](CONTRIBUTING.md)
- [Prova técnica e evidências locais](docs/technical-validation.md)

## Componentes e créditos

O extrator [go-readability do Readeck](https://codeberg.org/readeck/go-readability) deriva do [Readability da Mozilla](https://github.com/mozilla/readability). A geração usa [go-epub do Shiori](https://github.com/go-shiori/go-epub), derivado de bmaupin/go-epub. A sanitização usa [bluemonday](https://github.com/microcosm-cc/bluemonday). SQLite é fornecido pelo driver [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite). Versões exatas estão em `go.mod`, `go.sum` e `web/pnpm-lock.yaml`.

[Save as eBook](https://github.com/alexadam/save-as-ebook) e [Remarkablereader](https://github.com/buda-loco/Remarkablereader) inspiraram o fluxo; seu código não foi incorporado.

O Docker distribui o [Calibre](https://calibre-ebook.com/) oficial 9.15.0, sob GPLv3, com seu runtime e licenças. Nosso código é MIT; essa licença não substitui as licenças dos componentes distribuídos. [Avisos de distribuição](THIRD_PARTY_NOTICES.md).
