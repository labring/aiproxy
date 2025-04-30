import { HTMLMotionProps } from "motion/react"

// 对话框进入和退出的容器动画
export const dialogEnterExitAnimation: HTMLMotionProps<"div"> = {
    className: "fixed left-[50%] top-[50%] z-50 grid w-full translate-x-[-50%] translate-y-[-50%]",
    initial: "hidden",
    animate: "visible",
    exit: "exit",
    variants: {
        hidden: {
            opacity: 0,
            scale: 0.96,
            y: 8
        },
        visible: {
            opacity: 1,
            scale: 1,
            y: 0,
            transition: {
                duration: 0.3,
                ease: [0.16, 1, 0.3, 1], // custom ease curve for natural feel
                when: "beforeChildren"
            }
        },
        exit: {
            opacity: 0,
            scale: 0.96,
            y: -8,
            transition: {
                duration: 0.25,
                ease: [0.32, 0, 0.67, 0], // easeInCubic for natural exit
                when: "afterChildren"
            }
        }
    }
}

// DialogContent 内容区域动画 - 更自然的涌动效果
export const dialogContentAnimation: HTMLMotionProps<"div"> = {
    variants: {
        hidden: {
            opacity: 0,
        },
        visible: {
            opacity: 1,
            transition: {
                duration: 0.2,
                ease: "easeOut",
                staggerChildren: 0.06,
                delayChildren: 0.05
            }
        },
        exit: {
            opacity: 0,
            transition: {
                duration: 0.15,
                ease: "easeIn",
                staggerChildren: 0.03,
                staggerDirection: -1
            }
        }
    }
}

// 内容涌动的子元素动画 - 更自然的流动感
export const dialogContentItemAnimation: HTMLMotionProps<"div"> = {
    variants: {
        hidden: {
            opacity: 0,
            y: 12,
            scale: 0.98
        },
        visible: {
            opacity: 1,
            y: 0,
            scale: 1,
            transition: {
                type: "spring",
                damping: 20, // 更高的阻尼使动画更自然不过度弹跳
                stiffness: 260, // 适当的刚度平衡速度和流畅度
                mass: 0.4 // 较轻的质量使动画更灵活
            }
        },
        exit: {
            opacity: 0,
            y: -8,
            transition: {
                duration: 0.18,
                ease: [0.32, 0, 0.67, 0] // easeInCubic for natural exit
            }
        }
    }
}

// DialogHeader 标题区域动画
export const dialogHeaderAnimation: HTMLMotionProps<"div"> = {
    variants: {
        hidden: {
            opacity: 0,
            y: -10
        },
        visible: {
            opacity: 1,
            y: 0,
            transition: {
                duration: 0.3,
                ease: "easeOut"
            }
        },
        exit: {
            opacity: 0,
            y: -5,
            transition: {
                duration: 0.2,
                ease: "easeIn"
            }
        }
    }
} 