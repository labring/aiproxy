import { useEffect, useRef, type CSSProperties } from 'react'
import {
    init,
    getInstanceByDom,
    use as registerCharts,
    type ECharts,
} from 'echarts/core'
import type { EChartsOption } from 'echarts'
import { LineChart } from 'echarts/charts'
import {
    GridComponent,
    TooltipComponent,
    LegendComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { cn } from '@/lib/utils'

registerCharts([
    LineChart,
    GridComponent,
    TooltipComponent,
    LegendComponent,
    CanvasRenderer,
])

export interface EChartProps {
    option: EChartsOption
    style?: CSSProperties
    className?: string
    theme?: string | object
    onChartReady?: (chart: ECharts) => void
    onClick?: (params: unknown) => void
}

export function EChart({
    option,
    style = { width: '100%', height: '350px' },
    className,
    theme,
    onChartReady,
    onClick,
}: EChartProps) {
    const chartRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        const element = chartRef.current
        if (!element) return
        const chart = init(element, theme)
        let frame = 0
        const observer = new ResizeObserver(() => {
            cancelAnimationFrame(frame)
            frame = requestAnimationFrame(() => chart.resize())
        })
        observer.observe(element)
        return () => {
            cancelAnimationFrame(frame)
            observer.disconnect()
            chart.dispose()
        }
    }, [theme])

    useEffect(() => {
        const chart = chartRef.current && getInstanceByDom(chartRef.current)
        chart?.setOption(option, { notMerge: true, lazyUpdate: true })
    }, [option, theme])

    useEffect(() => {
        const chart = chartRef.current && getInstanceByDom(chartRef.current)
        if (!chart) return
        onChartReady?.(chart)
        if (onClick) chart.on('click', onClick)
        return () => {
            if (onClick) chart.off('click', onClick)
        }
    }, [theme, onClick, onChartReady])

    return (
        <div
            ref={chartRef}
            style={style}
            className={cn('w-full min-w-0', className)}
        />
    )
}
