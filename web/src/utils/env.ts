export const ENV = {
    NODE_ENV: import.meta.env.MODE || 'development',
    isDevelopment: import.meta.env.DEV,
    isProduction: import.meta.env.PROD,
    API_BASE_URL: import.meta.env.VITE_API_BASE_URL,
    API_TIMEOUT: import.meta.env.VITE_API_TIMEOUT,
}