#!/bin/sh
set -e

echo "[Blockchain] Starting Hardhat EVM Node..."
npx hardhat node --hostname 0.0.0.0 &
NODE_PID=$!

echo "[Blockchain] Waiting for node to be available on port 8545..."
while ! nc -z localhost 8545; do
  sleep 0.5
done

echo "[Blockchain] Node is ready. Deploying AkesaAuditLedger smart contract..."
npx hardhat run scripts/deploy.js --network localhost

echo "[Blockchain] Deployment complete. EVM node listening on 0.0.0.0:8545"
wait $NODE_PID
