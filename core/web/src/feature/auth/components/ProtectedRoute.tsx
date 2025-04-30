import { useEffect } from 'react'
import { useNavigate, useLocation, Outlet } from 'react-router'
import useAuthStore from '@/store/auth'

export function ProtectedRoute() {
    const { isAuthenticated } = useAuthStore()
    const navigate = useNavigate()
    const location = useLocation()

    useEffect(() => {
        if (!isAuthenticated) {
            // Redirect to login, but save the current location
            navigate('/login', { state: { from: location } })
        }
    }, [isAuthenticated, navigate, location])

    // If authenticated, render children
    return isAuthenticated ? <Outlet /> : null
}