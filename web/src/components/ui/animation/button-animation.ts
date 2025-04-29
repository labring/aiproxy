// src/components/ui/animation/button-animation.ts
import { HTMLMotionProps } from "motion/react"

// 标准按钮动画 - 流畅的序列缩放效果
export const buttonAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: [null, 1.05, 1.02],
        transition: {
            duration: 0.4,
            times: [0, 0.6, 1],
            ease: ["easeInOut", "easeOut"],
        },
    },
    whileTap: {
        scale: 0.95,
        transition: {
            duration: 0.1,
            ease: "easeIn",
        }
    },
    // 添加触摸板轻触反馈
    transition: {
        duration: 0.3,
        ease: "easeOut",
        type: "spring", // 使用弹簧动画提高触摸反馈
        stiffness: 400,
        damping: 17
    }
}

// 主要按钮动画 - 带有阴影增强的效果
export const primaryButtonAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: [null, 1.08, 1.03],
        boxShadow: "0 4px 12px rgba(0, 0, 0, 0.15)",
        transition: {
            duration: 0.5,
            times: [0, 0.6, 1],
            ease: ["easeInOut", "easeOut"],
        },
    },
    whileTap: {
        scale: 0.95,
        boxShadow: "0 2px 6px rgba(0, 0, 0, 0.1)",
        transition: {
            duration: 0.1,
            ease: "easeIn",
            // 触摸板优化参数
            type: "spring",
            stiffness: 600,
            damping: 25
        }
    },
    transition: {
        duration: 0.3,
        ease: "easeOut",
        type: "spring",
        stiffness: 400,
        damping: 17
    }
}

// 次要按钮动画 - 更微妙的效果
export const secondaryButtonAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: [null, 1.04, 1.02],
        transition: {
            duration: 0.3,
            times: [0, 0.5, 1],
            ease: ["easeInOut", "easeOut"],
        },
    },
    whileTap: {
        scale: 0.97,
        transition: {
            duration: 0.1,
            ease: "easeIn",
            // 为触摸板响应优化
            type: "spring",
            stiffness: 500,
            damping: 20
        }
    },
    transition: {
        duration: 0.3,
        ease: "easeOut",
        type: "spring",
        stiffness: 350,
        damping: 15
    }
}

// 幽灵按钮动画 - 轻微效果
export const ghostButtonAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: 1.03,
        transition: {
            duration: 0.3,
            ease: "easeOut",
        },
    },
    whileTap: {
        scale: 0.98,
        transition: {
            duration: 0.1,
            ease: "easeIn",
            // 更敏感的轻触反馈
            type: "spring",
            stiffness: 450,
            damping: 15
        }
    },
    transition: {
        duration: 0.3,
        ease: "easeOut",
        type: "spring",
        stiffness: 300,
        damping: 12
    }
}

// 破坏性操作按钮动画 - 更强调的警告效果
export const destructiveButtonAnimation: HTMLMotionProps<"div"> = {
    whileHover: {
        scale: [null, 1.06, 1.03],
        transition: {
            duration: 0.4,
            times: [0, 0.5, 1],
            ease: ["easeInOut", "easeOut"],
        },
    },
    whileTap: {
        // 碎裂消失效果
        scale: [0.95, 0.75, 0],
        opacity: [1, 0.7, 0],
        x: [0, 5, -5], // 左右轻微抖动
        transition: {
            duration: 0.4,
            times: [0, 0.5, 1],
            ease: "easeOut",
        }
    },
    transition: {
        duration: 0.3,
        ease: "easeOut",
        type: "spring",
        stiffness: 450,
        damping: 18
    }
}
