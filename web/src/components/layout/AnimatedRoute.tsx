import type { ReactNode } from "react"
import { motion, useReducedMotion } from "motion/react"
import { useLocation } from "react-router"

interface AnimatedRouteProps {
    children: ReactNode
    transitionType?: "slide" | "fade" | "scale" | "flip"
}

export function AnimatedRoute({ children, transitionType = "fade" }: AnimatedRouteProps) {
    const location = useLocation()
    const prefersReducedMotion = useReducedMotion()

    if (prefersReducedMotion) return <div className="h-full min-w-0">{children}</div>

    return (
        <motion.div
            key={location.pathname}
            className="h-full min-w-0"
            initial={{ opacity: 0.85, y: transitionType === "slide" ? 4 : 0 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.15, ease: "easeOut" }}
        >
            {children}
        </motion.div>
    )
}
