FROM node:23-alpine as builder

WORKDIR /app

# 安装pnpm
RUN npm install -g pnpm

# 复制package.json和pnpm-lock.yaml
COPY package.json pnpm-lock.yaml ./

# 安装依赖
RUN pnpm install

# 复制所有源代码
COPY . .

# 构建应用
RUN pnpm build

# 第二阶段：使用nginx来提供静态文件服务
FROM nginx:alpine

# 安装envsubst工具
RUN apk add --no-cache bash gettext

# 创建模板目录
RUN mkdir -p /etc/nginx/templates

# 复制nginx配置模板
COPY nginx.template.conf /etc/nginx/templates/

# 复制启动脚本并设置执行权限
COPY docker-entrypoint.sh /
RUN chmod +x /docker-entrypoint.sh

# 从构建阶段复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=3s CMD wget -q --spider http://localhost:80/ || exit 1

# 设置环境变量
ENV API_BACKEND_URL="http://localhost:8080"

EXPOSE 80

# 使用启动脚本替代直接启动nginx
CMD ["/docker-entrypoint.sh"] 