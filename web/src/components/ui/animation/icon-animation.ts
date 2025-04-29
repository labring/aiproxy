// src/components/ui/animation/icon-animation.ts
import { HTMLMotionProps } from "motion/react"
// 旋转动画 - 适用于刷新、重置图标
export const spinIconAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: 1.05,
        transition: {
            type: "spring",
            stiffness: 400,
            damping: 10
        }
    },
    whileTap: {
        rotate: 360,
        scale: 0.95,
        transition: {
            duration: 0.5,
            ease: "easeInOut",
            scale: {
                type: "spring",
                stiffness: 500,
                damping: 15
            }
        }
    },
    transition: {
        duration: 0.3,
        ease: "easeOut",
        type: "spring",
    }
}

// 持续旋转动画 - 适用于加载状态
export const continuousSpinAnimation: HTMLMotionProps<"div"> = {
    animate: {
        rotate: 360,
        transition: {
            duration: 1.5,
            ease: "linear",
            repeat: Infinity
        }
    },
    // 点击时可暂时加快旋转速度
    whileTap: {
        scale: 0.95,
        transition: {
            type: "spring",
            stiffness: 400,
            damping: 10
        }
    }
}

// 震动动画 - 适用于通知、警告图标
export const shakeIconAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: 1.05,
        transition: {
            type: "spring",
            stiffness: 400,
            damping: 10
        }
    },
    whileTap: {
        rotate: [0, -12, 10, -6, 3, -2, 0],
        scale: 0.95,
        transition: {
            duration: 0.7,
            times: [0, 0.25, 0.5, 0.75, 0.85, 0.92, 1],
            ease: "easeOut",
            scale: {
                type: "spring",
                stiffness: 500,
                damping: 15,
                duration: 0.1
            }
        }
    },
    transition: {
        type: "spring",
        stiffness: 350,
        damping: 15
    }
}

// 弹跳动画 - 适用于交互按钮
export const bounceIconAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: 1.05,
        y: -2,
        transition: {
            type: "spring",
            stiffness: 400,
            damping: 10
        }
    },
    whileTap: {
        y: [0, -8, 4, -2, 0],
        scale: 0.95,
        transition: {
            duration: 0.5,
            times: [0, 0.3, 0.6, 0.8, 1],
            ease: "easeOut",
            scale: {
                type: "spring",
                stiffness: 500,
                damping: 15,
                duration: 0.1
            }
        }
    },
    transition: {
        type: "spring",
        stiffness: 400,
        damping: 12
    }
}

// 脉冲动画 - 适用于强调图标
export const pulseIconAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: 1.05,
        transition: {
            type: "spring",
            stiffness: 400,
            damping: 10
        }
    },
    whileTap: {
        scale: [1, 1.3, 0.95, 1.05, 1],
        transition: {
            duration: 0.5,
            times: [0, 0.2, 0.4, 0.6, 1],
            ease: "easeOut"
        }
    },
    transition: {
        type: "spring",
        stiffness: 380,
        damping: 15
    }
}

// 新增：通知点亮动画 - 轻触时有发光效果
export const glowIconAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: 1.08,
        filter: "drop-shadow(0 0 3px rgba(255, 255, 255, 0.7))",
        transition: {
            type: "spring",
            stiffness: 400,
            damping: 10
        }
    },
    whileTap: {
        scale: 0.92,
        filter: "drop-shadow(0 0 6px rgba(255, 255, 255, 0.9))",
        transition: {
            type: "spring",
            stiffness: 500,
            damping: 12,
            duration: 0.2
        }
    },
    transition: {
        type: "spring",
        stiffness: 400,
        damping: 15
    }
}