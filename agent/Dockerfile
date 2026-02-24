# The Koptan SelfService session image: node, git and the agent CLI.
FROM node:24-alpine
RUN apk add --no-cache git
WORKDIR /opt/koptan-agent
COPY package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY bin ./bin
COPY src ./src
# The operator runs the pod read-only with /tmp and /workspace writable.
ENV HOME=/tmp KOPTAN_AGENT_WORKSPACE=/workspace/repo NODE_ENV=production
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["node", "/opt/koptan-agent/bin/koptan-agent.js"]
CMD ["serve"]
