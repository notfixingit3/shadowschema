FROM node:22-bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY package.json package-lock.json ./
RUN npm ci

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# Optional: enables shadowschema_explore_target inside the container
RUN npx playwright install --with-deps chromium

ENV SHADOWSCHEMA_EXPORT_URL=http://host.docker.internal:38081
ENV SHADOWSCHEMA_PROXY_URL=http://host.docker.internal:38080

CMD ["node", "dist/index.js"]