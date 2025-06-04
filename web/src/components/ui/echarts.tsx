import React, { useRef, useEffect, useMemo } from 'react'
import { init, getInstanceByDom, type EChartsOption, type ECharts } from 'echarts'
import { cn } from '@/lib/utils'

export interface EChartProps {
    option: EChartsOption
    style?: React.CSSProperties
    className?: string
    theme?: string | object
    onChartReady?: (chart: ECharts) => void
    onClick?: (params: unknown) => void
}

export const EChart: React.FC<EChartProps> = ({
    option,
    style = { width: '100%', height: '350px' },
    className,
    theme,
    onChartReady,
    onClick,
}) => {
    const chartRef = useRef<HTMLDivElement>(null)

    // 防抖的 resize 函数
    const resizeChart = useMemo(() => {
        let timeout: NodeJS.Timeout
        return () => {
            clearTimeout(timeout)
            timeout = setTimeout(() => {
                if (chartRef.current) {
                    const chart = getInstanceByDom(chartRef.current)
                    chart?.resize()
                }
            }, 300)
        }
    }, [])

    useEffect(() => {
        if (!chartRef.current) return

        // 初始化图表
        const chart = init(chartRef.current, theme)

        // 设置点击事件
        if (onClick) {
            chart.on('click', onClick)
        }

        // 监听窗口 resize 事件
        const handleResize = () => resizeChart()
        window.addEventListener('resize', handleResize)

        // 使用 ResizeObserver 监听容器大小变化
        const resizeObserver = new ResizeObserver(() => {
            resizeChart()
        })
        resizeObserver.observe(chartRef.current)

        // 图表准备完成回调
        if (onChartReady) {
            onChartReady(chart)
        }

        // 清理函数
        return () => {
            chart?.dispose()
            window.removeEventListener('resize', handleResize)
            if (chartRef.current) {
                resizeObserver.unobserve(chartRef.current)
            }
            resizeObserver.disconnect()
        }
    }, [theme, onChartReady, onClick, resizeChart])

    useEffect(() => {
        // 更新图表配置
        if (!chartRef.current) return

        const chart = getInstanceByDom(chartRef.current)
        if (chart && option) {
            chart.setOption(option, true)
        }
    }, [option])

    return (
        <div 
            ref={chartRef} 
            style={style} 
            className={cn("w-full", className)}
        />
    )
} 