import { HTMLMotionProps } from "motion/react"

// 页面过渡动画配置 - 优化后
export const pageTransitionAnimation: HTMLMotionProps<"div"> = {
    className: "w-full h-full",
    initial: "initial",
    animate: "animate",
    exit: "exit",
    variants: {
        initial: {
            opacity: 0,
            x: 15,
            scale: 0.98
        },
        animate: {
            opacity: 1,
            x: 0,
            scale: 1,
            transition: {
                opacity: { duration: 0.3, ease: [0.22, 1, 0.36, 1] },
                x: { duration: 0.35, ease: [0.32, 0.72, 0, 1] },
                scale: { duration: 0.35, ease: [0.34, 1.56, 0.64, 1] }
            }
        },
        exit: {
            opacity: 0,
            x: -15,
            scale: 0.96,
            transition: {
                opacity: { duration: 0.2, ease: "easeOut" },
                x: { duration: 0.25, ease: "easeInOut" },
                scale: { duration: 0.2, ease: "easeIn" }
            }
        }
    }
}

// 页面淡入淡出过渡 - 优化后
export const pageFadeTransition: HTMLMotionProps<"div"> = {
    className: "w-full h-full",
    initial: { opacity: 0, scale: 0.985 },
    animate: {
        opacity: 1,
        scale: 1,
        transition: {
            opacity: { duration: 0.35 },
            scale: { duration: 0.25, ease: [0.34, 1.56, 0.64, 1] }
        }
    },
    exit: {
        opacity: 0,
        scale: 0.985,
        transition: {
            opacity: { duration: 0.25 },
            scale: { duration: 0.2, ease: "easeOut" }
        }
    }
}

// 页面滑动过渡 - 优化后
export const pageSlideTransition: HTMLMotionProps<"div"> = {
    className: "w-full h-full",
    initial: { x: 25, opacity: 0, filter: "blur(2px)" },
    animate: {
        x: 0,
        opacity: 1,
        filter: "blur(0px)",
        transition: {
            x: { duration: 0.4, ease: [0.22, 1, 0.36, 1] },
            opacity: { duration: 0.3, ease: "easeOut" },
            filter: { duration: 0.2, ease: "easeOut", delay: 0.1 }
        }
    },
    exit: {
        x: -20,
        opacity: 0,
        filter: "blur(2px)",
        transition: {
            x: { duration: 0.3, ease: "easeInOut" },
            opacity: { duration: 0.25, ease: "easeIn" },
            filter: { duration: 0.15, ease: "easeIn" }
        }
    }
}

// 新增：弹性缩放过渡
export const pageScaleTransition: HTMLMotionProps<"div"> = {
    className: "w-full h-full",
    initial: {
        opacity: 0,
        scale: 0.92,
        y: 10
    },
    animate: {
        opacity: 1,
        scale: 1,
        y: 0,
        transition: {
            duration: 0.4,
            scale: { type: "spring", stiffness: 120, damping: 20 },
            opacity: { duration: 0.3 },
            y: { duration: 0.3, ease: [0.22, 1, 0.36, 1] }
        }
    },
    exit: {
        opacity: 0,
        scale: 0.96,
        y: -8,
        transition: {
            duration: 0.25,
            ease: "easeOut"
        }
    }
}

// 新增：3D卡片翻转效果
export const pageFlipTransition: HTMLMotionProps<"div"> = {
    className: "w-full h-full",
    initial: {
        opacity: 0,
        rotateX: 8,
        y: 20,
        transformPerspective: 1200
    },
    animate: {
        opacity: 1,
        rotateX: 0,
        y: 0,
        transition: {
            duration: 0.5,
            rotateX: { duration: 0.5, ease: [0.2, 0.65, 0.3, 0.9] },
            y: { duration: 0.45, ease: [0.22, 1, 0.36, 1] },
            opacity: { duration: 0.4 }
        }
    },
    exit: {
        opacity: 0,
        rotateX: -8,
        y: -20,
        transition: {
            duration: 0.3,
            ease: "easeInOut"
        }
    }
} 