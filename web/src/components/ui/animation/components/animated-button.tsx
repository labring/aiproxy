// src/components/ui/animation/components/animated-button.tsx
import { motion } from "motion/react"
import React, { forwardRef } from "react"
import {
    buttonAnimation,
    primaryButtonAnimation,
    secondaryButtonAnimation,
    ghostButtonAnimation,
    destructiveButtonAnimation
} from "../button-animation"

interface AnimatedButtonProps {
    children: React.ReactNode
    className?: string
    animationVariant?: "default" | "primary" | "secondary" | "ghost" | "destructive"
}

export const AnimatedButton = forwardRef<HTMLDivElement, AnimatedButtonProps>(
    ({
        children,
        className = "",
        animationVariant = "default",
        ...props
    }, ref) => {
        // 根据动画变体选择动画属性
        const getAnimationProps = () => {
            switch (animationVariant) {
                case "primary":
                    return primaryButtonAnimation
                case "secondary":
                    return secondaryButtonAnimation
                case "ghost":
                    return ghostButtonAnimation
                case "destructive":
                    return destructiveButtonAnimation
                case "default":
                default:
                    return buttonAnimation
            }
        }

        return (
            <motion.div
                ref={ref}
                className={className}
                {...getAnimationProps()}
                {...props}
            >
                {children}
            </motion.div>
        )
    }
)

AnimatedButton.displayName = "AnimatedButton"