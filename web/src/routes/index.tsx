// src/routes/AppRouter.tsx
import { createBrowserRouter, RouteObject, RouterProvider } from "react-router"
import { useRoutes } from "./config"
import { RouteErrorBoundary } from "@/handler/ErrorBoundary"

export function AppRouter() {
    // use existing routes config
    const routes = useRoutes()

    // iterate routes and add errorElement
    const routesWithErrorHandling = addErrorElementToRoutes(routes)

    // create router
    const router = createBrowserRouter(routesWithErrorHandling)

    return <RouterProvider router={router} />
}

// recursive add error handling
function addErrorElementToRoutes(routes: RouteObject[]) {
    return routes.map(route => {
        // add error element to each route
        const updatedRoute = {
            ...route,
            errorElement: <RouteErrorBoundary />
        }

        // recursive handle sub routes
        if (route.children) {
            updatedRoute.children = addErrorElementToRoutes(route.children)
        }

        return updatedRoute
    })
}