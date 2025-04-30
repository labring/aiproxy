// src/feature/auth/hooks.ts
import { useMutation } from '@tanstack/react-query'
import { useNavigate, useLocation } from 'react-router'
import { authApi } from '@/api/services'
import { useAuthStore } from '@/store/auth'
import { toast } from 'sonner'
import { ApiError } from '@/api/index'

export function useLoginMutation() {
    const navigate = useNavigate()
    const location = useLocation()
    const { login } = useAuthStore()

    // get redirect url from location
    const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/'

    return useMutation({
        mutationFn: async (token: string) => {
            const result = await authApi.getChannelTypeMetas(token)
            return { token, result }
        },
        onSuccess: ({ token }) => {
            // login success, save token
            login(token)
            toast.success('login success')
            // redirect to previous page or home page
            navigate(from, { replace: true })
        },
        onError: (error: unknown) => {
            if (error instanceof ApiError) {
                if (error.code === 401) {
                    toast.error('Token无效，请重新输入')
                } else {
                    toast.error(`API错误 (${error.code}): ${error.message}`)
                }
            } else {
                toast.error('登录失败，请重试')
            }
        }
    })
}