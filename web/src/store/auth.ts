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
    sessionToken: string | null  // JWT session token (independent of API keys)
    isAuthenticated: boolean
    isAuthenticating: boolean
    enterpriseUser: EnterpriseUser | null
    login: (token: string) => void
    loginWithFeishu: (sessionToken: string, user: EnterpriseUser) => void
    logout: () => void
    setToken: (token: string) => void
    refreshSessionToken: (newToken: string) => void
}

export const useAuthStore = create<AuthState>()(
    persist(
        (set) => ({
            token: null,
            sessionToken: null,
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

            loginWithFeishu: (sessionToken: string, user: EnterpriseUser) => {
                set({
                    sessionToken,
                    token: sessionToken, // backward compat: keep token in sync
                    isAuthenticated: true,
                    enterpriseUser: user,
                })
            },

            logout: () => {
                set({
                    token: null,
                    sessionToken: null,
                    isAuthenticated: false,
                    enterpriseUser: null,
                })
            },

            setToken: (token: string) => {
                set({
                    token,
                })
            },

            refreshSessionToken: (newToken: string) => {
                set({
                    sessionToken: newToken,
                    token: newToken,
                })
            },
        }),
        {
            name: 'auth-storage',
            partialize: (state) => ({
                token: state.token,
                sessionToken: state.sessionToken,
                isAuthenticated: state.isAuthenticated,
                enterpriseUser: state.enterpriseUser,
            }),
        }
    )
)

export default useAuthStore