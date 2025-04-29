import { HTMLMotionProps } from "motion/react"

// 基础容器动画 - 使用弹性动画效果
export const containerAnimation: HTMLMotionProps<"div"> = {
    layout: true,
    transition: {
        layout: {
            type: "spring",
            stiffness: 300,
            damping: 30
        }
    }
}

// 平滑容器动画 - 使用补间动画，更均匀的过渡
export const smoothContainerAnimation: HTMLMotionProps<"div"> = {
    layout: true,
    transition: {
        layout: {
            duration: 0.4,
            ease: [0.4, 0, 0.2, 1],
            type: "tween"
        }
    }
}

// 优化的滚动容器动画 - 减少过渡时间，提高响应性
export const scrollContainerAnimation: HTMLMotionProps<"div"> = {
    layout: true,
    layoutRoot: true,
    transition: {
        layout: {
            duration: 0.25,
            ease: [0.25, 0.1, 0.25, 1.0],
            type: "tween"
        }
    }
}

// 高性能布局根动画 - 适用于复杂列表和表格
export const layoutRootAnimation: HTMLMotionProps<"div"> = {
    layout: true,
    layoutRoot: true,
    transition: {
        layout: {
            type: "tween",
            duration: 0.2
        }
    }
} 