# Resultado da prova de conceito

PoC encerrada em **05/11/2026**. O código, os testes, as dependências fixadas e os procedimentos históricos permanecem disponíveis sob suas licenças originais. O repositório foi arquivado; não há compromisso de manutenção ou novas versões.

## O que foi demonstrado

Captura de URLs públicas, extração e sanitização, EPUB com imagens, prévia editável, fila SQLite persistente, idempotência e reconciliação com o Calibre oficial. A implantação privada foi usada com artigos reais; importação, reenvio, reinício, leitura no KOReader, backup externo e restauração isolada foram exercitados.

## Comparação que encerrou o experimento

No mesmo Lenovo (Debian 13, i5-8400T, 12 GB), foram comparadas nove páginas, incluindo texto de aproximadamente 37,6 mil palavras e páginas com diagramas. Cada serviço teve 27 operações na rodada principal ajustada, divididas entre sequencial, dois e quatro clientes. O fluxo próprio incluiu Content server Calibre isolado; o Wallabag 2.6.15 usou SQLite e exportação EPUB.

| Medida | page-to-calibre + Calibre | Wallabag |
| --- | ---: | ---: |
| Lote sequencial de nove | 35,1 s | 33,8 s |
| Lote com dois clientes | 31,5 s | 19,6 s |
| Lote com quatro clientes | 33,0 s | 14,7 s |
| CPU total das três etapas | 65,6 s | 17,0 s |
| Pico simultâneo de RAM | 472 MiB | 139 MiB |

Limites por container: um núcleo e 768 MiB; o conjunto com Calibre tem dois containers, portanto não possui a mesma quota agregada do Wallabag. Rede externa real, um lote por nível e sem previsão estatística para outros sites. RAM desconta `inactive_file`, como no Docker Linux. O consumo total do Calibre de laboratório não representa o incremento em uma instalação Calibre já existente.

As duas soluções tiveram problemas na exploração inicial. Descrições extraídas de artigos de Paul Graham excederam os limites da API do page-to-calibre e impediram o envio direto. Na rodada principal, o harness limitou a descrição de um caso a 4.000 caracteres; o defeito do produto não foi corrigido. Uma exportação Wallabag de outra página excedeu 180 segundos; esse caso foi preservado como falha separada e excluído dos lotes de vazão. Portanto, os 54 sucessos da rodada ajustada não significam sucesso sem intervenção em todos os sites.

Os EPUBs foram conferidos quanto a ZIP, OPF, capítulos XML e referências de imagens. Contagem de palavras considera todos os capítulos de artigo; não é uma certificação completa de fidelidade ou compatibilidade. Páginas que dependem de JavaScript continuam com limitações.

## Decisão

Para guardar e ler artigos, foi escolhido o Wallabag, que já oferece uma biblioteca própria e integração com leitores, com menor custo do conjunto nesta carga. A integração específica com Calibre foi tecnicamente validada, mas não justificou manter uma aplicação própria para esse uso.

As instruções de instalação deste repositório documentam a PoC histórica. Não constituem indicação de serviço ativo nem promessa de suporte. A biblioteca Calibre e seus livros continuam independentes da aplicação arquivada.
