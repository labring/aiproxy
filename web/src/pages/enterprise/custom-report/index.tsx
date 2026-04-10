import { useState, useEffect, useRef, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useMutation } from "@tanstack/react-query"
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
import { type TimeRange, getTimeRange } from "@/lib/enterprise"

import { type ChartType, type ViewMode, type ReportTemplate, TIME_DIMENSIONS, DEFAULT_DIMS, DEFAULT_MEASURES } from "./types"
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

    // Layout state
    const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
    const [mobileSheetOpen, setMobileSheetOpen] = useState(false)
    const [chartFullscreen, setChartFullscreen] = useState(false)
    const [saveTemplateOpen, setSaveTemplateOpen] = useState(false)
    const dialogRef = useRef<HTMLDialogElement>(null)

    // Generate report mutation
    const mutation = useMutation({
        mutationFn: (req: CustomReportRequest) => enterpriseApi.generateCustomReport(req),
        onSuccess: (data) => setReportData(data),
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

        const customStart = customDateRange?.from ? Math.floor(customDateRange.from.getTime() / 1000) : undefined
        const customEnd = customDateRange?.to ? Math.floor(customDateRange.to.getTime() / 1000) : undefined
        const { start, end } = getTimeRange(timeRange, customStart, customEnd)

        const filters: CustomReportRequest["filters"] = {}
        if (filterDepts.length > 0) filters.department_ids = filterDepts
        if (filterModels.length > 0) filters.models = filterModels
        if (filterUsers.length > 0) filters.user_names = filterUsers

        const req: CustomReportRequest = {
            dimensions: d,
            measures: m,
            filters,
            time_range: { start_timestamp: start, end_timestamp: end },
            sort_by: sortBy,
            sort_order: sortOrder,
            limit: 200,
        }
        mutateRef.current(req)
    }, [selectedDimensions, selectedMeasures, timeRange, customDateRange, filterDepts, filterModels, filterUsers, sortBy, sortOrder])

    // Handle preset template click
    const applyTemplate = useCallback((template: ReportTemplate) => {
        setSelectedDimensions(template.dimensions)
        setSelectedMeasures(template.measures)
        setPivotMeasure("")
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
    }, [])

    // Auto-generate on any config change (debounced 600ms)
    const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)
    useEffect(() => {
        if (selectedDimensions.length === 0 || selectedMeasures.length === 0) return
        clearTimeout(debounceRef.current)
        debounceRef.current = setTimeout(() => {
            handleGenerate()
        }, 600)
        return () => clearTimeout(debounceRef.current)
    }, [selectedDimensions, selectedMeasures, timeRange, customDateRange, filterDepts, filterModels, filterUsers, handleGenerate])

    // Reset viewMode if pivot not available
    const canPivot = selectedDimensions.length === 2
    useEffect(() => {
        if (!canPivot && viewMode === "pivot") setViewMode("table")
    }, [canPivot, viewMode])

    const activePivotMeasure = pivotMeasure && selectedMeasures.includes(pivotMeasure)
        ? pivotMeasure
        : selectedMeasures[0] ?? ""

    // Sort handler (client-side)
    const handleSort = (key: string, order: "asc" | "desc") => {
        setSortBy(key)
        setSortOrder(order)
        if (reportData) {
            const timeDim = selectedDimensions.find((d) => TIME_DIMENSIONS.has(d))
            const sorted = [...reportData.rows].sort((a, b) => {
                const va = Number(a[key]) || 0
                const vb = Number(b[key]) || 0
                if (va !== vb) return order === "desc" ? vb - va : va - vb
                if (timeDim && key !== timeDim) {
                    const ta = Number(a[timeDim] ?? 0)
                    const tb = Number(b[timeDim] ?? 0)
                    if (ta !== tb) return ta - tb
                }
                return String(a[key] ?? "").localeCompare(String(b[key] ?? ""))
            })
            setReportData({ ...reportData, rows: sorted })
        }
    }

    const canExport = useHasPermission('export_manage')

    // CSV export
    const handleExportCsv = () => {
        if (!reportData || reportData.rows.length === 0) return
        const cols = reportData.columns
        const header = cols.map((c) => c.label).join(",")
        const rows = reportData.rows.map((row) =>
            cols.map((c) => {
                const v = row[c.key]
                const s = String(v ?? "")
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
                            <CardContent className="py-4 text-center text-destructive">
                                {mutation.error instanceof Error ? mutation.error.message : String(mutation.error)}
                            </CardContent>
                        </Card>
                    )}

                    {/* Results */}
                    {hasResults && (
                        <>
                            {/* KPI Summary */}
                            <KpiSummaryRow data={reportData} measures={selectedMeasures} />

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
