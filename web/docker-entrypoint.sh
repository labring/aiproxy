   #!/bin/sh
   
   # 设置默认后端地址（如果没有提供环境变量）
   : ${API_BACKEND_URL:="http://localhost:8080"}
   
   # 确保URL末尾有斜杠，避免路径拼接问题
   if [[ ! "$API_BACKEND_URL" =~ /$ ]]; then
     export API_BACKEND_URL="${API_BACKEND_URL}/"
   fi
   
   echo "Using API backend: $API_BACKEND_URL"
   
   # 生成nginx配置
   envsubst '${API_BACKEND_URL}' < /etc/nginx/templates/nginx.template.conf > /etc/nginx/conf.d/default.conf
   
   # 启动nginx
   exec nginx -g 'daemon off;'