import React, { useMemo } from 'react'
import type { EChartsOption } from 'echarts'

import { EChart } from '@/components/ui/echarts'
import { ChartDataPoint } from '@/types/dashboard'

interface MonitorChartsProps {
    chartData: ChartDataPoint[]
    loading?: boolean
}

export function MonitorCharts({ chartData, loading = false }: MonitorChartsProps) {
    // 清新现代的颜色配置
    const colorPalette = [
        '#3b82f6',  // 蓝色 - 缓存创建 Tokens
        '#8b5cf6',  // 紫色 - 缓存 Tokens  
        '#06b6d4',  // 青色 - 输入 Tokens
        '#10b981',  // 绿色 - 输出 Tokens
        '#f59e0b',  // 橙色 - 总 Tokens
        '#ec4899'   // 粉色 - 搜索次数
    ]

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
                        backgroundColor: '#283042',
                        borderColor: '#283042',
                        borderWidth: 1,
                        borderRadius: 4,
                        color: '#fff'
                    },
                    crossStyle: {
                        color: '#999'
                    }
                },
                backgroundColor: 'rgba(255, 255, 255, 0.95)',
                borderColor: '#e1e4e8',
                borderWidth: 1,
                borderRadius: 8,
                textStyle: {
                    color: '#333',
                    fontSize: 12
                },
                extraCssText: 'box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);'
            },
            legend: {
                top: '10px',
                data: ['缓存创建 Tokens', '缓存 Tokens', '输入 Tokens', '输出 Tokens', '总 Tokens', '搜索次数'],
                textStyle: {
                    color: '#666',
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
                        color: '#e1e4e8',
                        width: 1
                    }
                },
                axisLabel: {
                    color: '#666',
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
                    color: '#666',
                    fontSize: 11,
                    margin: 10
                },
                axisTick: {
                    show: false
                },
                splitLine: {
                    lineStyle: {
                        color: '#f0f0f0',
                        type: 'dashed',
                        width: 1
                    }
                }
            },
            series: [
                {
                    name: '缓存创建 Tokens',
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
                        borderColor: '#fff'
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
                    name: '缓存 Tokens',
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
                        borderColor: '#fff'
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
                    name: '输入 Tokens',
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
                        borderColor: '#fff'
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
                    name: '输出 Tokens',
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
                        borderColor: '#fff'
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
                    name: '总 Tokens',
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
                        borderColor: '#fff'
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
                    name: '搜索次数',
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
                        borderColor: '#fff'
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
    }, [chartData])

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
                        backgroundColor: '#283042',
                        borderColor: '#283042',
                        borderWidth: 1,
                        borderRadius: 4,
                        color: '#fff'
                    },
                    crossStyle: {
                        color: '#999'
                    }
                },
                backgroundColor: 'rgba(255, 255, 255, 0.95)',
                borderColor: '#e1e4e8',
                borderWidth: 1,
                borderRadius: 8,
                textStyle: {
                    color: '#333',
                    fontSize: 12
                },
                extraCssText: 'box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);'
            },
            legend: {
                top: '10px',
                data: ['请求数量', '异常数量'],
                textStyle: {
                    color: '#666',
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
                        color: '#e1e4e8',
                        width: 1
                    }
                },
                axisLabel: {
                    color: '#666',
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
                    color: '#666',
                    fontSize: 11,
                    margin: 10
                },
                axisTick: {
                    show: false
                },
                splitLine: {
                    lineStyle: {
                        color: '#f0f0f0',
                        type: 'dashed',
                        width: 1
                    }
                }
            },
            series: [
                {
                    name: '请求数量',
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
                        borderColor: '#fff'
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
                    name: '异常数量',
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
                        borderColor: '#fff'
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
    }, [chartData])

    if (loading) {
        return (
            <div className="flex flex-col gap-4 h-[calc(100vh-280px)]">
                <div className="flex-1 bg-gradient-to-r from-slate-100 to-slate-200 animate-pulse rounded-lg"></div>
                <div className="flex-1 bg-gradient-to-r from-slate-100 to-slate-200 animate-pulse rounded-lg"></div>
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