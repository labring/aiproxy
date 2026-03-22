import { useEffect, useMemo, useRef } from "react"
import * as echarts from "echarts"
import type { CustomReportResponse } from "@/api/enterprise"
import { useDarkMode, getEChartsTheme } from "@/lib/enterprise"
import {
    type ChartType,
    CHART_COLORS,
    PERCENTAGE_FIELDS,
    TIME_DIMENSIONS,
    getLabel,
    formatDimValue,
    recommendChartType,
    sortRowsByTime,
} from "./types"

/** Estimate legend rows and compute grid top offset so legend never overlaps chart */
function legendGridTop(itemCount: number, containerWidth = 800): number {
    // Each legend item is roughly 80–120px wide; estimate items per row
    const usableWidth = containerWidth * 0.9
    const avgItemWidth = 100
    const itemsPerRow = Math.max(Math.floor(usableWidth / avgItemWidth), 1)
    const rows = Math.ceil(itemCount / itemsPerRow)
    // Each row ~22px, plus 8px padding
    return Math.max(rows * 22 + 8, 30)
}

/** Build a wrapping legend config with auto scroll for many items */
function wrapLegend(data: string[], textColor: string): echarts.EChartsOption["legend"] {
    const useScroll = data.length > 15
    return {
        data,
        textStyle: { color: textColor, fontSize: 11 },
        type: useScroll ? "scroll" : ("plain" as const),
        width: "90%",
        left: "center",
        top: 0,
        ...(useScroll ? { pageTextStyle: { color: textColor } } : {}),
    }
}

/** Compute chart container height based on legend count */
function computeChartHeight(legendCount: number, fullscreen: boolean): number {
    if (fullscreen) return 0 // CSS-controlled
    const legendRows = Math.ceil(Math.min(legendCount, 15) / 5)
    const legendHeight = legendRows * 22 + 8
    const minChartArea = 300
    const maxHeight = 600
    return Math.min(legendHeight + minChartArea, maxHeight)
}

/** Compute rotation and interval for X-axis labels */
function xAxisLabelConfig(labels: string[]): { rotate: number; interval: number; fontSize: number } {
    const count = labels.length
    const maxLen = Math.max(...labels.map((l) => l.length), 0)
    if (count <= 7 && maxLen <= 10) return { rotate: 0, interval: 0, fontSize: 11 }
    if (count <= 15) return { rotate: 30, interval: 0, fontSize: 10 }
    if (count <= 31) return { rotate: 45, interval: 0, fontSize: 10 }
    return { rotate: 45, interval: Math.floor(count / 25), fontSize: 9 }
}

// Separate dimensions into primary (X-axis) and secondary (series grouping).
// Time dimensions are preferred as primary; if none, use the first dimension.
function splitDimensions(dimensions: string[]): { primary: string; secondary: string | null } {
    if (dimensions.length <= 1) {
        return { primary: dimensions[0] ?? "", secondary: null }
    }
    const timeDim = dimensions.find((d) => TIME_DIMENSIONS.has(d))
    if (timeDim) {
        const other = dimensions.find((d) => d !== timeDim) ?? null
        return { primary: timeDim, secondary: other }
    }
    return { primary: dimensions[0], secondary: dimensions[1] ?? null }
}

// Build formatted label for a single dimension value
function dimLabel(dimKey: string, row: Record<string, unknown>): string {
    return formatDimValue(dimKey, row[dimKey])
}

