# Yuheng collaboration service.
#
# Build context must be the repository root, because the service bundles the
# shared document schema from packages/docs-schema:
#
#   docker build -f collab/Dockerfile -t yuheng-collab .

FROM node:24-alpine AS build
WORKDIR /src
COPY collab/package.json collab/package-lock.json ./collab/
RUN cd collab && npm ci --no-audit --no-fund
COPY packages/docs-schema ./packages/docs-schema
COPY collab ./collab
RUN cd collab && npm run build && npm prune --omit=dev

FROM node:24-alpine
ENV NODE_ENV=production \
    COLLAB_PORT=1234
WORKDIR /app
RUN addgroup -S collab && adduser -S -G collab collab
COPY --from=build /src/collab/dist ./dist
COPY --from=build /src/collab/node_modules ./node_modules
COPY collab/package.json ./package.json
USER collab
EXPOSE 1234
# Checked with node rather than wget, because busybox wget ignores no_proxy.
# On a host with an HTTP proxy configured, Docker passes http_proxy into every
# container, and busybox sends even a request to 127.0.0.1 through it — the
# proxy then refuses to route back to the caller and the check fails with a
# 403 while the service is perfectly healthy. Node's fetch does not read the
# proxy variables at all, and it is already in the image: this is a node
# service, so checking it with node is one less thing that can disagree.
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:'+process.env.COLLAB_PORT+'/healthz').then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))"
CMD ["node", "dist/main.js"]
