import axios, { AxiosError, AxiosRequestConfig, AxiosResponse } from 'axios'
import { useAuthStore } from '@/store/auth'
import { ENV } from '@/utils/env'

// ======================
// API 响应类型定义
// ======================

/**
 * 通用API响应接口
 */
export interface APIResponse<T = unknown> {
    message: string
    success: boolean
    data?: T
}

/**
 * Custom API error class
 */
export class ApiError extends Error {
    code: number

    constructor(message: string, code: number) {
        super(message)
        this.name = 'ApiError'
        this.code = code
    }
}

// 定义基础请求和响应接口
export type ApiRequestData = Record<string, unknown>
export type ApiResponseData = Record<string, unknown>

// ======================
// API 客户端配置
// ======================

const API_BASE_URL = ENV.API_BASE_URL || '/api'
const API_TIMEOUT = Number(ENV.API_TIMEOUT || 10000)

// 创建axios实例
const apiClient = axios.create({
    baseURL: API_BASE_URL,
    timeout: API_TIMEOUT,
    headers: {
        'Content-Type': 'application/json',
    },
})

// 请求拦截器
apiClient.interceptors.request.use(
    (config) => {
        const token = useAuthStore.getState().token

        if (token && config.headers) {
            config.headers.Authorization = `${token}`
        }

        return config
    },
    (error) => {
        return Promise.reject(error)
    }
)

// 响应拦截器 - 统一处理错误和响应格式
apiClient.interceptors.response.use(
    (response) => {
        // Check if response format matches API standard format
        const data = response.data as APIResponse

        // If response has success field and it's false, consider it a business logic error
        if (data && data.success === false) {
            throw new ApiError(
                data.message || 'Request failed',
                response.status
            )
        }

        return response
    },
    (error: AxiosError<APIResponse>) => {
        const status = error.response?.status
        const errorData = error.response?.data

        // Handle 401 unauthorized error
        if (status === 401) {
            useAuthStore.getState().logout()
            window.location.href = '/login'
        }

        // Convert to custom API error object
        if (errorData) {
            throw new ApiError(
                errorData.message || 'Request failed',
                status || 500
            )
        } else {
            // Network error or other non-standard error
            throw new ApiError(
                error.message || 'Network request failed',
                status || 500
            )
        }
    }
)

// 增加请求重试功能的封装方法
const withRetry = async <T>(
    requestFn: () => Promise<T>,
    maxRetries = 3,
    delay = 1000
): Promise<T> => {
    let retries = 0

    while (retries < maxRetries) {
        try {
            return await requestFn()
        } catch (error) {
            // 使用类型断言而不是 any
            const err = error as Error
            if (error instanceof ApiError && error.code >= 500 && retries < maxRetries - 1) {
                // 只有服务器错误才重试，并且不是最后一次尝试
                retries++
                await new Promise(resolve => setTimeout(resolve, delay * retries))
                continue
            }
            throw err
        }
    }

    throw new Error('Max retries reached')
}

// ======================
// API 请求方法
// ======================

/**
 * GET请求
 * @param url 请求URL
 * @param config 请求配置
 * @returns 响应数据
 */
export const get = <T = ApiResponseData>(url: string, config?: AxiosRequestConfig): Promise<T> => {
    return apiClient.get<APIResponse<T>>(url, config)
        .then((response: AxiosResponse<APIResponse<T>>) => {
            return response.data.data as T
        })
}

/**
 * POST请求
 * @param url 请求URL
 * @param data 请求数据
 * @param config 请求配置
 * @returns 响应数据
 */
export const post = <T = ApiResponseData>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> => {
    return apiClient.post<APIResponse<T>>(url, data, config)
        .then((response: AxiosResponse<APIResponse<T>>) => {
            return response.data.data as T
        })
}

/**
 * PUT请求
 * @param url 请求URL
 * @param data 请求数据
 * @param config 请求配置
 * @returns 响应数据
 */
export const put = <T = ApiResponseData>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> => {
    return apiClient.put<APIResponse<T>>(url, data, config)
        .then((response: AxiosResponse<APIResponse<T>>) => {
            return response.data.data as T
        })
}

/**
 * PATCH请求
 * @param url 请求URL
 * @param data 请求数据
 * @param config 请求配置
 * @returns 响应数据
 */
export const patch = <T = ApiResponseData>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> => {
    return apiClient.patch<APIResponse<T>>(url, data, config)
        .then((response: AxiosResponse<APIResponse<T>>) => {
            return response.data.data as T
        })
}

/**
 * DELETE请求
 * @param url 请求URL
 * @param config 请求配置
 * @returns 响应数据
 */
export const del = <T = ApiResponseData>(url: string, config?: AxiosRequestConfig): Promise<T> => {
    return apiClient.delete<APIResponse<T>>(url, config)
        .then((response: AxiosResponse<APIResponse<T>>) => {
            return response.data.data as T
        })
}

/**
 * 带重试功能的GET请求
 * @param url 请求URL
 * @param config 请求配置
 * @param retries 重试次数
 * @returns 响应数据
 */
export const getWithRetry = <T = ApiResponseData>(url: string, config?: AxiosRequestConfig, retries = 3): Promise<T> => {
    return withRetry(() => get<T>(url, config), retries)
}

/**
 * 带重试功能的POST请求
 * @param url 请求URL
 * @param data 请求数据
 * @param config 请求配置
 * @param retries 重试次数
 * @returns 响应数据
 */
export const postWithRetry = <T = ApiResponseData>(url: string, data?: unknown, config?: AxiosRequestConfig, retries = 3): Promise<T> => {
    return withRetry(() => post<T>(url, data, config), retries)
}

/**
 * 带重试功能的PUT请求
 * @param url 请求URL
 * @param data 请求数据
 * @param config 请求配置
 * @param retries 重试次数
 * @returns 响应数据
 */
export const putWithRetry = <T = ApiResponseData>(url: string, data?: unknown, config?: AxiosRequestConfig, retries = 3): Promise<T> => {
    return withRetry(() => put<T>(url, data, config), retries)
}

/**
 * 带重试功能的DELETE请求
 * @param url 请求URL
 * @param config 请求配置
 * @param retries 重试次数
 * @returns 响应数据
 */
export const delWithRetry = <T = ApiResponseData>(url: string, config?: AxiosRequestConfig, retries = 3): Promise<T> => {
    return withRetry(() => del<T>(url, config), retries)
}

/**
 * 带重试功能的PATCH请求
 * @param url 请求URL
 * @param data 请求数据
 * @param config 请求配置
 * @param retries 重试次数
 * @returns 响应数据
 */
export const patchWithRetry = <T = ApiResponseData>(url: string, data?: unknown, config?: AxiosRequestConfig, retries = 3): Promise<T> => {
    return withRetry(() => patch<T>(url, data, config), retries)
}


export default apiClient