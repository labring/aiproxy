import { ReactNode, useRef, useEffect, useState } from "react"
import { motion, useScroll, useTransform } from "motion/react"
import { containerAnimation } from "../container-animation"

interface TableScrollContainerProps {
    children: ReactNode
    className?: string
    showShadows?: boolean
    shadowOpacity?: number
}

export function TableScrollContainer({
    children,
    className = "",
    showShadows = true,
    shadowOpacity = 0.15
}: TableScrollContainerProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const { scrollYProgress } = useScroll({ container: containerRef })
    const [canScroll, setCanScroll] = useState(false)

    // 顶部和底部阴影透明度
    const topShadowOpacity = useTransform(scrollYProgress, [0, 0.1], [0, shadowOpacity])
    const bottomShadowOpacity = useTransform(scrollYProgress, [0.9, 1], [shadowOpacity, 0])

    // 检查内容是否可滚动
    useEffect(() => {
        const checkScrollability = () => {
            if (containerRef.current) {
                const { scrollHeight, clientHeight } = containerRef.current
                setCanScroll(scrollHeight > clientHeight)
            }
        }

        checkScrollability()

        // 添加窗口大小变化的监听
        window.addEventListener('resize', checkScrollability)
        return () => window.removeEventListener('resize', checkScrollability)
    }, [])

    return (
        <div className={`relative w-full h-full ${className}`}>
            {/* 顶部滚动阴影 */}
            {showShadows && canScroll && (
                <motion.div
                    className="absolute top-0 left-0 right-0 h-4 pointer-events-none z-10"
                    style={{
                        opacity: topShadowOpacity,
                        background: `linear-gradient(to bottom, rgba(0,0,0,${shadowOpacity}), transparent)`
                    }}
                />
            )}

            {/* 滚动容器 */}
            <motion.div
                ref={containerRef}
                className="overflow-auto h-full w-full scroll-smooth"
                style={{
                    // 添加平滑滚动效果
                    scrollBehavior: 'smooth',
                    // 优化移动端滚动
                    WebkitOverflowScrolling: 'touch'
                }}
                {...containerAnimation}
            >
                {children}
            </motion.div>

            {/* 底部滚动阴影 */}
            {showShadows && canScroll && (
                <motion.div
                    className="absolute bottom-0 left-0 right-0 h-4 pointer-events-none z-10"
                    style={{
                        opacity: bottomShadowOpacity,
                        background: `linear-gradient(to top, rgba(0,0,0,${shadowOpacity}), transparent)`
                    }}
                />
            )}
        </div>
    )
} 