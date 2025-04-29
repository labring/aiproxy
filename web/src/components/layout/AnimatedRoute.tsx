import { ReactNode } from "react"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { useLocation } from "react-router"
import {
    pageSlideTransition,
    pageFadeTransition,
    pageScaleTransition,
    pageFlipTransition
} from "@/components/ui/animation/route-animation"

interface AnimatedRouteProps {
    children: ReactNode
    transitionType?: "slide" | "fade" | "scale" | "flip"
}

export function AnimatedRoute({
    children,
    transitionType = "slide"
}: AnimatedRouteProps) {
    const location = useLocation()
    const prefersReducedMotion = useReducedMotion()

    // 如果用户设置了减少动画，则使用简单淡入淡出
    if (prefersReducedMotion) {
        return (
            <AnimatePresence mode="wait">
                <motion.div
                    key={location.pathname}
                    {...pageFadeTransition}
                >
                    {children}
                </motion.div>
            </AnimatePresence>
        )
    }

    // 根据传入的类型选择不同的动画效果
    const getTransitionProps = () => {
        switch (transitionType) {
            case "fade":
                return pageFadeTransition
            case "scale":
                return pageScaleTransition
            case "flip":
                return pageFlipTransition
            case "slide":
            default:
                return pageSlideTransition
        }
    }

    return (
        <AnimatePresence mode="wait">
            <motion.div
                key={location.pathname}
                {...getTransitionProps()}
            >
                {children}
            </motion.div>
        </AnimatePresence>
    )
} 