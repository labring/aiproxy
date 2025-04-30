import { AnimatePresence, motion } from "motion/react"
import React from "react"
import { fadeAnimation, slideAnimation, scaleAnimation } from "../display-animation"

interface DisplayProps {
    visible: boolean
    children: React.ReactNode
    className?: string
    animationType?: "fade" | "slide" | "scale"
    initial?: boolean
    mode?: "sync" | "wait" | "popLayout"
}

export function Display({
    visible,
    children,
    className = "",
    animationType = "fade",
    initial = false,
    mode = "sync"
}: DisplayProps) {
    // 根据动画类型选择适当的动画属性
    const getAnimationProps = () => {
        switch (animationType) {
            case "slide":
                return slideAnimation
            case "scale":
                return scaleAnimation
            case "fade":
            default:
                return fadeAnimation
        }
    }

    return (
        <AnimatePresence initial={initial} mode={mode}>
            {visible && (
                <motion.div
                    key="display-content"
                    className={className}
                    {...getAnimationProps()}
                >
                    {children}
                </motion.div>
            )}
        </AnimatePresence>
    )
} 