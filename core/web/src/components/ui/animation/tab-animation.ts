// src/components/ui/animations/tabs-animations.tsx
import { HTMLMotionProps } from "motion/react"

// 标签内容切换动画 - 水平滑动效果
export const tabContentAnimation: HTMLMotionProps<"div"> = {
    initial: {
        opacity: 0,
        x: 10
    },
    animate: {
        opacity: 1,
        x: 0,
        transition: {
            duration: 0.3,
            ease: [0.22, 1, 0.36, 1]
        }
    },
    exit: {
        opacity: 0,
        x: -10,
        transition: {
            duration: 0.2
        }
    }
}

// 标签内容淡入淡出动画 - 没有位移只有透明度变化
export const tabFadeAnimation: HTMLMotionProps<"div"> = {
    initial: {
        opacity: 0
    },
    animate: {
        opacity: 1,
        transition: {
            duration: 0.25
        }
    },
    exit: {
        opacity: 0,
        transition: {
            duration: 0.2
        }
    }
}

// 标签内容缩放动画 - 带有轻微的缩放效果
export const tabScaleAnimation: HTMLMotionProps<"div"> = {
    initial: {
        opacity: 0,
        scale: 0.98
    },
    animate: {
        opacity: 1,
        scale: 1,
        transition: {
            duration: 0.25,
            ease: [0.25, 1, 0.5, 1]
        }
    },
    exit: {
        opacity: 0,
        scale: 0.98,
        transition: {
            duration: 0.2
        }
    }
}