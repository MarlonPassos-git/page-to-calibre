# syntax=docker/dockerfile:1
FROM node:24.22.2-bookworm-slim@sha256:01d566e8d94d8c619591d78652babfccd3c6992a3dffa9ed5ab39e11cf6c70e9 AS web-build
WORKDIR /src/web
RUN npm install -g pnpm@12.14.0
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.27.3-bookworm@sha256:c93aa68d51e382ec1ab4caa559ae9efd7858d8e5330051671c56c1096eaccd66 AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/page-to-calibre/ cmd/page-to-calibre/
COPY internal/ internal/
COPY tools/licenses/ tools/licenses/
RUN go run ./tools/licenses /licenses
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /page-to-calibre ./cmd/page-to-calibre

FROM debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2 AS calibre-download
# Package archive and checksum are published by calibre; no installer is piped
# into a shell and no network download is performed by the running application.
RUN rm -f /etc/apt/sources.list.d/debian.sources \
 && printf 'deb [check-valid-until=no] http://snapshot.debian.org/archive/debian/20261105T000000Z trixie main\ndeb [check-valid-until=no] http://snapshot.debian.org/archive/debian-security/20261105T000000Z trixie-security main\n' > /etc/apt/sources.list \
 && apt-get update && apt-get install -y --no-install-recommends curl ca-certificates xz-utils && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://download.calibre-ebook.com/9.15.0/calibre-9.15.0-x86_64.txz -o /calibre.txz \
 && echo '77fb209188906a1be5ca1e8147339bd06242414ffbfe7dbc54e65667c8d6057fdc6e709efff0c415c42fb95ca5f53359ae041d5089cd9927a0d1d74d9fc8f4a5  /calibre.txz' | sha512sum -c - \
 && curl -fsSL https://raw.githubusercontent.com/kovidgoyal/calibre/v9.15.0/LICENSE -o /calibre-LICENSE \
 && echo '3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986  /calibre-LICENSE' | sha256sum -c - \
 && mkdir /calibre && tar -xJf /calibre.txz -C /calibre && chmod -R a+rX /calibre

FROM debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2 AS runtime
RUN rm -f /etc/apt/sources.list.d/debian.sources \
 && printf 'deb [check-valid-until=no] http://snapshot.debian.org/archive/debian/20261105T000000Z trixie main\ndeb [check-valid-until=no] http://snapshot.debian.org/archive/debian-security/20261105T000000Z trixie-security main\n' > /etc/apt/sources.list \
 && apt-get update && apt-get install -y --no-install-recommends ca-certificates libegl1 libopengl0 libxcb-cursor0 libxkbcommon0 libxrender1 libnss3 libxcomposite1 libasound2t64 libxdamage1 libxrandr2 libfontconfig1 libglx0 libfreetype6 \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /data /app && chown 10001:10001 /data /app
COPY --from=calibre-download /calibre /opt/calibre
COPY --from=calibre-download /calibre-LICENSE /usr/share/doc/calibre/LICENSE
ENV PATH="/opt/calibre:${PATH}" QT_QPA_PLATFORM=offscreen HOME=/data PTC_DATA_DIR=/data PTC_WEB_DIR=/app/web PTC_LISTEN=0.0.0.0:8091
COPY --from=go-build /licenses /usr/share/doc/page-to-calibre/dependencies
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/doc/page-to-calibre/
COPY --from=go-build /page-to-calibre /usr/local/bin/page-to-calibre
COPY --from=web-build /src/web/dist /app/web
USER 10001:10001
WORKDIR /app
RUN calibredb --version
EXPOSE 8091
HEALTHCHECK --interval=30s --timeout=4s --start-period=10s --retries=3 CMD ["page-to-calibre", "healthcheck"]
ENTRYPOINT ["page-to-calibre"]
