import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface EnterpriseUser {
    name: string
    avatar: string
    openId: string
    role: 'viewer' | 'analyst' | 'admin'
}

export interface AuthState {
    token: string | null
    isAuthenticated: boolean
    isAuthenticating: boolean
    enterpriseUser: EnterpriseUser | null
    login: (token: string) => void
    loginWithFeishu: (token: string, user: EnterpriseUser) => void
    logout: () => void
    setToken: (token: string) => void
}

export const useAuthStore = create<AuthState>()(
    persist(
        (set) => ({
            token: null,
            isAuthenticated: false,
            isAuthenticating: false,
            enterpriseUser: null,

            login: (token: string) => {
                set({
                    token,
                    isAuthenticated: true,
                    enterpriseUser: null,
                })
            },

            loginWithFeishu: (token: string, user: EnterpriseUser) => {
                set({
                    token,
                    isAuthenticated: true,
                    enterpriseUser: user,
                })
            },

            logout: () => {
                set({
                    token: null,
                    isAuthenticated: false,
                    enterpriseUser: null,
                })
            },

            setToken: (token: string) => {
                set({
                    token,
                })
            },
        }),
        {
            name: 'auth-storage',
            partialize: (state) => ({
                token: state.token,
                isAuthenticated: state.isAuthenticated,
                enterpriseUser: state.enterpriseUser,
            }),
        }
    )
)

export default useAuthStore 