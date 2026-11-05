# Validação técnica local — 05/11/2026

## Resultado comprovado

A abordagem Go foi mantida: Readeck go-readability v2.1.3 extraiu um artigo público, e Shiori go-epub v1.2.1 gerou EPUB legível estruturalmente. O artigo de avaliação foi [Go 1.26 is released](https://go.dev/blog/go1.26), sem autenticação ou execução de JavaScript. Metadados da fonte foram preservados, sem substituir ausências por autores/datas inventados.

O adaptador usou primeiro Calibre 8.5 do Debian para explorar o contrato e depois repetiu os ensaios com o binário oficial **9.15.0**, usado no Dockerfile. O contrato oficial, a biblioteca isolada e o frontend integrado passaram com 9.15.0. Nenhuma biblioteca pessoal participou desses testes.

## Verificações

| Verificação | Evidência e alcance |
| --- | --- |
| Go | `go test ./...`, `go test -race ./...`, `go vet ./...` e gofmt; extração, metadados, rede, EPUB, adaptador, SQLite, worker e API |
| Rede | Timeout, tamanho máximo, redirects limitados, redirect para localhost/IP privado/metadados, validação DNS antes do socket e rejeição de destinos não públicos |
| EPUB | ZIP/XML/manifest/nav, título editado, origem, etiquetas, código e imagem offline; EPUBCheck 4.2.6 com regras EPUB 3.2 |
| Calibre real | Autenticação, biblioteca incorreta, usuário sem escrita, inclusão confirmada e reenvio sem modificação; `./integration/check.sh` cria e remove sua própria instância sintética |
| Resposta perdida/concorrência | Testes controlados de reconciliação, serialização e bloqueio de outra chave enquanto houver escrita incerta |
| Persistência | Reabertura do banco, reaproveitamento do snapshot sem nova captura do texto, recuperação de escrita interrompida e retenção de snapshots/EPUBs ativos ou incertos |
| Interface | Oito testes Vitest/MSW: debounce, ausência de erro prematuro, edição, clique durante preparo, resposta antiga, retomada, expiração e resposta de aceite perdida |
| Navegador | Quatro testes Playwright desktop/celular com API/SQLite/worker/EPUB/Calibre reais; só a página de origem é sintética; foco e ausência de overflow conferidos |
| Visual | Capturas desktop/celular inspecionadas, detector sem achados e revisão independente com disposição `ship`; sistema registrado em DESIGN.md e sidecar |
| Runtime | Build Docker, CLI 9.15, UID/GID 10001, raiz somente leitura e healthcheck barato; teste inicial sem conectividade ao Calibre para comprovar que health não captura/importa |

A versão inicial do gerador adicionava um campo ZIP extra ao `mimetype`; a reescrita preserva o arquivo primeiro e sem compressão/campos extras. EPUBCheck também identificou `creator` vazio quando o autor estava ausente: o aplicativo omite esse elemento, sem inventar autor. Esses comportamentos têm regressões no teste do artefato.

## Runtime e limites

O runtime com o pacote oficial Calibre ocupa aproximadamente **1,40 GB sem compressão**. O pacote traz bibliotecas do próprio Calibre, mas não há outro daemon ou GUI em execução. Node, MSW, Playwright, compilador Go e EPUBCheck ficam fora da imagem runtime.

No PC de desenvolvimento, um smoke test isolado mostrou aproximadamente **8–15 MiB** de memória e **0,12–0,16%** de CPU em duas amostras do `docker stats`; health permaneceu `healthy`. São amostras locais com healthcheck, sem carga de captura/Calibre, e não medem o Lenovo nem justificam promessa de consumo zero. Carga real, latência e consumo no servidor serão medidos na implantação.

Bases Docker estão fixadas por digest, Calibre por versão/SHA-512, licença por SHA-256 e pacotes Debian por snapshot de 05/11/2026. Go e frontend mantêm seus lockfiles. A imagem atual é amd64. Os limites por recurso, quantidade e soma de imagens estão em [instalação](installation.md).

## Pendências de entrega

Abertura no KOReader, backup externo/restauração e medições de uma instância instalada dependem da validação de cada ambiente. Os testes locais descritos aqui não comprovam esses itens em outras instalações.
