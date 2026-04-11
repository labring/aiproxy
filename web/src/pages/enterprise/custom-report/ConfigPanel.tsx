import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import {
    ChevronDown,
    ChevronRight,
    X,
    Zap,
    PanelLeftClose,
    PanelLeftOpen,
    Loader2,
    Search,
    RotateCcw,
    Save,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import {
    DIMENSION_FIELDS,
    MEASURE_FIELDS,
    CATEGORIES,
    CATEGORY_META,
    REPORT_TEMPLATES,
    DEFAULT_DIMS,
    DEFAULT_MEASURES,
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
    onReset?: () => void
    onSaveTemplate?: () => void
    isPending: boolean
    templateManagerSlot?: React.ReactNode
}

export function ConfigPanel({
    collapsed,
    onToggleCollapse,
    selectedDimensions,
    onDimensionsChange,
    selectedMeasures,
    onMeasuresChange,
    onApplyTemplate,
    onReset,
    onSaveTemplate,
    isPending,
    templateManagerSlot,
}: ConfigPanelProps) {
    const { t, i18n } = useTranslation()
    const lang = i18n.language

    const [templatesOpen, setTemplatesOpen] = useState(true)
    const [searchQuery, setSearchQuery] = useState("")
    const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set(CATEGORIES))

    const toggleCategory = (cat: string) => {
        setExpandedCategories((prev) => {
            const next = new Set(prev)
            if (next.has(cat)) next.delete(cat)
            else next.add(cat)
            return next
        })
    }

    const filteredMeasuresByCategory = useMemo(() => {
        const query = searchQuery.trim().toLowerCase()
        return CATEGORIES.map((cat) => ({
            category: cat,
            fields: MEASURE_FIELDS.filter((f) => {
                if (f.category !== cat) return false
                if (!query) return true
                const label = getLabel(f.key, lang).toLowerCase()
                return label.includes(query) || f.key.toLowerCase().includes(query)
            }),
        })).filter((g) => g.fields.length > 0)
    }, [searchQuery, lang])

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

    const isDefault = selectedDimensions.length === DEFAULT_DIMS.length
        && selectedDimensions.every((d) => DEFAULT_DIMS.includes(d))
        && selectedMeasures.length === DEFAULT_MEASURES.length
        && selectedMeasures.every((m) => DEFAULT_MEASURES.includes(m))

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
                        <div className="mt-2 ml-5 space-y-2">
                            <div className="flex flex-wrap gap-1.5">
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
                            {/* Custom template manager slot */}
                            {templateManagerSlot}
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

                {/* Measures — with search and collapsible categories */}
                <div className="rounded-lg bg-muted/30 p-3 space-y-2">
                    <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        {t("enterprise.customReport.measures")}
                        {selectedMeasures.length > 0 && (
                            <span className="text-[#6A6DE6] ml-1.5 normal-case">({selectedMeasures.length})</span>
                        )}
                    </h3>

                    {/* Search input */}
                    <div className="relative">
                        <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/50" />
                        <Input
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder={t("enterprise.customReport.searchMeasures", "Search measures...")}
                            className="h-7 text-xs pl-7 bg-background/50"
                        />
                        {searchQuery && (
                            <button
                                type="button"
                                className="absolute right-2 top-1/2 -translate-y-1/2"
                                onClick={() => setSearchQuery("")}
                            >
                                <X className="w-3 h-3 text-muted-foreground hover:text-foreground" />
                            </button>
                        )}
                    </div>

                    {/* Category sections */}
                    {filteredMeasuresByCategory.map(({ category, fields }) => {
                        const meta = CATEGORY_META[category]
                        const selectedCount = fields.filter((f) => selectedMeasures.includes(f.key)).length
                        const isExpanded = expandedCategories.has(category)

                        return (
                            <div key={category}>
                                <button
                                    type="button"
                                    className={`flex items-center gap-1.5 w-full text-left mb-1 px-1.5 py-0.5 rounded transition-colors hover:bg-muted/50`}
                                    onClick={() => toggleCategory(category)}
                                >
                                    {isExpanded
                                        ? <ChevronDown className="w-3 h-3 text-muted-foreground/60" />
                                        : <ChevronRight className="w-3 h-3 text-muted-foreground/60" />
                                    }
                                    <span className={`text-[10px] font-semibold uppercase tracking-wider ${meta?.textColor ?? "text-muted-foreground/60"}`}>
                                        {t(`enterprise.customReport.categories.${category}` as never)}
                                    </span>
                                    {selectedCount > 0 && (
                                        <Badge variant="secondary" className="text-[9px] px-1 py-0 h-3.5 ml-auto">
                                            {selectedCount}
                                        </Badge>
                                    )}
                                </button>
                                {isExpanded && (
                                    <ChipSelector
                                        fields={fields}
                                        selected={selectedMeasures}
                                        onChange={onMeasuresChange}
                                        lang={lang}
                                    />
                                )}
                            </div>
                        )
                    })}
                </div>

                {/* Selected measures summary */}
                {selectedMeasures.length > 0 && (
                    <div className="rounded-lg bg-[#6A6DE6]/5 border border-[#6A6DE6]/10 p-2">
                        <div className="text-[10px] font-medium text-muted-foreground mb-1.5">
                            {t("enterprise.customReport.selectedMeasures", "Selected")} ({selectedMeasures.length})
                        </div>
                        <div className="flex flex-wrap gap-1">
                            {selectedMeasures.map((key, idx) => (
                                <Badge
                                    key={key}
                                    variant="secondary"
                                    className="text-[10px] px-1.5 py-0 gap-1 cursor-pointer hover:bg-destructive/10"
                                    onClick={() => onMeasuresChange(selectedMeasures.filter((k) => k !== key))}
                                >
                                    <span className="text-muted-foreground/60 mr-0.5">{idx + 1}.</span>
                                    {getLabel(key, lang)}
                                    <X className="w-2.5 h-2.5" />
                                </Badge>
                            ))}
                        </div>
                    </div>
                )}
            </div>

            {/* Footer actions */}
            <div className="px-4 py-3 border-t flex gap-2">
                {onSaveTemplate && (
                    <Button
                        variant="outline"
                        size="sm"
                        className="flex-1 text-xs gap-1.5"
                        onClick={onSaveTemplate}
                        disabled={selectedDimensions.length === 0 || selectedMeasures.length === 0}
                    >
                        <Save className="w-3.5 h-3.5" />
                        {t("enterprise.customReport.saveTemplate", "Save Template")}
                    </Button>
                )}
                {onReset && !isDefault && (
                    <Button
                        variant="ghost"
                        size="sm"
                        className="text-xs gap-1.5"
                        onClick={onReset}
                    >
                        <RotateCcw className="w-3.5 h-3.5" />
                        {t("enterprise.customReport.reset", "Reset")}
                    </Button>
                )}
            </div>
        </div>
    )
}
