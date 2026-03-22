import { useState } from "react"
import { useTranslation } from "react-i18next"
import {
    ChevronDown,
    ChevronRight,
    X,
    Zap,
    PanelLeftClose,
    PanelLeftOpen,
    Loader2,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
    DIMENSION_FIELDS,
    MEASURE_FIELDS,
    CATEGORIES,
    REPORT_TEMPLATES,
    getLabel,
    type FieldDef,
    type ReportTemplate,
} from "./types"

// ─── ChipSelector ───────────────────────────────────────────────────────────

function ChipSelector({
    fields,
    selected,
    onChange,
    lang,
    active: activeColor = "bg-[#6A6DE6] text-white",
}: {
    fields: FieldDef[]
    selected: string[]
    onChange: (keys: string[]) => void
    lang: string
    active?: string
}) {
    return (
        <div className="flex flex-wrap gap-1.5">
            {fields.map((f) => {
                const isActive = selected.includes(f.key)
                return (
                    <Badge
                        key={f.key}
                        variant={isActive ? "default" : "outline"}
                        className={`cursor-pointer select-none transition-all text-xs px-2.5 py-1 ${
                            isActive ? `border-transparent ${activeColor}` : "hover:bg-muted/50"
                        }`}
                        onClick={() => {
                            onChange(
                                isActive
                                    ? selected.filter((k) => k !== f.key)
                                    : [...selected, f.key],
                            )
                        }}
                    >
                        {getLabel(f.key, lang)}
                        {isActive && <X className="w-3 h-3 ml-1" />}
                    </Badge>
                )
            })}
        </div>
    )
}

// ─── ConfigPanel props ──────────────────────────────────────────────────────

export interface ConfigPanelProps {
    collapsed: boolean
    onToggleCollapse: () => void
    selectedDimensions: string[]
    onDimensionsChange: (dims: string[]) => void
    selectedMeasures: string[]
    onMeasuresChange: (measures: string[]) => void
    onApplyTemplate: (template: ReportTemplate) => void
    isPending: boolean
}

export function ConfigPanel({
    collapsed,
    onToggleCollapse,
    selectedDimensions,
    onDimensionsChange,
    selectedMeasures,
    onMeasuresChange,
    onApplyTemplate,
    isPending,
}: ConfigPanelProps) {
    const { t, i18n } = useTranslation()
    const lang = i18n.language

    const [templatesOpen, setTemplatesOpen] = useState(false)

    const measuresByCategory = CATEGORIES.map((cat) => ({
        category: cat,
        fields: MEASURE_FIELDS.filter((f) => f.category === cat),
    }))

    // Department dimensions are mutually exclusive; time dimensions are mutually exclusive
    const DEPT_DIMS = new Set(["department", "level1_department", "level2_department"])
    const TIME_DIMS = new Set(["time_day", "time_week", "time_hour"])

    const handleDimensionChange = (next: string[]) => {
        let result = next
        const addedDept = next.find((d) => DEPT_DIMS.has(d) && !selectedDimensions.includes(d))
        if (addedDept) {
            result = result.filter((d) => !DEPT_DIMS.has(d) || d === addedDept)
        }
        const addedTime = result.find((d) => TIME_DIMS.has(d) && !selectedDimensions.includes(d))
        if (addedTime) {
            result = result.filter((d) => !TIME_DIMS.has(d) || d === addedTime)
        }
        onDimensionsChange(result)
    }

    if (collapsed) {
        return (
            <div className="flex flex-col items-center py-4 gap-2">
                <Button
                    variant="ghost"
                    size="icon"
                    onClick={onToggleCollapse}
                    className="h-8 w-8"
                    title={t("enterprise.customReport.expandPanel")}
                >
                    <PanelLeftOpen className="w-4 h-4" />
                </Button>
            </div>
        )
    }

    return (
        <div className="flex flex-col h-full">
            {/* Header with collapse button */}
            <div className="flex items-center justify-between px-4 py-3 border-b">
                <h2 className="text-sm font-semibold">{t("enterprise.customReport.configPanel")}</h2>
                <div className="flex items-center gap-1">
                    {isPending && <Loader2 className="w-3.5 h-3.5 animate-spin text-[#6A6DE6]" />}
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={onToggleCollapse}
                        className="h-7 w-7"
                        title={t("enterprise.customReport.collapsePanel")}
                    >
                        <PanelLeftClose className="w-4 h-4" />
                    </Button>
                </div>
            </div>

            {/* Scrollable content */}
            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
                {/* Templates — collapsed by default */}
                <div>
                    <button
                        type="button"
                        className="flex items-center gap-1.5 text-sm font-medium w-full text-left hover:text-[#6A6DE6] transition-colors"
                        onClick={() => setTemplatesOpen(!templatesOpen)}
                    >
                        {templatesOpen ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                        <Zap className="w-3.5 h-3.5 text-amber-500" />
                        {t("enterprise.customReport.templates.title")}
                    </button>
                    {templatesOpen && (
                        <div className="flex flex-wrap gap-1.5 mt-2 ml-5">
                            {REPORT_TEMPLATES.map((tpl) => (
                                <Button
                                    key={tpl.id}
                                    variant="outline"
                                    size="sm"
                                    className="text-xs h-7"
                                    onClick={() => {
                                        onApplyTemplate(tpl)
                                        setTemplatesOpen(false)
                                    }}
                                    disabled={isPending}
                                >
                                    {t(tpl.labelKey as never)}
                                </Button>
                            ))}
                        </div>
                    )}
                </div>

                {/* Dimensions */}
                <div className="rounded-lg bg-muted/30 p-3 space-y-2">
                    <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        {t("enterprise.customReport.dimensions")}
                    </h3>
                    <ChipSelector
                        fields={DIMENSION_FIELDS.filter((f) => f.category === "identity")}
                        selected={selectedDimensions}
                        onChange={handleDimensionChange}
                        lang={lang}
                        active="bg-[#6A6DE6]/15 text-[#6A6DE6] border-[#6A6DE6]/30"
                    />
                    <div className="border-t border-border/50" />
                    <ChipSelector
                        fields={DIMENSION_FIELDS.filter((f) => f.category === "time")}
                        selected={selectedDimensions}
                        onChange={handleDimensionChange}
                        lang={lang}
                        active="bg-[#6A6DE6]/15 text-[#6A6DE6] border-[#6A6DE6]/30"
                    />
                </div>

                {/* Measures — all flat with category headers */}
                <div className="rounded-lg bg-muted/30 p-3 space-y-2">
                    <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        {t("enterprise.customReport.measures")}
                    </h3>
                    {measuresByCategory.map(({ category, fields }) => {
                        const selectedCount = fields.filter((f) => selectedMeasures.includes(f.key)).length
                        return (
                            <div key={category}>
                                <div className="text-[10px] font-medium text-muted-foreground/60 uppercase tracking-wider mb-1">
                                    {t(`enterprise.customReport.categories.${category}`)}
                                    {selectedCount > 0 && (
                                        <span className="text-[#6A6DE6] ml-1">({selectedCount})</span>
                                    )}
                                </div>
                                <ChipSelector
                                    fields={fields}
                                    selected={selectedMeasures}
                                    onChange={onMeasuresChange}
                                    lang={lang}
                                />
                            </div>
                        )
                    })}
                </div>
            </div>
        </div>
    )
}
