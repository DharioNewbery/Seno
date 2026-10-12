# Dockerfile do web Seno (multi-stage; SvelteKit com adapter-node).
FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# Origem do self-URL (CSRF/form actions) gravada no build; valida em compose.
ARG SENO_SITE_ORIGIN
RUN npm run build

FROM node:22-alpine
WORKDIR /app
COPY --from=build /src/build ./build
COPY --from=build /src/node_modules ./node_modules
COPY --from=build /src/package.json ./package.json
USER node
EXPOSE 3000
CMD ["node", "build"]
