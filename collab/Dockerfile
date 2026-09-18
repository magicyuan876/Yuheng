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
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${COLLAB_PORT}/healthz" || exit 1
CMD ["node", "dist/main.js"]
