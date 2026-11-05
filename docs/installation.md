# Instalação e operação

## Calibre

Use um Content server ativo com autenticação. A porta da interface gráfica, a porta do Content server e a conexão sem fio são serviços diferentes. Este aplicativo acessa somente o Content server pelo `calibredb` oficial. [Contrato oficial](https://manual.calibre-ebook.com/generated/en/calibredb.html).

Crie um usuário de escrita próprio, limitado à biblioteca alvo, preservando os usuários de leitura existentes. Descubra o ID da biblioteca com `calibredb list --with-library 'http://HOST:PORT/#-' --username USUARIO --password '<f:/caminho/secret>'`. O arquivo deve ser legível pelo UID 10001 do aplicativo, sem ficar legível por outros usuários do host. Em uma GUI Calibre, gerencie os usuários em Preferências → Compartilhamento pela rede. Não ative escrita anônima para viabilizar a integração.

Não monte a biblioteca no aplicativo nem edite diretamente seu banco em uso. O aplicativo não recebe socket Docker. A conexão Calibre é configurada pelo operador; o capturador de artigos é outro cliente e bloqueia destinos internos.

## Docker

Pré-requisitos: Docker/Compose, Calibre configurado e alcançável na rede Docker, arquivo de senha e espaço para imagem/dados. A imagem atual é Linux amd64 porque usa o binário oficial x86_64 fixado no Dockerfile. O frontend usa Node somente no build.

Copie `.env.example` para `.env` fora do controle de versão, ajuste os endereços e o arquivo de senha, conecte o serviço à mesma rede Docker do Calibre e execute:

```sh
docker compose config --quiet
docker compose build
docker compose up -d
```

O exemplo publica apenas `127.0.0.1:8091`. Para acesso remoto, configure HTTPS privado, por exemplo com Tailscale, e ajuste `PTC_ORIGIN` para a URL exata, sem barra final. O proxy precisa preservar Host; `X-Forwarded-Host` não é aceito como autoridade. Não exponha diretamente à internet: esta versão não possui autenticação própria de usuários.

A aplicação executa como UID/GID 10001, raiz da imagem somente leitura e volume gravável em `/data`. Secrets e `/data` precisam das permissões correspondentes. Verifique `GET /healthz`, prepare uma URL pública e confirme o livro no Calibre antes de considerar a configuração pronta.

## Variáveis

| Variável | Valor padrão / contrato |
| --- | --- |
| `PTC_LISTEN` | `127.0.0.1:8091` no executável; `0.0.0.0:8091` na imagem |
| `PTC_ORIGIN` | `http://127.0.0.1:8091`, origem externa exata |
| `PTC_DATA_DIR` | `./data` no executável; `/data` na imagem |
| `PTC_WEB_DIR` | `./web/dist` no executável; `/app/web` na imagem |
| `CALIBRE_URL` | Obrigatória, HTTP(S), sem query/fragmento/credenciais |
| `CALIBRE_PUBLIC_URL` | Endereço do Content server alcançável pelo navegador, padrão `CALIBRE_URL` |
| `CALIBRE_LIBRARY` | ID da biblioteca, obrigatório |
| `CALIBRE_USERNAME` | Usuário exclusivo de escrita, obrigatório |
| `CALIBRE_PASSWORD_FILE` | Caminho absoluto do secret dentro do container |
| `CALIBRE_PASSWORD_HOST_FILE` | Caminho do secret no host, usado pelo Compose |
| `PTC_FETCH_TIMEOUT_SECONDS` | 30, intervalo 1–120, por recurso |
| `PTC_MAX_RESOURCE_BYTES` | 8388608, intervalo 1024–33554432, por recurso |
| `PTC_MAX_IMAGE_BYTES` | 33554432, intervalo 1024–268435456, soma das imagens empacotadas |
| `PTC_MAX_IMAGES` | 30, intervalo 0–100; imagens adicionais omitidas com aviso |
| `PTC_PREVIEW_TTL_MINUTES` | 60, intervalo 1–1440 |
| `PTC_RETENTION_DAYS` | 30, intervalo 1–365, para resultados terminados |

São permitidos até cinco redirecionamentos e portas de origem 80/443. DNS é validado antes da conexão e o socket usa o IP validado. Proxy de ambiente não é utilizado para capturar artigos. TLS mantém validação de certificados. As imagens suportadas para empacotamento são PNG/JPEG/GIF; outros formatos ficam como texto alternativo com aviso. Limites são por recurso, até `PTC_MAX_IMAGES`; capturas são serializadas pelo worker inicial.

Execute uma única instância do worker por banco e biblioteca alvo. Coordenação entre réplicas independentes não faz parte desta versão.

A identidade usa a URL final dos redirects, sem fragmentos, `utm_*`, `fbclid` ou `gclid`; parâmetros restantes são preservados. A URL original fica no snapshot e no EPUB; canonical do mesmo host é guardado como relação informativa. Canonical não substitui automaticamente essa identidade, e canonical de outro host é descartado. A deduplicação vale para o identificador estável `page-to-calibre`, não para títulos/autores. Não há transação distribuída com Calibre nem promessa de deduplicar aliases arbitrários.

## Backup e recuperação

Dados persistentes: volume `/data` inteiro (SQLite, WAL/SHM e snapshots no banco), configuração/Compose e arquivo de senha. A biblioteca/perfil/usuários do Calibre exigem seu próprio backup consistente; backup do Portainer não cobre esses dados.

Para cópia consistente simples, pare **somente este aplicativo**, copie `/data` e configuração/secrets para o conjunto protegido, faça o backup externo e então reinicie o aplicativo. Nenhuma requisição aceita depende da aba aberta. Operações interrompidas antes da escrita são retomadas; após o marcador de escrita, o worker consulta Calibre e confirma ou conserva estado incerto. O worker consulta novamente um estado incerto uma vez por minuto, sem executar outra inclusão. O EPUB exato permanece no volume até a confirmação; não reenviar manualmente enquanto a incerteza persistir.

Restaure em volume separado, com rede isolada e Calibre de teste ou acesso somente leitura. Compare banco/resultados, health e consultas; não restaure a biblioteca por cima da biblioteca ativa para testar. A proteção só está validada depois de backup externo e restauração separados. **Integração Backrest da instância pessoal ainda pendente; o serviço não é apresentado como protegido.**

## Reversão

Pare o serviço e remova sua publicação HTTPS. Preserve o volume e os secrets até decidir sua retenção; não use `docker compose down -v` para uma reversão rotineira. Revogue a credencial exclusiva quando a integração for removida definitivamente. Livros já confirmados continuam no Calibre e não são apagados automaticamente.
