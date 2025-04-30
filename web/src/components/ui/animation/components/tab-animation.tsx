import { tabFadeAnimation, tabScaleAnimation, tabContentAnimation } from "../tab-animation"
import React from "react"
import { AnimatePresence, motion } from "motion/react"

interface TabsAnimationProviderProps {
    children: React.ReactNode
    currentView: string
    animationVariant?: "slide" | "fade" | "scale"
}

export function TabsAnimationProvider({
    children,
    currentView,
    animationVariant = "slide"
}: TabsAnimationProviderProps) {
    // 根据选择的变体选择相应的动画
    const getAnimationProps = () => {
        switch (animationVariant) {
            case "fade":
                return tabFadeAnimation
            case "scale":
                return tabScaleAnimation
            case "slide":
            default:
                return tabContentAnimation
        }
    }

    return (
        <AnimatePresence mode="wait">
            <motion.div
                key={currentView}
                {...getAnimationProps()}
            >
                {children}
            </motion.div>
        </AnimatePresence>
    )
}