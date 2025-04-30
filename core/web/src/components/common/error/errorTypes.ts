import { ApiError } from '@/api/index'

// 错误类型枚举
export const ErrorType = {
    NETWORK: 'network',
    TIMEOUT: 'timeout',
    FORBIDDEN: 'forbidden',
    UNAUTHORIZED: 'unauthorized',
    SERVER: 'server',
    CLIENT: 'client',
    VALIDATION: 'validation',
    UNKNOWN: 'unknown'
} as const

// 定义错误类型类型
export type ErrorTypeValue = typeof ErrorType[keyof typeof ErrorType]

/**
 * 根据错误对象确定错误类型
 * @param error - 错误对象
 * @returns 错误类型
 */
export const getErrorType = (error: unknown): ErrorTypeValue => {
    // 判断是否为ApiError
    const isApiError = error instanceof Error && error.name === 'ApiError'

    // 如果是ApiError，使用code判断错误类型
    if (isApiError && 'code' in error) {
        const apiError = error as ApiError
        const code = apiError.code

        if (code === 401) {
            return ErrorType.UNAUTHORIZED
        } else if (code === 403) {
            return ErrorType.FORBIDDEN
        } else if (code === 408 || code === 504) {
            return ErrorType.TIMEOUT
        } else if (code >= 400 && code < 500) {
            // 判断是否为验证错误
            if ('errorDetail' in apiError && apiError.errorDetail && typeof apiError.errorDetail === 'object') {
                return ErrorType.VALIDATION
            }
            return ErrorType.CLIENT
        } else if (code >= 500) {
            return ErrorType.SERVER
        }
    } else if (error instanceof Error) {
        // 非ApiError情况，尝试从消息判断
        const message = error.message.toLowerCase()
        if (message.includes('network') || message.includes('连接') || message.includes('connection')) {
            return ErrorType.NETWORK
        } else if (message.includes('timeout') || message.includes('超时')) {
            return ErrorType.TIMEOUT
        }
    }

    return ErrorType.UNKNOWN
}