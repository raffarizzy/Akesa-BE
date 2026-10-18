FROM node:20-alpine

WORKDIR /app

# Install netcat for port readiness check in entrypoint
RUN apk add --no-cache netcat-openbsd

COPY package.json package-lock.json ./
RUN npm ci

COPY hardhat.config.cjs ./
COPY contracts ./contracts
COPY scripts ./scripts
COPY entrypoint.sh ./

RUN chmod +x entrypoint.sh

EXPOSE 8545

ENTRYPOINT ["/app/entrypoint.sh"]
