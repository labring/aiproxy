import { AnimatePresence, motion } from "motion/react"
import React from "react"
import { collapseAnimation, collapseScaleAnimation, collapseLightAnimation } from "../collapse-animation"

interface CollapseProps {
    isOpen: boolean
    children: React.ReactNode
    className?: string
    animationType?: "default" | "scale" | "light"
    initial?: boolean
}

export function Collapse({
    isOpen,
    children,
    className = "",
    animationType = "default",
    initial = false
}: CollapseProps) {
    // Select the appropriate animation based on the animationType
    const getAnimationProps = () => {
        switch (animationType) {
            case "scale":
                return collapseScaleAnimation
            case "light":
                return collapseLightAnimation
            case "default":
            default:
                return collapseAnimation
        }
    }

    return (
        <AnimatePresence initial={initial}>
            {isOpen && (
                <motion.div
                    key="collapse-content"
                    className={className}
                    {...getAnimationProps()}
                >
                    {children}
                </motion.div>
            )}
        </AnimatePresence>
    )
}