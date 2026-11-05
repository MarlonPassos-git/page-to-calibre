# Componentes distribuídos

O código page-to-calibre tem licença MIT. Componentes independentes mantêm suas licenças:

- Readeck go-readability v2.1.3, MIT; inclui linhagem Mozilla Readability e seus avisos.
- go-shiori/go-epub v1.2.1, MIT, derivado de bmaupin/go-epub.
- bluemonday v1.0.27, BSD-3-Clause.
- modernc.org/sqlite, BSD-3-Clause; SQLite em domínio público. Consulte também as dependências transitivas fixadas em `go.mod`.
- Calibre 9.15.0 e seu runtime, GPLv3 e licenças de componentes incluídas no pacote oficial. Binário fixado e SHA-512 no Dockerfile. [Código-fonte da mesma versão](https://download.calibre-ebook.com/9.15.0/calibre-9.15.0.tar.xz) e [licença](https://github.com/kovidgoyal/calibre/blob/v9.15.0/LICENSE).
- Pacotes Debian mantêm copyrights em `/usr/share/doc/*/copyright` na imagem. [Fontes Debian](https://sources.debian.org/).

Ferramentas de build/teste não são distribuídas como dependência de execução do frontend. A imagem contém apenas os assets compilados. O build copia as licenças e avisos disponíveis na raiz de cada módulo Go para `/usr/share/doc/page-to-calibre/dependencies`, além da licença do Calibre para `/usr/share/doc/calibre/LICENSE`. As fontes correspondentes ao Calibre ficam no link da versão acima; o Dockerfile mantém versão e hash do binário.
