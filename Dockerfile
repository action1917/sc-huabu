# syntax=m.daocloud.io/docker.io/docker/dockerfile:1.7

# 构建 Vite 前端产物。
FROM m.daocloud.io/docker.io/oven/bun:1.3.13 AS web-build

WORKDIR /app/web
ARG VITE_TLDRAW_LICENSE_KEY
ENV VITE_TLDRAW_LICENSE_KEY=${VITE_TLDRAW_LICENSE_KEY}
COPY web/package.json web/bun.lock ./
# 并发构建中断可能留下不完整的包链接；强制校验安装可避免 tsc 等二进制缺文件。
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --force --cache-dir=/root/.bun/install/cache
COPY VERSION /app/VERSION
COPY CHANGELOG.md /app/CHANGELOG.md
COPY web ./
# TypeScript 7 的平台编译器位于可选平台包；直接调用可避开 Bun 1.3 生成的失效 .bin/tsc 链接。
RUN ./node_modules/@typescript/typescript-linux-x64/lib/tsc --noEmit \
    && bun node_modules/vite/bin/vite.js build

# 运行镜像：nginx 托管静态前端，并在 Compose 中把 /api 转发到后端服务。
FROM m.daocloud.io/docker.io/library/nginx:1.27-alpine

COPY --from=web-build /app/web/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:3000/ >/dev/null || exit 1
