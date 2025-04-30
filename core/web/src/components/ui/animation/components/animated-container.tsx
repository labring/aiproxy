import { motion } from "motion/react"
import React, { forwardRef } from "react"
import {
    containerAnimation,
    smoothContainerAnimation,
    scrollContainerAnimation,
    layoutRootAnimation
} from "../container-animation"

interface AnimatedContainerProps {
    children: React.ReactNode
    className?: string
    variant?: "default" | "smooth" | "scroll" | "root"
    layoutId?: string
    layoutDependency?: unknown
}

export const AnimatedContainer = forwardRef<HTMLDivElement, AnimatedContainerProps>(
    ({
        children,
        className = "",
        variant = "default",
        layoutId,
        layoutDependency,
        ...props
    }, ref) => {
        // 根据变体选择动画属性
        const getAnimationProps = () => {
            switch (variant) {
                case "smooth":
                    return smoothContainerAnimation
                case "scroll":
                    return scrollContainerAnimation
                case "root":
                    return layoutRootAnimation
                case "default":
                default:
                    return containerAnimation
            }
        }

        return (
            <motion.div
                ref={ref}
                className={className}
                layoutId={layoutId}
                layoutDependency={layoutDependency}
                {...getAnimationProps()}
                {...props}
            >
                {children}
            </motion.div>
        )
    }
)

AnimatedContainer.displayName = "AnimatedContainer" 