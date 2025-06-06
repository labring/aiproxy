import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { EChartsOption } from 'echarts'

import { EChart } from '@/components/ui/echarts'
import { Skeleton } from '@/components/ui/skeleton'
import { useTheme } from '@/handler/ThemeContext'
import { ChartDataPoint } from '@/types/dashboard'

interface MonitorChartsProps {
    chartData: ChartDataPoint[]
    loading?: boolean
}

export function MonitorCharts({ chartData, loading = false }: MonitorChartsProps) {
    const { t } = useTranslation()
    const { theme } = useTheme()

    // 清新现代的颜色配置 - 适配暗色模式
    const colorPalette = [
        '#3b82f6',  // 蓝色 - 缓存创建 Tokens
        '#8b5cf6',  // 紫色 - 缓存 Tokens  
        '#06b6d4',  // 青色 - 输入 Tokens
        '#10b981',  // 绿色 - 输出 Tokens
        '#f59e0b',  // 橙色 - 总 Tokens
        '#ec4899'   // 粉色 - 搜索次数
    ]

    // 检测暗色模式 - 基于主题设置或系统偏好
    const isDarkMode = useMemo(() => {
        if (theme === 'dark') return true
        if (theme === 'light') return false
        // 如果是 system，检查系统偏好
        return window.matchMedia('(prefers-color-scheme: dark)').matches
    }, [theme])

    // 根据主题模式调整颜色
    const getThemeColors = () => ({
        textColor: isDarkMode ? '#e5e7eb' : '#666',
        axisLineColor: isDarkMode ? '#374151' : '#e1e4e8',
        splitLineColor: isDarkMode ? '#374151' : '#f0f0f0',
        tooltipBg: isDarkMode ? 'rgba(31, 41, 55, 0.95)' : 'rgba(255, 255, 255, 0.95)',
        tooltipBorder: isDarkMode ? '#4b5563' : '#e1e4e8',
        tooltipTextColor: isDarkMode ? '#f3f4f6' : '#333',
        crossLabelBg: isDarkMode ? '#4b5563' : '#283042'
    })

    const themeColors = getThemeColors()

    // Tokens 相关图表配置
    const tokensChartOption: EChartsOption = useMemo(() => {
        const timestamps = chartData.map(item => new Date(item.timestamp * 1000).toLocaleString())

        return {
            backgroundColor: 'transparent',
            tooltip: {
                trigger: 'axis',
                axisPointer: {
                    type: 'cross',
                    label: {
                        backgroundColor: themeColors.crossLabelBg,
                        borderColor: themeColors.crossLabelBg,
                        borderWidth: 1,
                        borderRadius: 4,
                        color: '#fff'
                    },
                    crossStyle: {
                        color: themeColors.textColor
                    }
                },
                backgroundColor: themeColors.tooltipBg,
                borderColor: themeColors.tooltipBorder,
                borderWidth: 1,
                borderRadius: 8,
                textStyle: {
                    color: themeColors.tooltipTextColor,
                    fontSize: 12
                },
                extraCssText: `box-shadow: 0 4px 12px ${isDarkMode ? 'rgba(0, 0, 0, 0.3)' : 'rgba(0, 0, 0, 0.1)'};`
            },
            legend: {
                top: '10px',
                data: [
                    t('monitor.charts.tokensChart.cacheCreationTokens'),
                    t('monitor.charts.tokensChart.cachedTokens'),
                    t('monitor.charts.tokensChart.inputTokens'),
                    t('monitor.charts.tokensChart.outputTokens'),
                    t('monitor.charts.tokensChart.totalTokens'),
                    t('monitor.charts.tokensChart.webSearchCount')
                ],
                textStyle: {
                    color: themeColors.textColor,
                    fontSize: 12
                },
                itemGap: 20,
                icon: 'circle'
            },
            grid: {
                left: '12px',
                right: '12px',
                bottom: '3%',
                top: '50px',
                containLabel: true
            },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: timestamps,
                axisLine: {
                    lineStyle: {
                        color: themeColors.axisLineColor,
                        width: 1
                    }
                },
                axisLabel: {
                    color: themeColors.textColor,
                    fontSize: 11,
                    margin: 10
                },
                axisTick: {
                    show: false
                },
                splitLine: {
                    show: false
                }
            },
            yAxis: {
                type: 'value',
                axisLine: {
                    show: false
                },
                axisLabel: {
                    color: themeColors.textColor,
                    fontSize: 11,
                    margin: 10
                },
                axisTick: {
                    show: false
                },
                splitLine: {
                    lineStyle: {
                        color: themeColors.splitLineColor,
                        type: 'dashed',
                        width: 1
                    }
                }
            },
            series: [
                {
                    name: t('monitor.charts.tokensChart.cacheCreationTokens'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 5,
                    lineStyle: {
                        width: 2,
                        color: colorPalette[0],
                        shadowColor: `${colorPalette[0]}20`,
                        shadowBlur: 4,
                        shadowOffsetY: 1
                    },
                    itemStyle: {
                        color: colorPalette[0],
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 10,
                            shadowColor: `${colorPalette[0]}40`,
                            shadowOffsetY: 2
                        }
                    },
                    data: chartData.map(item => item.cache_creation_tokens)
                },
                {
                    name: t('monitor.charts.tokensChart.cachedTokens'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 5,
                    lineStyle: {
                        width: 2,
                        color: colorPalette[1],
                        shadowColor: `${colorPalette[1]}20`,
                        shadowBlur: 4,
                        shadowOffsetY: 1
                    },
                    itemStyle: {
                        color: colorPalette[1],
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 10,
                            shadowColor: `${colorPalette[1]}40`,
                            shadowOffsetY: 2
                        }
                    },
                    data: chartData.map(item => item.cached_tokens)
                },
                {
                    name: t('monitor.charts.tokensChart.inputTokens'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 5,
                    lineStyle: {
                        width: 2,
                        color: colorPalette[2],
                        shadowColor: `${colorPalette[2]}20`,
                        shadowBlur: 4,
                        shadowOffsetY: 1
                    },
                    itemStyle: {
                        color: colorPalette[2],
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 10,
                            shadowColor: `${colorPalette[2]}40`,
                            shadowOffsetY: 2
                        }
                    },
                    data: chartData.map(item => item.input_tokens)
                },
                {
                    name: t('monitor.charts.tokensChart.outputTokens'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 5,
                    lineStyle: {
                        width: 2,
                        color: colorPalette[3],
                        shadowColor: `${colorPalette[3]}20`,
                        shadowBlur: 4,
                        shadowOffsetY: 1
                    },
                    itemStyle: {
                        color: colorPalette[3],
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 10,
                            shadowColor: `${colorPalette[3]}40`,
                            shadowOffsetY: 2
                        }
                    },
                    data: chartData.map(item => item.output_tokens)
                },
                {
                    name: t('monitor.charts.tokensChart.totalTokens'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 5,
                    lineStyle: {
                        width: 2,
                        color: colorPalette[4],
                        shadowColor: `${colorPalette[4]}20`,
                        shadowBlur: 4,
                        shadowOffsetY: 1
                    },
                    itemStyle: {
                        color: colorPalette[4],
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 10,
                            shadowColor: `${colorPalette[4]}40`,
                            shadowOffsetY: 2
                        }
                    },
                    data: chartData.map(item => item.total_tokens)
                },
                {
                    name: t('monitor.charts.tokensChart.webSearchCount'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 5,
                    lineStyle: {
                        width: 2,
                        color: colorPalette[5],
                        shadowColor: `${colorPalette[5]}20`,
                        shadowBlur: 4,
                        shadowOffsetY: 1
                    },
                    itemStyle: {
                        color: colorPalette[5],
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 10,
                            shadowColor: `${colorPalette[5]}40`,
                            shadowOffsetY: 2
                        }
                    },
                    data: chartData.map(item => item.web_search_count)
                }
            ],
            animation: true,
            animationDuration: 1000,
            animationEasing: 'cubicOut'
        }
    }, [chartData, t, isDarkMode, themeColors])

    // 请求和错误图表配置
    const requestsChartOption: EChartsOption = useMemo(() => {
        const timestamps = chartData.map(item => new Date(item.timestamp * 1000).toLocaleString())

        return {
            backgroundColor: 'transparent',
            tooltip: {
                trigger: 'axis',
                axisPointer: {
                    type: 'cross',
                    label: {
                        backgroundColor: themeColors.crossLabelBg,
                        borderColor: themeColors.crossLabelBg,
                        borderWidth: 1,
                        borderRadius: 4,
                        color: '#fff'
                    },
                    crossStyle: {
                        color: themeColors.textColor
                    }
                },
                backgroundColor: themeColors.tooltipBg,
                borderColor: themeColors.tooltipBorder,
                borderWidth: 1,
                borderRadius: 8,
                textStyle: {
                    color: themeColors.tooltipTextColor,
                    fontSize: 12
                },
                extraCssText: `box-shadow: 0 4px 12px ${isDarkMode ? 'rgba(0, 0, 0, 0.3)' : 'rgba(0, 0, 0, 0.1)'};`
            },
            legend: {
                top: '10px',
                data: [
                    t('monitor.charts.requestsChart.requestCount'),
                    t('monitor.charts.requestsChart.exceptionCount')
                ],
                textStyle: {
                    color: themeColors.textColor,
                    fontSize: 12
                },
                itemGap: 20,
                icon: 'circle'
            },
            grid: {
                left: '12px',
                right: '12px',
                bottom: '3%',
                top: '50px',
                containLabel: true
            },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: timestamps,
                axisLine: {
                    lineStyle: {
                        color: themeColors.axisLineColor,
                        width: 1
                    }
                },
                axisLabel: {
                    color: themeColors.textColor,
                    fontSize: 11,
                    margin: 10
                },
                axisTick: {
                    show: false
                },
                splitLine: {
                    show: false
                }
            },
            yAxis: {
                type: 'value',
                axisLine: {
                    show: false
                },
                axisLabel: {
                    color: themeColors.textColor,
                    fontSize: 11,
                    margin: 10
                },
                axisTick: {
                    show: false
                },
                splitLine: {
                    lineStyle: {
                        color: themeColors.splitLineColor,
                        type: 'dashed',
                        width: 1
                    }
                }
            },
            series: [
                {
                    name: t('monitor.charts.requestsChart.requestCount'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 6,
                    lineStyle: {
                        width: 2.5,
                        color: '#3b82f6',
                        shadowColor: '#3b82f620',
                        shadowBlur: 6,
                        shadowOffsetY: 2
                    },
                    itemStyle: {
                        color: '#3b82f6',
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 12,
                            shadowColor: '#3b82f640',
                            shadowOffsetY: 3
                        }
                    },
                    data: chartData.map(item => item.request_count)
                },
                {
                    name: t('monitor.charts.requestsChart.exceptionCount'),
                    type: 'line',
                    smooth: true,
                    showSymbol: false,
                    symbolSize: 6,
                    lineStyle: {
                        width: 2.5,
                        color: '#ef4444',
                        shadowColor: '#ef444420',
                        shadowBlur: 6,
                        shadowOffsetY: 2
                    },
                    itemStyle: {
                        color: '#ef4444',
                        borderWidth: 2,
                        borderColor: isDarkMode ? '#1f2937' : '#fff'
                    },
                    emphasis: {
                        focus: 'series',
                        scale: true,
                        itemStyle: {
                            shadowBlur: 12,
                            shadowColor: '#ef444440',
                            shadowOffsetY: 3
                        }
                    },
                    data: chartData.map(item => item.exception_count)
                }
            ],
            animation: true,
            animationDuration: 1000,
            animationEasing: 'cubicOut'
        }
    }, [chartData, t, isDarkMode, themeColors])

    if (loading) {
        return (
            <div className="flex flex-col gap-4 h-[calc(100vh-280px)]">
                <div className="flex-1">
                    <Skeleton className="w-full h-full rounded-lg" />
                </div>
                <div className="flex-1">
                    <Skeleton className="w-full h-full rounded-lg" />
                </div>
            </div>
        )
    }

    return (
        <div className="flex flex-col gap-4 h-[calc(100vh-280px)]">
            <div className="flex-1">
                <EChart
                    option={tokensChartOption}
                    style={{ width: '100%', height: '100%' }}
                />
            </div>
            <div className="flex-1">
                <EChart
                    option={requestsChartOption}
                    style={{ width: '100%', height: '100%' }}
                />
            </div>
        </div>
    )
} 