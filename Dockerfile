# 基础镜像：使用官方 Node.js 18 长期支持版（Alpine 轻量化版本）
FROM node:18-alpine

# 设置工作目录
WORKDIR /app

# 复制 package.json 和 package-lock.json（优先安装依赖，利用 Docker 缓存）
COPY package*.json ./

# 安装依赖（Alpine 需先安装 git，因为 Sub2API 依赖含 git 包）
RUN apk add --no-cache git && \
    npm install --production

# 复制所有源代码到工作目录
COPY . .

# 暴露端口（必须与 Sub2API 配置的 PORT 一致，默认 3000）
EXPOSE 3000

# 启动命令（生产环境启动）
CMD ["npm", "run", "start:prod"]
