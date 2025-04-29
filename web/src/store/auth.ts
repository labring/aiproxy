import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface AuthState {
    token: string | null
    isAuthenticated: boolean
    isAuthenticating: boolean
    login: (token: string) => void
    logout: () => void
}

export const useAuthStore = create<AuthState>()(
    persist(
        (set) => ({
            token: null,
            isAuthenticated: false,
            isAuthenticating: false,

            login: (token: string) => {
                set({
                    token,
                    isAuthenticated: true,
                })
            },

            logout: () => {
                set({
                    token: null,
                    isAuthenticated: false,
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
            // Only persist these fields
            partialize: (state) => ({
                token: state.token,
                isAuthenticated: state.isAuthenticated,
            }),
        }
    )
)

export default useAuthStore 