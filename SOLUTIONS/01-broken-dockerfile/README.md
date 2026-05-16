# Solution — 01 broken Dockerfile

## The 5 bugs

| # | Original                       | Fix                                  | Why                                                                                  |
|---|--------------------------------|--------------------------------------|--------------------------------------------------------------------------------------|
| 1 | `FROM node:14-alpine`          | `FROM node:20-alpine`                | Node 14 is EOL — no security patches, npm warnings, modern features missing.         |
| 2 | (no `WORKDIR`)                 | `WORKDIR /app`                       | Without it, everything lands at `/`, polluting the root filesystem and confusing future `COPY`/`CMD` paths. |
| 3 | `COPY . .` then `RUN npm install` | `COPY package*.json ./` → `RUN npm install` → `COPY . .` | Layer caching: deps only re-install when `package.json` changes, not on every code edit. |
| 4 | (no `USER`)                    | `USER node`                          | Containers run as root by default. `node` is a built-in non-root user in the official Node images. |
| 5 | `CMD node server`              | `CMD ["node", "server.js"]`          | Shell form spawns an extra `/bin/sh` (no signal forwarding → slow `docker stop`); missing `.js` is a typo bomb. |

## One-paragraph reasoning

A production Node Dockerfile has four jobs: pick a supported runtime, install
dependencies in a way the layer cache can exploit, drop privileges, and start
the app reliably. The original failed each one. Pinning `node:20-alpine` gives
us a maintained LTS on a small base; `WORKDIR /app` localizes the filesystem;
copying `package*.json` and running `npm install` before copying source means
edits to `server.js` don't bust the dependency layer; `USER node` removes a
trivial container-escape blast radius; and the exec-form `CMD ["node",
"server.js"]` makes the Node process PID 1 so `SIGTERM` from `docker stop`
shuts it down cleanly in under a second instead of waiting 10s for SIGKILL.
