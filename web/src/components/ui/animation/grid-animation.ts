import { HTMLMotionProps } from "motion/react"

// 布局动画配置 - 用于处理元素添加、删除和重排序的动画效果
export const layoutAnimationProps = {
    layout: true,
    initial: { opacity: 0, scale: 0.8 },
    animate: { opacity: 1, scale: 1 },
    exit: { opacity: 0, scale: 0.8 },
    transition: {
        layout: {
            type: "spring",
            damping: 25,
            stiffness: 300,
            mass: 0.8
        },
        opacity: { duration: 0.3 },
        scale: {
            type: "spring",
            damping: 15,
            stiffness: 200
        }
    }
}

// 保留网格项目的动画配置，因为它在SiteCard中使用
export const gridItemAnimation: HTMLMotionProps<"div"> = {
    variants: {
        initial: {
            opacity: 0,
            y: 20,
            scale: 0.95
        },
        animate: {
            opacity: 1,
            y: 0,
            scale: 1,
            transition: {
                type: "spring",
                damping: 15,
                stiffness: 200
            }
        }
    }
}