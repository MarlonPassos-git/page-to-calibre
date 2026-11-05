# API v1

Página e API usam a mesma origem. Não há CORS aberto nem credenciais do Calibre no frontend. Os IDs são opacos e não concedem acesso fora da rede privada.

Antes de um POST, faça `GET /api/v1/session` e leia `csrf_token`. Envie-o em `X-CSRF-Token`, com `Content-Type: application/json`. O token muda a cada reinício; recupere-o novamente em `csrf_forbidden`. Clientes de navegador precisam de Origin igual a `PTC_ORIGIN`; o Host também deve corresponder. Chamadas CLI podem omitir Origin e continuam exigindo JSON e token. Corpos são limitados a 32 KiB.

## Preparar

`POST /api/v1/previews`, corpo `{"url":"https://example.org/article"}`: HTTP 202 com `id`, `url`, `state: queued`, `expires_at` (Unix segundos). Consulte `GET /api/v1/previews/{id}`. Estados: `queued`, `ready`, `failed`. Quando pronta, `snapshot` inclui `metadata`, `source`, `excerpt`, `warnings`. O HTML do artigo permanece no backend. Preparar não adiciona livros.

## Importar

`POST /api/v1/imports` exige `Idempotency-Key` de 16–128 caracteres (letras, números, `_`, `-`) e corpo:

```json
{
  "preview_id": "ID_DA_PREVIA",
  "metadata": {
    "title": "Título revisado",
    "author": "",
    "date": "",
    "tags": ["Artigos"],
    "language": "pt-BR",
    "site": "Exemplo",
    "description": ""
  }
}
```

Título: 1–500 caracteres. Data: `YYYY-MM-DD` ou vazio. Até 30 etiquetas com até 100 caracteres cada. Campos textuais restantes: até 4.000 caracteres. A mesma chave e payload retornam a mesma operação; mudar o payload com a chave anterior gera HTTP 409.

HTTP 202 confirma persistência da operação, não a inclusão no Calibre. Consulte `GET /api/v1/imports/{id}`. Estados: `queued`, `running`, `succeeded`, `failed`, `uncertain`. Etapas: `queued`, `packaging`, `writing`, `confirmed`. `result` confirmado inclui `id`, `title`, `url`, `duplicate`, `formats`, `identifiers`. Falhas preservam a prévia até a retenção; uma prévia expirada gera HTTP 410. O cliente pode preparar de novo mantendo suas edições.

## Erros e retenção

Erros HTTP seguem `{"error":{"code":"...","message":"...","stage":"..."}}`. Operações falhadas/incertas retornam HTTP 200 na consulta com seu estado e mensagem. Não interprete HTTP 200 ou 202 como sucesso de importação.

Resultados confirmados/falhados expiram conforme a retenção. Trabalhos ativos e incertos conservam os snapshots necessários. Sem histórico geral na interface. `GET /healthz` responde apenas `{"status":"ok"}` e não acessa origem nem Calibre.
