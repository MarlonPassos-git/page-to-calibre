# Desenvolvimento

Use Go 1.27.3, Node 24 LTS e pnpm 12.14.0. Versões locais em `mise.toml`; o lockfile do frontend é obrigatório. A produção não depende de Node, MSW ou Playwright.

Responsabilidades: `internal/capture` controla rede; `article` extrai/sanitiza; `book` gera EPUB; `calibre` encapsula o CLI; `store` mantém SQLite; `service` executa o worker; `api` publica o contrato JSON. `web/` usa a mesma API. O executável `cmd/probe` serve à avaliação local, sem Calibre.

```sh
mise exec go@1.27.3 -- go test ./...
mise exec go@1.27.3 -- go test -race ./...
mise exec go@1.27.3 -- go vet ./...
pnpm --dir web install --frozen-lockfile
pnpm --dir web typecheck
pnpm --dir web lint
pnpm --dir web test
pnpm --dir web build
```

Para o contrato real, execute `./integration/check.sh`: cria um Calibre 9.15 isolado a partir da imagem runtime, usa contas sintéticas, executa os contratos e o Playwright e remove somente seu container temporário. Para EPUBCheck, `integration/Dockerfile` fornece uma imagem de ferramentas separada. `PTC_CONTRACT_CONTAINER` habilita os testes que executam `calibredb` dentro desse container, com contas sintéticas. O servidor `integration/e2e-server` substitui apenas a captura de páginas por HTML sintético; banco, API, EPUB, fila e Calibre são reais.

```sh
PTC_CONTRACT_CONTAINER=page-to-calibre-contract mise exec go@1.27.3 -- go test ./internal/calibre -v
PTC_CONTRACT_CONTAINER=page-to-calibre-contract pnpm --dir web e2e
```

Instale o Chromium do Playwright no ambiente de desenvolvimento quando necessário. O MSW é importado apenas pelos testes Vitest; não existe service worker de interceptação no produto. Nunca use biblioteca pessoal nem credenciais reais em fixtures. Testes Go seguem a convenção idiomática `*_test.go`.
