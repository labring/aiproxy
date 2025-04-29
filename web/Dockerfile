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
# 安装需要的工具
RUN apk add --no-cache bash gettext
# 从构建阶段复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html
# 复制nginx配置文件
COPY nginx.template.conf /etc/nginx/templates/default.conf.template
# 设置健康检查
HEALTHCHECK --interval=30s --timeout=3s CMD wget -q --spider http://localhost:80/ || exit 1
# 设置默认环境变量
ENV API_BACKEND_URL="http://localhost:8080"
EXPOSE 80
# 改进的启动脚本，修复路径处理
CMD ["sh", "-c", "API_URL=\"$API_BACKEND_URL\"; if [ \"${API_URL: -1}\" != \"/\" ]; then API_URL=\"${API_URL}/\"; fi; echo \"使用API后端: $API_URL\"; export API_BACKEND_URL=\"$API_URL\"; envsubst '$${API_BACKEND_URL}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"]