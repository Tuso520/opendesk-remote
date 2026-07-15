FROM node:22-bookworm AS builder
WORKDIR /src
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /src/dist /usr/share/nginx/html
EXPOSE 80

