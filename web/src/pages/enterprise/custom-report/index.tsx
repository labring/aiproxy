import { useState, useEffect, useRef, useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery } from "@tanstack/react-query"
import { type DateRange } from "react-day-picker"
import {
    FileBarChart,
    Table2,
    BarChart3,
    Grid3X3,
    Download,
    Columns2,
    Maximize2,
    X,
    LayoutGrid,
    AlertTriangle,
    ChevronRight,
    Home,
} from "lucide-react"
import { useHasPermission } from "@/lib/permissions"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import {
    enterpriseApi,
    type CustomReportRequest,
    type CustomReportResponse,
} from "@/api/enterprise"
import { type TimeRange, computeTimeRangeTs } from "@/lib/enterprise"

import {
    type ChartType, type ViewMode, type ReportTemplate, type DrillStep,
    DEFAULT_DIMS, DEFAULT_MEASURES, formatCellValue, getLabel,
    DRILL_HIERARCHY, DRILL_FILTER_MAP,
} from "./types"
import { ConfigPanel } from "./ConfigPanel"
import { FilterBar } from "./FilterBar"
import { KpiSummaryRow } from "./KpiSummaryRow"
import { ReportTable } from "./ReportTable"
import { ReportChart } from "./ReportChart"
import { PivotTable } from "./PivotTable"
import { SplitView } from "./SplitView"
import { ChartTypePicker } from "./ChartTypePicker"
import { DashboardGrid } from "./DashboardGrid"
import { TemplateManager } from "./TemplateManager"
import { SkeletonChart, EmptyState } from "./EmptyState"

