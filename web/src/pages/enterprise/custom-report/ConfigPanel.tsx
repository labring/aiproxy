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
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
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
}: {
    fields: FieldDef[]
    selected: string[]
    onChange: (keys: string[]) => void
    lang: string
}) {
    return (
        <div className="flex flex-wrap gap-1.5">
            {fields.map((f) => {
                const active = selected.includes(f.key)
                return (
                    <Badge
                        key={f.key}
                        variant={active ? "default" : "outline"}
                        className={`cursor-pointer select-none transition-all text-xs px-2.5 py-1 ${
                            active
                                ? "bg-[#6A6DE6] hover:bg-[#5A5DD6] text-white border-transparent"
                                : "hover:bg-[#6A6DE6]/10 hover:border-[#6A6DE6]/30"
                        }`}
                        onClick={() => {
                            onChange(
                                active
                                    ? selected.filter((k) => k !== f.key)
                                    : [...selected, f.key],
                            )
                        }}
                    >
                        {getLabel(f.key, lang)}
                        {active && <X className="w-3 h-3 ml-1" />}
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
    onGenerate: () => void
    onApplyTemplate: (template: ReportTemplate) => void
    canGenerate: boolean
    isPending: boolean
}

export function ConfigPanel({
    collapsed,
    onToggleCollapse,
    selectedDimensions,
    onDimensionsChange,
    selectedMeasures,
    onMeasuresChange,
    onGenerate,
    onApplyTemplate,
    canGenerate,
    isPending,
}: ConfigPanelProps) {
    const { t, i18n } = useTranslation()
    const lang = i18n.language

    const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set(["requests", "computed"]))

    const toggleCategory = (cat: string) => {
        setExpandedCategories((prev) => {
            const next = new Set(prev)
            if (next.has(cat)) next.delete(cat)
            else next.add(cat)
            return next
        })
    }

    const measuresByCategory = CATEGORIES.map((cat) => ({
        category: cat,
        fields: MEASURE_FIELDS.filter((f) => f.category === cat),
    }))

    // Department dimensions are mutually exclusive; time dimensions are mutually exclusive
    const DEPT_DIMS = new Set(["department", "level1_department", "level2_department"])
    const TIME_DIMS = new Set(["time_day", "time_week", "time_hour"])

    const handleDimensionChange = (next: string[]) => {
        let result = next
        // Find which department dim was just added
        const addedDept = next.find((d) => DEPT_DIMS.has(d) && !selectedDimensions.includes(d))
        if (addedDept) {
            result = result.filter((d) => !DEPT_DIMS.has(d) || d === addedDept)
        }
        // Find which time dim was just added
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

            {/* Scrollable content */}
            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-4">
                {/* Templates */}
                <div>
                    <div className="flex items-center gap-1.5 text-sm font-medium mb-2">
                        <Zap className="w-3.5 h-3.5 text-amber-500" />
                        {t("enterprise.customReport.templates.title")}
                    </div>
                    <div className="flex flex-wrap gap-1.5">
                        {REPORT_TEMPLATES.map((tpl) => (
                            <Button
                                key={tpl.id}
                                variant="outline"
                                size="sm"
                                className="text-xs h-7"
                                onClick={() => onApplyTemplate(tpl)}
                                disabled={isPending}
                            >
                                {t(tpl.labelKey as never)}
                            </Button>
                        ))}
                    </div>
                </div>

                {/* Dimensions */}
                <Card className="border-dashed">
                    <CardHeader className="pb-2 pt-3 px-3">
                        <CardTitle className="text-sm">{t("enterprise.customReport.dimensions")}</CardTitle>
                        <CardDescription className="text-xs">{t("enterprise.customReport.dimensionsDesc")}</CardDescription>
                    </CardHeader>
                    <CardContent className="px-3 pb-3">
                        <ChipSelector
                            fields={DIMENSION_FIELDS}
                            selected={selectedDimensions}
                            onChange={handleDimensionChange}
                            lang={lang}
                        />
                    </CardContent>
                </Card>

                {/* Measures */}
                <Card className="border-dashed">
                    <CardHeader className="pb-2 pt-3 px-3">
                        <CardTitle className="text-sm">{t("enterprise.customReport.measures")}</CardTitle>
                        <CardDescription className="text-xs">{t("enterprise.customReport.measuresDesc")}</CardDescription>
                    </CardHeader>
                    <CardContent className="px-3 pb-3 space-y-1.5">
                        {measuresByCategory.map(({ category, fields }) => (
                            <div key={category}>
                                <button
                                    type="button"
                                    className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors w-full text-left py-0.5"
                                    onClick={() => toggleCategory(category)}
                                >
                                    {expandedCategories.has(category) ? (
                                        <ChevronDown className="w-3 h-3" />
                                    ) : (
                                        <ChevronRight className="w-3 h-3" />
                                    )}
                                    {t(`enterprise.customReport.categories.${category}`)}
                                    <span className="text-xs text-muted-foreground/60 ml-1">
                                        ({fields.filter((f) => selectedMeasures.includes(f.key)).length}/{fields.length})
                                    </span>
                                </button>
                                {expandedCategories.has(category) && (
                                    <div className="ml-4 mt-1">
                                        <ChipSelector
                                            fields={fields}
                                            selected={selectedMeasures}
                                            onChange={onMeasuresChange}
                                            lang={lang}
                                        />
                                    </div>
                                )}
                            </div>
                        ))}
                    </CardContent>
                </Card>
            </div>

            {/* Sticky generate button */}
            <div className="px-4 py-3 border-t">
                <Button
                    onClick={onGenerate}
                    disabled={!canGenerate || isPending}
                    className="w-full bg-[#6A6DE6] hover:bg-[#5A5DD6] text-white"
                >
                    {isPending ? (
                        <>
                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                            {t("enterprise.customReport.generating")}
                        </>
                    ) : (
                        t("enterprise.customReport.generate")
                    )}
                </Button>
            </div>
        </div>
    )
}
