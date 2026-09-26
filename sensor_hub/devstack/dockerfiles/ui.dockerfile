FROM node:25
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN --mount=type=cache,id=sensor-hub-dev-npm,target=/root/.npm npm ci
COPY . .
CMD ["npm", "run", "dev", "--", "--host"]