export default function EnterpriseCustomReport() {
    const { t, i18n } = useTranslation()
    const lang = i18n.language

    // Config state
    const [selectedDimensions, setSelectedDimensions] = useState<string[]>([...DEFAULT_DIMS])
    const [selectedMeasures, setSelectedMeasures] = useState<string[]>([...DEFAULT_MEASURES])
    const [timeRange, setTimeRange] = useState<TimeRange>("last_week")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()

    // Filter state
    const [filterDepts, setFilterDepts] = useState<string[]>([])
    const [filterModels, setFilterModels] = useState<string[]>([])
    const [filterUsers, setFilterUsers] = useState<string[]>([])

    // View state
    const [viewMode, setViewMode] = useState<ViewMode>("table")
    const [chartType, setChartType] = useState<ChartType>("auto")
    const [reportData, setReportData] = useState<CustomReportResponse | null>(null)
    const [sortBy, setSortBy] = useState<string | undefined>()
    const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc")
    const [pivotMeasure, setPivotMeasure] = useState<string>("")

    // Drill-down state
    const [drillPath, setDrillPath] = useState<DrillStep[]>([])

    // Layout state
    const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
    const [mobileSheetOpen, setMobileSheetOpen] = useState(false)
    const [chartFullscreen, setChartFullscreen] = useState(false)
    const [saveTemplateOpen, setSaveTemplateOpen] = useState(false)
    const dialogRef = useRef<HTMLDialogElement>(null)

    // AbortController-based request lifecycle: each new request aborts the
    // previous in-flight one, preventing stale-response overwrites and freeing
    // network resources. This replaces the old sequence-number approach.
    const abortRef = useRef<AbortController | null>(null)
    const mutation = useMutation({
        mutationFn: ({ req, signal }: { req: CustomReportRequest; signal: AbortSignal }) =>
            enterpriseApi.generateCustomReport(req, signal),
    })
    const mutateRef = useRef(mutation.mutate)
    mutateRef.current = mutation.mutate

    const handleGenerate = useCallback((
        dims?: string[],
        meas?: string[],
    ) => {
        const d = dims ?? selectedDimensions
        const m = meas ?? selectedMeasures
        if (d.length === 0 || m.length === 0) return

        const { start, end } = computeTimeRangeTs(timeRange, customDateRange)

        const filters: CustomReportRequest["filters"] = {}
        if (filterDepts.length > 0) filters.department_ids = filterDepts
        if (filterModels.length > 0) filters.models = filterModels
        if (filterUsers.length > 0) filters.user_names = filterUsers

        // Align day/week buckets with the client's local timezone so that
        // a selection of "April 10" shows activity from local 00:00–23:59,
        // matching what the user sees on the dashboard. getTimezoneOffset()
        // returns minutes-west-of-UTC (e.g. -480 for CST); we want
        // seconds-east-of-UTC (e.g. +28800 for CST).
        //
        // DST caveat: getTimezoneOffset() returns the *current* offset, not
        // the offset that applied at each historical timestamp. Queries that
        // span a DST transition in affected zones (most of the Americas and
        // Europe) will bucket the hours around the transition into the wrong
        // local day. China/CST observes no DST so is unaffected. A proper
        // fix requires server-side timezone-aware bucketing (e.g. PostgreSQL
        // `AT TIME ZONE 'Asia/Shanghai'`); deferred since current deployment
        // is CST-only.
        const tzOffsetSec = -new Date().getTimezoneOffset() * 60

        // Abort any in-flight request before starting a new one
        abortRef.current?.abort()
        const controller = new AbortController()
        abortRef.current = controller

        const req: CustomReportRequest = {
            dimensions: d,
            measures: m,
            filters,
            time_range: { start_timestamp: start, end_timestamp: end },
            sort_by: sortBy,
            sort_order: sortOrder,
            limit: 200,
            timezone_offset_seconds: tzOffsetSec,
        }
        mutateRef.current({ req, signal: controller.signal }, {
            onSuccess: (data) => setReportData(data),
        })
    }, [selectedDimensions, selectedMeasures, timeRange, customDateRange, filterDepts, filterModels, filterUsers, sortBy, sortOrder])

    // Handle preset template click
    const applyTemplate = useCallback((template: ReportTemplate) => {
        setSelectedDimensions(template.dimensions)
        setSelectedMeasures(template.measures)
        setPivotMeasure("")
        setDrillPath([])
    }, [])

    // Handle saved template apply
    const applySavedTemplate = useCallback((dims: string[], meas: string[], chartTypeVal?: string, viewModeVal?: string) => {
        setSelectedDimensions(dims)
        setSelectedMeasures(meas)
        if (chartTypeVal) setChartType(chartTypeVal as ChartType)
        if (viewModeVal) setViewMode(viewModeVal as ViewMode)
        setPivotMeasure("")
    }, [])

    // Reset to defaults
    const handleReset = useCallback(() => {
        setSelectedDimensions([...DEFAULT_DIMS])
        setSelectedMeasures([...DEFAULT_MEASURES])
        setChartType("auto")
        setViewMode("table")
        setSortBy(undefined)
        setSortOrder("desc")
        setPivotMeasure("")
        setDrillPath([])
    }, [])

    // Auto-generate on any config change (debounced 600ms)
    const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)
    useEffect(() => {
        if (selectedDimensions.length === 0 || selectedMeasures.length === 0) return
        clearTimeout(debounceRef.current)
        debounceRef.current = setTimeout(() => {
            handleGenerate()
        }, 600)
        return () => {
            clearTimeout(debounceRef.current)
            abortRef.current?.abort()
        }
    }, [selectedDimensions, selectedMeasures, timeRange, customDateRange, filterDepts, filterModels, filterUsers, handleGenerate])

    // Reset viewMode if pivot not available
    const canPivot = selectedDimensions.length === 2
    useEffect(() => {
        if (!canPivot && viewMode === "pivot") setViewMode("table")
    }, [canPivot, viewMode])

    // Clear stale pivotMeasure when the selected measure is removed
    useEffect(() => {
        if (pivotMeasure && !selectedMeasures.includes(pivotMeasure)) {
            setPivotMeasure("")
        }
    }, [selectedMeasures, pivotMeasure])

    const activePivotMeasure = pivotMeasure && selectedMeasures.includes(pivotMeasure)
        ? pivotMeasure
        : selectedMeasures[0] ?? ""

    // Sort handler — only update state; the debounced useEffect re-fetches
    // from backend with the new sort_by/sort_order, avoiding stale-data flicker.
    const handleSort = (key: string, order: "asc" | "desc") => {
        setSortBy(key)
        setSortOrder(order)
    }

    // Map filter field names to their state setters — shared by drill/drillBack
    const filterSetters = useMemo(() => ({
        department_ids: setFilterDepts,
        models: setFilterModels,
        user_names: setFilterUsers,
    }), [])

    // Drill-down handler: click a dimension value → filter by it + drill to child dimension
    const handleDrill = useCallback((dimension: string, value: string, label: string) => {
        const child = DRILL_HIERARCHY[dimension]
        const filterField = DRILL_FILTER_MAP[dimension]
        if (!child || !filterField) return

        setDrillPath((prev) => [...prev, { dimension, value, label }])
        setSelectedDimensions((prev) =>
            prev.map((d) => d === dimension ? child : d),
        )
        filterSetters[filterField]((prev) => prev.includes(value) ? prev : [...prev, value])
        setSortBy(undefined)
    }, [filterSetters])

    // Drill breadcrumb navigation: revert to a specific level
    const handleDrillBack = useCallback((toIndex: number) => {
        // toIndex = -1 means go back to root (before any drill)
        const stepsToRemove = drillPath.slice(toIndex + 1)
        setDrillPath((prev) => prev.slice(0, toIndex + 1))

        // Replay dimensions: start from current and reverse each removed step
        let dims = [...selectedDimensions]
        for (let i = stepsToRemove.length - 1; i >= 0; i--) {
            const step = stepsToRemove[i]
            const child = DRILL_HIERARCHY[step.dimension]
            if (child) {
                dims = dims.map((d) => d === child ? step.dimension : d)
            }
        }
        setSelectedDimensions(dims)

        // Remove filters added by removed steps
        const toRemove: Record<string, Set<string>> = {}
        for (const step of stepsToRemove) {
            const ff = DRILL_FILTER_MAP[step.dimension]
            if (ff) (toRemove[ff] ??= new Set()).add(step.value)
        }
        for (const [field, values] of Object.entries(toRemove)) {
            filterSetters[field as keyof typeof filterSetters]((prev) => prev.filter((v) => !values.has(v)))
        }

        setSortBy(undefined)
    }, [drillPath, selectedDimensions, filterSetters])

    // Memoize time range timestamps — used by comparison query and ConfigPanel.
    // Without useMemo, computeTimeRangeTs runs every render (calls new Date() internally).
    const { start: tsStart, end: tsEnd } = useMemo(
        () => computeTimeRangeTs(timeRange, customDateRange),
        [timeRange, customDateRange],
    )
    const comparisonQuery = useQuery({
        queryKey: ["custom-report-comparison", filterDepts, tsStart, tsEnd],
        queryFn: () => enterpriseApi.getComparison(
            filterDepts.length > 0 ? filterDepts : undefined,
            tsStart,
            tsEnd,
        ),
        enabled: !!reportData,
        staleTime: 60_000,
    })

    const canExport = useHasPermission('export_manage')

    // CSV export — values are formatted identically to the on-screen table
    // so that what the user sees is what they get in the spreadsheet.
    const handleExportCsv = () => {
        if (!reportData || reportData.rows.length === 0) return
        const cols = reportData.columns
        const header = cols.map((c) => getLabel(c.key, lang)).join(",")
        const rows = reportData.rows.map((row) =>
            cols.map((c) => {
                const s = formatCellValue(c.key, row[c.key])
                if (s.includes(",") || s.includes('"') || s.includes("\n")) {
                    return `"${s.replace(/"/g, '""')}"`
                }
                return s
            }).join(","),
        )
        const bom = "\uFEFF"
        const csv = bom + header + "\n" + rows.join("\n")
        const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" })
        const url = URL.createObjectURL(blob)
        const a = document.createElement("a")
        a.href = url
        a.download = `custom_report_${new Date().toISOString().slice(0, 10)}.csv`
        a.click()
        URL.revokeObjectURL(url)
    }

    const openFullscreen = useCallback(() => {
        setChartFullscreen(true)
        dialogRef.current?.showModal()
    }, [])

    const closeFullscreen = useCallback(() => {
        dialogRef.current?.close()
        setChartFullscreen(false)
    }, [])

    const hasResults = reportData && reportData.rows.length > 0

    // Template manager slot for ConfigPanel
    const templateManagerSlot = (
        <TemplateManager
            onApply={applySavedTemplate}
            currentDimensions={selectedDimensions}
            currentMeasures={selectedMeasures}
            currentChartType={chartType}
            currentViewMode={viewMode}
            saveDialogOpen={saveTemplateOpen}
            onSaveDialogChange={setSaveTemplateOpen}
        />
    )

    // ConfigPanel content (shared between desktop sidebar and mobile sheet)
    const configContent = (
        <ConfigPanel
            collapsed={false}
            onToggleCollapse={() => setSidebarCollapsed(true)}
            selectedDimensions={selectedDimensions}
            onDimensionsChange={setSelectedDimensions}
            selectedMeasures={selectedMeasures}
            onMeasuresChange={setSelectedMeasures}
            onApplyTemplate={(tpl) => {
                applyTemplate(tpl)
                setMobileSheetOpen(false)
            }}
            onReset={handleReset}
            onSaveTemplate={() => setSaveTemplateOpen(true)}
            isPending={mutation.isPending}
            templateManagerSlot={templateManagerSlot}
            startTs={tsStart}
            endTs={tsEnd}
        />
    )

    return (
        <div className="h-full flex flex-col">
            {/* Header */}
            <div className="px-6 py-4 border-b">
                <h1 className="text-2xl font-bold flex items-center gap-2">
                    <FileBarChart className="w-6 h-6 text-[#6A6DE6]" />
                    {t("enterprise.customReport.title")}
                </h1>
                <p className="text-muted-foreground mt-1 text-sm">
                    {t("enterprise.customReport.description")}
                </p>
            </div>

            {/* Main layout: sidebar + content */}
            <div className="flex-1 flex overflow-hidden">
                {/* Desktop sidebar */}
                <div className={`hidden lg:flex flex-col border-r bg-background transition-all duration-200 ${
                    sidebarCollapsed ? "w-12" : "w-[320px]"
                }`}>
                    {sidebarCollapsed ? (
                        <ConfigPanel
                            collapsed={true}
                            onToggleCollapse={() => setSidebarCollapsed(false)}
                            selectedDimensions={selectedDimensions}
                            onDimensionsChange={setSelectedDimensions}
                            selectedMeasures={selectedMeasures}
                            onMeasuresChange={setSelectedMeasures}
                            onApplyTemplate={applyTemplate}
                            isPending={mutation.isPending}
                        />
                    ) : (
                        configContent
                    )}
                </div>

                {/* Mobile sheet */}
                <Sheet open={mobileSheetOpen} onOpenChange={setMobileSheetOpen}>
                    <SheetContent side="left" className="w-[320px] p-0">
                        <SheetHeader className="px-4 py-3 border-b">
                            <SheetTitle>{t("enterprise.customReport.configPanel")}</SheetTitle>
                        </SheetHeader>
                        {configContent}
                    </SheetContent>
                </Sheet>

                {/* Content area */}
                <div className="flex-1 overflow-y-auto p-6 space-y-4">
                    {/* Mobile: config button */}
                    <div className="lg:hidden">
                        <Button
                            variant="outline"
                            onClick={() => setMobileSheetOpen(true)}
                            className="gap-2"
                        >
                            <FileBarChart className="w-4 h-4" />
                            {t("enterprise.customReport.configPanel")}
                        </Button>
                    </div>

                    {/* Filter bar — always visible at top */}
                    <FilterBar
                        timeRange={timeRange}
                        onTimeRangeChange={setTimeRange}
                        customDateRange={customDateRange}
                        onCustomDateRangeChange={setCustomDateRange}
                        filterDepts={filterDepts}
                        onFilterDeptsChange={setFilterDepts}
                        filterModels={filterModels}
                        onFilterModelsChange={setFilterModels}
                        filterUsers={filterUsers}
                        onFilterUsersChange={setFilterUsers}
                    />

                    {/* Loading */}
                    {mutation.isPending && !reportData && <SkeletonChart />}
                    {mutation.isPending && reportData && (
                        <div className="h-0.5 w-full bg-[#6A6DE6]/20 rounded-full overflow-hidden">
                            <div className="h-full bg-[#6A6DE6] rounded-full animate-pulse w-2/3" />
                        </div>
                    )}

                    {/* Error state */}
                    {mutation.isError && (
                        <Card className="border-destructive shadow-sm">
                            <CardContent className="py-4 flex items-center justify-center gap-3 text-destructive">
                                <span>{mutation.error instanceof Error ? mutation.error.message : String(mutation.error)}</span>
                                <Button variant="outline" size="sm" onClick={() => handleGenerate()}>
                                    {t("enterprise.customReport.retry")}
                                </Button>
                            </CardContent>
                        </Card>
                    )}

                    {/* Results */}
                    {hasResults && (
                        <>
                            {/* KPI Summary */}
                            <KpiSummaryRow data={reportData} measures={selectedMeasures} comparison={comparisonQuery.data} />

                            {/* Truncation warning */}
                            {reportData.total > reportData.rows.length && (
                                <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-sm text-amber-700 dark:text-amber-300">
                                    <AlertTriangle className="w-4 h-4 shrink-0" />
                                    <span>
                                        {t("enterprise.customReport.truncationWarning", {
                                            shown: reportData.rows.length.toLocaleString(),
                                            total: reportData.total.toLocaleString(),
                                        })}
                                    </span>
                                    {canExport && (
                                        <Button variant="link" size="sm" className="text-amber-700 dark:text-amber-300 p-0 h-auto" onClick={handleExportCsv}>
                                            {t("enterprise.customReport.exportForFull")}
                                        </Button>
                                    )}
                                </div>
                            )}

                            {/* Drill-down breadcrumbs */}
                            {drillPath.length > 0 && (
                                <div className="flex items-center gap-1 text-sm flex-wrap">
                                    <button
                                        type="button"
                                        className="flex items-center gap-1 text-muted-foreground hover:text-foreground transition-colors"
                                        onClick={() => handleDrillBack(-1)}
                                    >
                                        <Home className="w-3.5 h-3.5" />
                                        <span>{t("enterprise.customReport.drillRoot")}</span>
                                    </button>
                                    {drillPath.map((step, i) => (
                                        <span key={i} className="flex items-center gap-1">
                                            <ChevronRight className="w-3.5 h-3.5 text-muted-foreground/50" />
                                            <button
                                                type="button"
                                                className={`hover:text-foreground transition-colors ${
                                                    i === drillPath.length - 1
                                                        ? "text-[#6A6DE6] font-medium"
                                                        : "text-muted-foreground"
                                                }`}
                                                onClick={() => handleDrillBack(i)}
                                            >
                                                {getLabel(step.dimension, lang)}: {step.label}
                                            </button>
                                        </span>
                                    ))}
                                </div>
                            )}

                            {/* Toolbar */}
                            <div className="sticky top-0 z-10 bg-background/95 backdrop-blur-sm flex flex-wrap items-center gap-2 py-1">
                                {/* View mode switcher */}
                                <div className="flex items-center border rounded-lg overflow-hidden">
                                    {([
                                        { mode: "table" as ViewMode, icon: Table2, labelKey: "enterprise.customReport.tableView" },
                                        { mode: "chart" as ViewMode, icon: BarChart3, labelKey: "enterprise.customReport.chartView" },
                                        ...(canPivot ? [{ mode: "pivot" as ViewMode, icon: Grid3X3, labelKey: "enterprise.customReport.pivotView" }] : []),
                                        { mode: "split" as ViewMode, icon: Columns2, labelKey: "enterprise.customReport.splitView" },
                                        { mode: "dashboard" as ViewMode, icon: LayoutGrid, labelKey: "enterprise.customReport.dashboardView" },
                                    ]).map(({ mode, icon: Icon, labelKey }) => (
                                        <Button
                                            key={mode}
                                            variant={viewMode === mode ? "default" : "ghost"}
                                            size="sm"
                                            onClick={() => setViewMode(mode)}
                                            className={`gap-1.5 rounded-none text-xs ${viewMode === mode ? "bg-[#6A6DE6] text-white" : ""}`}
                                        >
                                            <Icon className="w-3.5 h-3.5" />
                                            <span className="hidden sm:inline">{t(labelKey as never)}</span>
                                        </Button>
                                    ))}
                                </div>

                                {/* Chart type picker (visible in chart, split, dashboard modes) */}
                                {(viewMode === "chart" || viewMode === "split") && (
                                    <ChartTypePicker value={chartType} onChange={setChartType} />
                                )}

                                <div className="ml-auto flex items-center gap-2">
                                    {/* Fullscreen (chart/split modes only) */}
                                    {(viewMode === "chart" || viewMode === "split") && (
                                        <TooltipProvider>
                                            <Tooltip>
                                                <TooltipTrigger asChild>
                                                    <Button variant="outline" size="sm" onClick={openFullscreen}>
                                                        <Maximize2 className="w-4 h-4" />
                                                    </Button>
                                                </TooltipTrigger>
                                                <TooltipContent>{t("enterprise.customReport.fullscreen")}</TooltipContent>
                                            </Tooltip>
                                        </TooltipProvider>
                                    )}

                                    {/* Export */}
                                    {canExport && (
                                        <Button variant="outline" size="sm" onClick={handleExportCsv}>
                                            <Download className="w-4 h-4 mr-1.5" />
                                            {t("enterprise.customReport.exportCsv")}
                                        </Button>
                                    )}
                                </div>
                            </div>

                            {/* Content card */}
                            <Card className="shadow-sm border-0">
                                <CardContent className="p-0">
                                    {viewMode === "table" && (
                                        <ReportTable
                                            data={reportData}
                                            dimensions={selectedDimensions}
                                            sortBy={sortBy}
                                            sortOrder={sortOrder}
                                            onSort={handleSort}
                                            onDrill={handleDrill}
                                        />
                                    )}
                                    {viewMode === "chart" && (
                                        <div className="p-4">
                                            <ReportChart
                                                data={reportData}
                                                dimensions={selectedDimensions}
                                                measures={selectedMeasures}
                                                chartType={chartType}
                                                lang={lang}
                                            />
                                        </div>
                                    )}
                                    {viewMode === "pivot" && canPivot && (
                                        <PivotTable
                                            data={reportData}
                                            dim1={selectedDimensions[0]}
                                            dim2={selectedDimensions[1]}
                                            measures={selectedMeasures}
                                            selectedMeasure={activePivotMeasure}
                                            onMeasureChange={setPivotMeasure}
                                            lang={lang}
                                            t={t}
                                        />
                                    )}
                                    {viewMode === "split" && (
                                        <SplitView
                                            data={reportData}
                                            dimensions={selectedDimensions}
                                            measures={selectedMeasures}
                                            chartType={chartType}
                                            lang={lang}
                                            sortBy={sortBy}
                                            sortOrder={sortOrder}
                                            onSort={handleSort}
                                        />
                                    )}
                                    {viewMode === "dashboard" && (
                                        <DashboardGrid
                                            data={reportData}
                                            dimensions={selectedDimensions}
                                            measures={selectedMeasures}
                                            lang={lang}
                                        />
                                    )}
                                </CardContent>
                            </Card>
                        </>
                    )}

                    {/* Empty result state */}
                    {reportData && reportData.rows.length === 0 && (
                        <Card className="shadow-sm border-0">
                            <CardContent className="py-12 text-center text-muted-foreground">
                                {t("enterprise.customReport.noData")}
                            </CardContent>
                        </Card>
                    )}

                    {/* Initial state */}
                    {!reportData && !mutation.isPending && <EmptyState />}
                </div>
            </div>

            {/* Fullscreen chart dialog */}
            <dialog
                ref={dialogRef}
                className="fixed inset-0 w-[95vw] h-[90vh] max-w-none max-h-none bg-background rounded-xl shadow-2xl p-0 backdrop:bg-black/50"
                onClose={() => setChartFullscreen(false)}
            >
                {chartFullscreen && hasResults && (
                    <div className="flex flex-col h-full">
                        <div className="flex items-center justify-between px-4 py-2 border-b shrink-0">
                            <div className="flex items-center gap-2">
                                <ChartTypePicker value={chartType} onChange={setChartType} />
                            </div>
                            <div className="flex items-center gap-3">
                                <span className="text-xs text-muted-foreground">
                                    {t("enterprise.customReport.pressEscToExit")}
                                    <kbd className="ml-1.5 px-1.5 py-0.5 rounded border bg-muted text-[10px] font-mono">esc</kbd>
                                </span>
                                <Button variant="ghost" size="icon" onClick={closeFullscreen} className="h-8 w-8">
                                    <X className="w-4 h-4" />
                                </Button>
                            </div>
                        </div>
                        <div className="flex-1 p-4 overflow-hidden">
                            <ReportChart
                                data={reportData}
                                dimensions={selectedDimensions}
                                measures={selectedMeasures}
                                chartType={chartType}
                                lang={lang}
                                fullscreen={true}
                            />
                        </div>
                    </div>
                )}
            </dialog>
        </div>
    )
}
