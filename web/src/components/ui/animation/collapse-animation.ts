// src/components/ui/animations/collapse-animations.tsx
import { HTMLMotionProps } from "motion/react"

// 优化基础折叠/展开动画，避免布局跳动
export const collapseAnimation: HTMLMotionProps<"div"> = {
    initial: "collapsed",
    animate: "open",
    exit: "collapsed",
    variants: {
        open: {
            height: "auto",
            opacity: 1,
            marginBottom: 16, // 增加展开状态下的底部边距以预留空间
            transition: {
                height: {
                    duration: 0.35, // 略微增加持续时间使过渡更平滑
                    ease: [0.33, 1, 0.68, 1]
                },
                opacity: {
                    duration: 0.25,
                    delay: 0.1 // 增加不透明度变化的延迟
                },
                marginBottom: {
                    duration: 0.35,
                    ease: [0.33, 1, 0.68, 1]
                }
            }
        },
        collapsed: {
            height: 0,
            opacity: 0,
            marginBottom: 0,
            transition: {
                height: {
                    duration: 0.3,
                    ease: [0.33, 1, 0.68, 1]
                },
                opacity: {
                    duration: 0.2,
                    ease: "easeIn"
                },
                marginBottom: {
                    duration: 0.3,
                    ease: [0.33, 1, 0.68, 1]
                }
            }
        }
    },
    style: {
        overflow: "hidden",
        position: "relative",
        willChange: "height, opacity, margin",
        transformOrigin: "top center", // 确保任何变换都从顶部开始
        minHeight: 0 // 确保折叠时可以完全收起
    }
}

// 带缩放效果的折叠动画，优化以减少视觉跳动
export const collapseScaleAnimation: HTMLMotionProps<"div"> = {
    initial: "collapsed",
    animate: "open",
    exit: "collapsed",
    variants: {
        open: {
            height: "auto",
            opacity: 1,
            scale: 1,
            marginBottom: 16, // 添加展开状态的边距
            transformOrigin: "top center",
            y: 0, // 保持垂直位置不变
            transition: {
                height: {
                    duration: 0.35,
                    ease: [0.33, 1, 0.68, 1]
                },
                scale: {
                    duration: 0.25,
                    delay: 0.05
                },
                opacity: {
                    duration: 0.25,
                    delay: 0.1
                },
                marginBottom: {
                    duration: 0.35
                },
                y: {
                    duration: 0.25
                }
            }
        },
        collapsed: {
            height: 0,
            opacity: 0,
            scale: 0.95, // 稍微调整以减少视觉跳动
            marginBottom: 0,
            transformOrigin: "top center",
            y: -5, // 轻微向上位移以更自然地消失
            transition: {
                height: {
                    duration: 0.3,
                    ease: [0.33, 1, 0.68, 1]
                },
                scale: {
                    duration: 0.25
                },
                opacity: {
                    duration: 0.2
                },
                marginBottom: {
                    duration: 0.3
                },
                y: {
                    duration: 0.2
                }
            }
        }
    },
    style: {
        overflow: "hidden",
        position: "relative",
        willChange: "height, opacity, margin, transform",
        minHeight: 0,
        isolation: "isolate" // 创建新的层叠上下文，减少重绘区域
    }
}

// 轻度动效的展开动画，优化以减少视觉跳动
export const collapseLightAnimation: HTMLMotionProps<"div"> = {
    initial: "collapsed",
    animate: "open",
    exit: "collapsed",
    variants: {
        open: {
            height: "auto",
            opacity: 1,
            marginBottom: 12, // 提供适当的边距
            y: 0,
            transition: {
                height: {
                    duration: 0.25,
                    ease: [0.33, 1, 0.68, 1]
                },
                opacity: {
                    duration: 0.2,
                    delay: 0.05
                },
                marginBottom: {
                    duration: 0.25
                },
                y: {
                    duration: 0.2
                }
            }
        },
        collapsed: {
            height: 0,
            opacity: 0,
            marginBottom: 0,
            y: -3, // 轻微向上位移
            transition: {
                height: {
                    duration: 0.25,
                    ease: [0.33, 1, 0.68, 1]
                },
                opacity: {
                    duration: 0.15
                },
                marginBottom: {
                    duration: 0.25
                },
                y: {
                    duration: 0.15
                }
            }
        }
    },
    style: {
        overflow: "hidden",
        position: "relative",
        willChange: "height, opacity, margin, transform",
        minHeight: 0,
        transformOrigin: "top center"
    }
}

// 新增一个用于列表项的动画，特别适合添加/删除服务器的场景
export const listItemAnimation: HTMLMotionProps<"div"> = {
    initial: "collapsed",
    animate: "open",
    exit: "collapsed",
    layout: true, // 启用布局动画
    variants: {
        open: {
            height: "auto",
            opacity: 1,
            scale: 1,
            y: 0,
            marginBottom: 16,
            transition: {
                height: {
                    duration: 0.35,
                    ease: [0.33, 1, 0.68, 1]
                },
                opacity: {
                    duration: 0.25,
                    delay: 0.05
                },
                scale: {
                    duration: 0.25,
                    delay: 0.05
                },
                y: {
                    duration: 0.25
                },
                marginBottom: {
                    duration: 0.35
                }
            }
        },
        collapsed: {
            height: 0,
            opacity: 0,
            scale: 0.98,
            y: -10,
            marginBottom: 0,
            transition: {
                height: {
                    duration: 0.3,
                    ease: [0.33, 1, 0.68, 1]
                },
                opacity: {
                    duration: 0.2
                },
                scale: {
                    duration: 0.2
                },
                y: {
                    duration: 0.2
                },
                marginBottom: {
                    duration: 0.3
                }
            }
        }
    },
    style: {
        overflow: "hidden",
        position: "relative",
        willChange: "height, opacity, margin, transform",
        minHeight: 0,
        transformOrigin: "top center",
        zIndex: 1
    }
}