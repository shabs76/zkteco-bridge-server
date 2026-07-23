# Multi-stage build for the ZKBio management console (Angular SPA → nginx).
#
# node:*-alpine and nginx:*-alpine are multi-arch images, and `ng build` emits
# arch-independent static files, so buildx produces the arm64 image for the
# Raspberry Pi with no cross-compilation dance (unlike the Go gateway image).
FROM node:22-alpine AS builder

WORKDIR /app

# Install deps first so the layer caches when only source changes.
COPY package.json package-lock.json* ./
RUN npm ci

COPY . .
RUN npm run build

# ── Serve the static bundle with nginx ──────────────────────────────────────
FROM nginx:1.27-alpine

# SPA fallback + gzip; see nginx.conf.
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/web/browser /usr/share/nginx/html

EXPOSE 80
