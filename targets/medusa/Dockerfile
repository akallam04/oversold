FROM node:22.23.3-bookworm-slim AS build
ARG MEDUSA_VERSION
WORKDIR /app
COPY versions/${MEDUSA_VERSION}/package.json versions/${MEDUSA_VERSION}/package-lock.json ./
RUN npm ci --legacy-peer-deps --no-audit --no-fund
COPY app/ ./
RUN npx medusa build

FROM node:22.23.3-bookworm-slim AS release
ARG MEDUSA_VERSION
ENV NODE_ENV=production
ENV MEDUSA_VERSION=${MEDUSA_VERSION}
LABEL org.opencontainers.image.version=${MEDUSA_VERSION}
WORKDIR /server
RUN chown node:node /server
USER node
COPY --chown=node:node --from=build /app/.medusa/server ./
COPY --chown=node:node versions/${MEDUSA_VERSION}/package-lock.json ./
RUN npm ci --legacy-peer-deps --omit=dev --no-audit --no-fund
EXPOSE 9000
CMD ["node_modules/.bin/medusa", "start"]

FROM release AS variant
ARG VARIANT
COPY --chown=node:node variants/${VARIANT}/files/ /server/node_modules/