export function ReportChart({
    data,
    dimensions,
    measures,
    chartType,
    lang,
    fullscreen = false,
}: {
    data: CustomReportResponse
    dimensions: string[]
    measures: string[]
    chartType: ChartType
    lang: string
    fullscreen?: boolean
}) {
    const chartRef = useRef<HTMLDivElement>(null)
    const instance = useRef<echarts.ECharts | null>(null)
    const isDark = useDarkMode()

    useEffect(() => {
        if (!chartRef.current || data.rows.length === 0) return

        if (!instance.current) {
            instance.current = echarts.init(chartRef.current)
        }

        const theme = getEChartsTheme(isDark)
        const resolvedType = chartType === "auto" ? recommendChartType(dimensions, measures) : chartType

        const { primary, secondary } = splitDimensions(dimensions)

        // Sort rows by time dimension if present
        const rows = sortRowsByTime(data.rows, dimensions)

        // Build formatted labels from all dimensions (for single-dim charts)
        const labels = rows.map((row) =>
            dimensions.map((d) => formatDimValue(d, row[d])).join(" / "),
        )

        // Only numeric measures
        const numericMeasures = measures.filter((m) => {
            const first = rows[0]?.[m]
            return first !== undefined && !Number.isNaN(Number(first))
        })

        let option: echarts.EChartsOption

        switch (resolvedType) {
            case "pie": {
                const measure = numericMeasures[0]
                if (!measure) return
                option = {
                    tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
                    series: [{
                        type: "pie",
                        radius: ["40%", "70%"],
                        data: rows.slice(0, 15).map((row, i) => ({
                            name: labels[i],
                            value: Number(row[measure] ?? 0),
                            itemStyle: { color: CHART_COLORS[i % CHART_COLORS.length] },
                        })),
                        label: { show: true, formatter: "{b}\n{d}%", color: theme.textColor },
                    }],
                }
                break
            }

            case "heatmap": {
                const dim0 = dimensions[0]
                const dim1 = dimensions[1]
                const dim0Values = [...new Set(rows.map((r) => formatDimValue(dim0, r[dim0])))]
                const dim1Values = [...new Set(rows.map((r) => formatDimValue(dim1, r[dim1])))]
                const measure = numericMeasures[0]
                if (!measure) return

                const heatData: [number, number, number][] = []
                for (const row of rows) {
                    const x = dim0Values.indexOf(formatDimValue(dim0, row[dim0]))
                    const y = dim1Values.indexOf(formatDimValue(dim1, row[dim1]))
                    if (x >= 0 && y >= 0) {
                        heatData.push([x, y, Number(row[measure] ?? 0)])
                    }
                }

                const values = heatData.map((d) => d[2])
                const minVal = Math.min(...values)
                const maxVal = Math.max(...values)

                option = {
                    tooltip: {
                        position: "top",
                        formatter: (p: unknown) => {
                            const params = p as { value: [number, number, number] }
                            return `${dim0Values[params.value[0]]} × ${dim1Values[params.value[1]]}<br/>${getLabel(measure, lang)}: ${params.value[2]}`
                        },
                    },
                    grid: { left: "15%", right: "10%", bottom: "15%", top: "5%" },
                    xAxis: {
                        type: "category",
                        data: dim0Values,
                        axisLabel: { rotate: 30, fontSize: 10, color: theme.subTextColor },
                        splitArea: { show: true },
                    },
                    yAxis: {
                        type: "category",
                        data: dim1Values,
                        axisLabel: { fontSize: 10, color: theme.subTextColor },
                        splitArea: { show: true },
                    },
                    visualMap: {
                        min: minVal,
                        max: maxVal,
                        calculable: true,
                        orient: "horizontal",
                        left: "center",
                        bottom: "0",
                        inRange: { color: ["#f0f0ff", "#6A6DE6", "#3a0ca3"] },
                        textStyle: { color: theme.subTextColor },
                    },
                    series: [{
                        type: "heatmap",
                        data: heatData,
                        label: { show: heatData.length <= 100, fontSize: 9 },
                        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: "rgba(0,0,0,0.3)" } },
                    }],
                }
                break
            }

            case "treemap": {
                const measure = numericMeasures[0]
                if (!measure) return

                if (dimensions.length >= 2) {
                    const groups = new Map<string, { name: string; value: number; children: { name: string; value: number }[] }>()
                    for (const row of rows) {
                        const parent = dimLabel(dimensions[0], row)
                        const child = dimLabel(dimensions[1], row)
                        const val = Number(row[measure] ?? 0)
                        if (!groups.has(parent)) {
                            groups.set(parent, { name: parent, value: 0, children: [] })
                        }
                        const g = groups.get(parent)!
                        g.value += val
                        g.children.push({ name: child, value: val })
                    }
                    option = {
                        tooltip: { formatter: (p: unknown) => {
                            const params = p as { name: string; value: number }
                            return `${params.name}: ${params.value}`
                        }},
                        series: [{
                            type: "treemap",
                            data: Array.from(groups.values()),
                            label: { show: true, formatter: "{b}" },
                            levels: [
                                { itemStyle: { borderColor: theme.borderColor, borderWidth: 2 } },
                                { itemStyle: { borderColor: theme.borderColor, borderWidth: 1 }, label: { fontSize: 10 } },
                            ],
                        }],
                    }
                } else {
                    option = {
                        tooltip: {},
                        series: [{
                            type: "treemap",
                            data: rows.slice(0, 30).map((row, i) => ({
                                name: labels[i],
                                value: Number(row[measure] ?? 0),
                            })),
                            label: { show: true, formatter: "{b}\n{c}" },
                        }],
                    }
                }
                break
            }

            case "radar": {
                const maxValues = numericMeasures.map((m) => {
                    const vals = rows.map((r) => Number(r[m] ?? 0))
                    return Math.max(...vals, 1)
                })
                const indicator = numericMeasures.map((m, i) => ({
                    name: getLabel(m, lang),
                    max: maxValues[i],
                }))

                const radarRows = rows.slice(0, 8)
                const radarLegendTop = legendGridTop(radarRows.length)
                option = {
                    tooltip: {},
                    legend: wrapLegend(radarRows.map((_, i) => labels[i]), theme.textColor),
                    radar: { indicator, shape: "polygon", center: ["50%", `${50 + radarLegendTop / 8}%`] },
                    series: [{
                        type: "radar",
                        data: radarRows.map((row, i) => ({
                            name: labels[i],
                            value: numericMeasures.map((m) => Number(row[m] ?? 0)),
                            lineStyle: { color: CHART_COLORS[i % CHART_COLORS.length] },
                            itemStyle: { color: CHART_COLORS[i % CHART_COLORS.length] },
                            areaStyle: { color: CHART_COLORS[i % CHART_COLORS.length], opacity: 0.1 },
                        })),
                    }],
                }
                break
            }

            default: {
                // bar, stacked_bar, line, area
                const isStacked = resolvedType === "stacked_bar"
                const isArea = resolvedType === "area"
                const seriesType = (resolvedType === "stacked_bar" || resolvedType === "bar") ? "bar" : "line"

                if (secondary) {
                    // ── Multi-dimension: primary = X-axis, secondary = series grouping ──
                    const primaryValues = [...new Set(rows.map((r) => formatDimValue(primary, r[primary])))]
                    const secondaryValues = [...new Set(rows.map((r) => formatDimValue(secondary, r[secondary])))]

                    // Build lookup: primaryLabel -> secondaryLabel -> { measure: value }
                    const lookup = new Map<string, Map<string, Record<string, number>>>()
                    for (const row of rows) {
                        const pKey = formatDimValue(primary, row[primary])
                        const sKey = formatDimValue(secondary, row[secondary])
                        if (!lookup.has(pKey)) lookup.set(pKey, new Map())
                        const sMap = lookup.get(pKey)!
                        if (!sMap.has(sKey)) sMap.set(sKey, {})
                        const rec = sMap.get(sKey)!
                        for (const m of numericMeasures) {
                            rec[m] = Number(row[m] ?? 0)
                        }
                    }

                    const labelCfg = xAxisLabelConfig(primaryValues)

                    // Build series: for each (secondaryValue, measure) pair
                    // If only 1 measure, series name = secondaryValue (cleaner legend)
                    // If multiple measures, series name = "secondaryValue - measureLabel"
                    const allSeries: echarts.EChartsOption["series"] = []
                    const legendData: string[] = []
                    let colorIdx = 0

                    if (numericMeasures.length === 1) {
                        const m = numericMeasures[0]
                        for (const sVal of secondaryValues) {
                            legendData.push(sVal)
                            allSeries.push({
                                name: sVal,
                                type: seriesType,
                                data: primaryValues.map((pVal) => lookup.get(pVal)?.get(sVal)?.[m] ?? 0),
                                itemStyle: { color: CHART_COLORS[colorIdx % CHART_COLORS.length] },
                                smooth: seriesType === "line",
                                stack: isStacked ? "total" : undefined,
                                ...(isArea ? { areaStyle: { opacity: 0.3 } } : {}),
                            })
                            colorIdx++
                        }
                    } else {
                        for (const sVal of secondaryValues) {
                            for (const m of numericMeasures) {
                                const seriesName = `${sVal} - ${getLabel(m, lang)}`
                                legendData.push(seriesName)
                                allSeries.push({
                                    name: seriesName,
                                    type: seriesType,
                                    data: primaryValues.map((pVal) => lookup.get(pVal)?.get(sVal)?.[m] ?? 0),
                                    itemStyle: { color: CHART_COLORS[colorIdx % CHART_COLORS.length] },
                                    smooth: seriesType === "line",
                                    stack: isStacked ? sVal : undefined,
                                    ...(isArea ? { areaStyle: { opacity: 0.3 } } : {}),
                                })
                                colorIdx++
                            }
                        }
                    }

                    const gridTop = legendGridTop(legendData.length)

                    option = {
                        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
                        legend: wrapLegend(legendData, theme.textColor),
                        grid: { left: "3%", right: "4%", bottom: "3%", top: gridTop, containLabel: true },
                        xAxis: {
                            type: "category",
                            data: primaryValues,
                            axisLabel: { rotate: labelCfg.rotate, interval: labelCfg.interval, fontSize: labelCfg.fontSize, color: theme.subTextColor },
                        },
                        yAxis: { type: "value", axisLabel: { color: theme.subTextColor }, splitLine: { lineStyle: { color: theme.splitLineColor } } },
                        series: allSeries,
                    }
                } else {
                    // ── Single dimension: each measure is a series ──
                    const xLabels = rows.slice(0, 50).map((row) => formatDimValue(primary, row[primary]))
                    const labelCfg = xAxisLabelConfig(xLabels)

                    const hasPercentage = numericMeasures.some((m) => PERCENTAGE_FIELDS.has(m))
                    const hasAbsolute = numericMeasures.some((m) => !PERCENTAGE_FIELDS.has(m))
                    const needDualAxis = hasPercentage && hasAbsolute && numericMeasures.length > 1

                    const singleGridTop = legendGridTop(numericMeasures.length)

                    option = {
                        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
                        legend: wrapLegend(numericMeasures.map((m) => getLabel(m, lang)), theme.textColor),
                        grid: { left: "3%", right: needDualAxis ? "8%" : "4%", bottom: "3%", top: singleGridTop, containLabel: true },
                        xAxis: {
                            type: "category",
                            data: xLabels,
                            axisLabel: { rotate: labelCfg.rotate, interval: labelCfg.interval, fontSize: labelCfg.fontSize, color: theme.subTextColor },
                        },
                        yAxis: needDualAxis
                            ? [
                                { type: "value", name: lang.startsWith("zh") ? "数值" : "Value", nameTextStyle: { color: theme.subTextColor }, axisLabel: { color: theme.subTextColor }, splitLine: { lineStyle: { color: theme.splitLineColor } } },
                                { type: "value", name: "%", max: 100, min: 0, nameTextStyle: { color: theme.subTextColor }, axisLabel: { color: theme.subTextColor }, splitLine: { show: false } },
                            ]
                            : { type: "value", axisLabel: { color: theme.subTextColor }, splitLine: { lineStyle: { color: theme.splitLineColor } } },
                        series: numericMeasures.map((m, i) => ({
                            name: getLabel(m, lang),
                            type: seriesType,
                            yAxisIndex: needDualAxis && PERCENTAGE_FIELDS.has(m) ? 1 : 0,
                            data: rows.slice(0, 50).map((row) => Number(row[m] ?? 0)),
                            itemStyle: { color: CHART_COLORS[i % CHART_COLORS.length] },
                            smooth: seriesType === "line",
                            ...(isStacked ? { stack: "total" } : {}),
                            ...(isArea ? { areaStyle: { opacity: 0.3 } } : {}),
                        })),
                    }
                }
                break
            }
        }

        instance.current.setOption(option, true)

        const handleResize = () => instance.current?.resize()
        window.addEventListener("resize", handleResize)
        return () => {
            window.removeEventListener("resize", handleResize)
        }
    }, [data, dimensions, measures, chartType, lang, isDark])

    // Estimate legend count for dynamic height
    const legendCount = useMemo(() => {
        const resolvedType = chartType === "auto" ? recommendChartType(dimensions, measures) : chartType
        if (resolvedType === "pie" || resolvedType === "treemap") return 0 // no legend-driven height
        if (resolvedType === "heatmap") return 0

        const { secondary } = splitDimensions(dimensions)
        if (secondary) {
            const secondaryValues = new Set(data.rows.map((r) => formatDimValue(secondary, r[secondary])))
            return measures.length === 1 ? secondaryValues.size : secondaryValues.size * measures.length
        }
        return measures.length
    }, [data, dimensions, measures, chartType])

    const dynamicHeight = computeChartHeight(legendCount, fullscreen)

    // Resize when fullscreen or dynamic height changes
    useEffect(() => {
        const timer = setTimeout(() => instance.current?.resize(), 50)
        return () => clearTimeout(timer)
    }, [fullscreen, dynamicHeight])

    // Clean up on unmount
    useEffect(() => {
        return () => {
            instance.current?.dispose()
            instance.current = null
        }
    }, [])

    return (
        <div
            ref={chartRef}
            className="w-full"
            style={{ height: fullscreen ? "calc(100vh - 120px)" : `${dynamicHeight}px` }}
        />
    )
}
