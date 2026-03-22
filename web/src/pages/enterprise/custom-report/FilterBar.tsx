import { useState, useEffect, useRef, type KeyboardEvent } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import { useQuery } from "@tanstack/react-query"
import { type DateRange } from "react-day-picker"
import {
    X,
    Filter,
    Check,
    ChevronsUpDown,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import { enterpriseApi } from "@/api/enterprise"
import { modelApi } from "@/api/model"
import type { TimeRange } from "@/lib/enterprise"
import useAuthStore from "@/store/auth"

// ─── AutocompleteTagInput ────────────────────────────────────────────────────

function AutocompleteTagInput({
    values,
    onChange,
    placeholder,
    suggestions,
}: {
    values: string[]
    onChange: (vals: string[]) => void
    placeholder: string
    suggestions: string[]
}) {
    const [input, setInput] = useState("")
    const [showDropdown, setShowDropdown] = useState(false)
    const [highlightIdx, setHighlightIdx] = useState(-1)
    const containerRef = useRef<HTMLDivElement>(null)

    const filtered = input.trim()
        ? suggestions.filter(
              (s) => s.toLowerCase().includes(input.trim().toLowerCase()) && !values.includes(s),
          )
        : []

    useEffect(() => { setHighlightIdx(-1) }, [filtered.length])

    const addTag = (tag: string) => {
        const trimmed = tag.trim()
        if (trimmed && !values.includes(trimmed)) {
            onChange([...values, trimmed])
        }
        setInput("")
        setShowDropdown(false)
    }

    const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "ArrowDown") {
            e.preventDefault()
            setHighlightIdx((prev) => Math.min(prev + 1, filtered.length - 1))
        } else if (e.key === "ArrowUp") {
            e.preventDefault()
            setHighlightIdx((prev) => Math.max(prev - 1, 0))
        } else if (e.key === "Enter" || e.key === ",") {
            e.preventDefault()
            if (highlightIdx >= 0 && highlightIdx < filtered.length) {
                addTag(filtered[highlightIdx])
            } else {
                addTag(input)
            }
        } else if (e.key === "Escape") {
            setShowDropdown(false)
        } else if (e.key === "Backspace" && input === "" && values.length > 0) {
            onChange(values.slice(0, -1))
        }
    }

    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setShowDropdown(false)
            }
        }
        document.addEventListener("mousedown", handler)
        return () => document.removeEventListener("mousedown", handler)
    }, [])

    return (
        <div ref={containerRef} className="relative">
            <div className="flex flex-wrap items-center gap-1.5 p-1.5 border rounded-md min-h-[36px] bg-background">
                {values.map((v) => (
                    <Badge key={v} variant="secondary" className="text-xs gap-1 px-2 py-0.5">
                        {v}
                        <X
                            className="w-3 h-3 cursor-pointer hover:text-destructive"
                            onPointerDown={(e) => {
                                e.preventDefault()
                                e.stopPropagation()
                                onChange(values.filter((x) => x !== v))
                            }}
                        />
                    </Badge>
                ))}
                <Input
                    value={input}
                    onChange={(e) => {
                        setInput(e.target.value)
                        setShowDropdown(true)
                    }}
                    onKeyDown={handleKeyDown}
                    onFocus={() => setShowDropdown(true)}
                    onBlur={() => {
                        setTimeout(() => {
                            if (input.trim()) addTag(input)
                        }, 150)
                    }}
                    placeholder={values.length === 0 ? placeholder : ""}
                    className="border-0 shadow-none h-7 min-w-[100px] flex-1 focus-visible:ring-0 p-0 px-1"
                />
            </div>
            {showDropdown && filtered.length > 0 && (
                <div className="absolute z-20 left-0 right-0 mt-1 bg-popover border rounded-md shadow-md max-h-[180px] overflow-y-auto py-1">
                    {filtered.slice(0, 20).map((item, idx) => (
                        <button
                            key={item}
                            type="button"
                            className={`w-full text-left text-sm px-3 py-1.5 transition-colors ${
                                idx === highlightIdx ? "bg-[#6A6DE6]/10 text-[#6A6DE6]" : "hover:bg-muted/50"
                            }`}
                            onMouseDown={(e) => {
                                e.preventDefault()
                                addTag(item)
                            }}
                        >
                            {item}
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
}

// ─── DepartmentHierarchyFilter ──────────────────────────────────────────────

function DepartmentHierarchyFilter({
    selected,
    onChange,
    t,
}: {
    selected: string[]
    onChange: (ids: string[]) => void
    t: TFunction
}) {
    const [open, setOpen] = useState(false)
    const [level1Id, setLevel1Id] = useState<string>("")

    const { data: deptData } = useQuery({
        queryKey: ["enterprise", "department-levels", level1Id || "all"],
        queryFn: () => enterpriseApi.getDepartmentLevels(level1Id || undefined),
    })

    const level1Depts = deptData?.level1_departments ?? []
    const level2Depts = deptData?.level2_departments ?? []

    const toggleDept = (id: string) => {
        onChange(
            selected.includes(id)
                ? selected.filter((d) => d !== id)
                : [...selected, id],
        )
    }

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button variant="outline" size="sm" className="gap-1.5 h-8 justify-start">
                    <Filter className="w-3.5 h-3.5" />
                    {t("enterprise.customReport.filterDepartments")}
                    {selected.length > 0 && (
                        <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-xs">
                            {selected.length}
                        </Badge>
                    )}
                    <ChevronsUpDown className="w-3.5 h-3.5 opacity-50 ml-1" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[320px] p-0" align="start">
                <div className="px-3 py-2 border-b">
                    <Select value={level1Id} onValueChange={setLevel1Id}>
                        <SelectTrigger className="h-8 text-xs">
                            <SelectValue placeholder={t("enterprise.customReport.allLevel1Departments")} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t("enterprise.customReport.allLevel1Departments")}</SelectItem>
                            {level1Depts.map((d) => (
                                <SelectItem key={d.department_id} value={d.department_id}>
                                    {d.name}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
                <div className="flex items-center justify-between px-3 py-2 border-b">
                    <span className="text-xs text-muted-foreground">
                        {selected.length > 0
                            ? `${selected.length} ${t("enterprise.customReport.selected")}`
                            : t("enterprise.customReport.allDepartments")}
                    </span>
                    {selected.length > 0 && (
                        <Button variant="ghost" size="sm" className="h-6 text-xs px-2" onClick={() => onChange([])}>
                            {t("enterprise.customReport.clearSelection")}
                        </Button>
                    )}
                </div>
                <div className="max-h-[240px] overflow-y-auto py-1">
                    {(level1Id && level1Id !== "all" ? level2Depts : level1Depts).map((dept) => {
                        const isSelected = selected.includes(dept.department_id)
                        return (
                            <button
                                key={dept.department_id}
                                type="button"
                                className="w-full flex items-center gap-2 px-3 py-1.5 text-sm hover:bg-muted/50 transition-colors text-left"
                                onClick={() => toggleDept(dept.department_id)}
                            >
                                <div className={`w-4 h-4 rounded border flex items-center justify-center ${
                                    isSelected ? "bg-[#6A6DE6] border-[#6A6DE6]" : "border-muted-foreground/30"
                                }`}>
                                    {isSelected && <Check className="w-3 h-3 text-white" />}
                                </div>
                                <span className="truncate">{dept.name}</span>
                                <span className="text-xs text-muted-foreground ml-auto">{dept.member_count}</span>
                            </button>
                        )
                    })}
                    {level1Depts.length === 0 && level2Depts.length === 0 && (
                        <div className="text-center text-muted-foreground text-sm py-4">
                            {t("enterprise.customReport.allDepartments")}
                        </div>
                    )}
                </div>
            </PopoverContent>
        </Popover>
    )
}

// ─── FilterBar ──────────────────────────────────────────────────────────────

export interface FilterBarProps {
    timeRange: TimeRange
    onTimeRangeChange: (range: TimeRange) => void
    customDateRange: DateRange | undefined
    onCustomDateRangeChange: (range: DateRange | undefined) => void
    filterDepts: string[]
    onFilterDeptsChange: (ids: string[]) => void
    filterModels: string[]
    onFilterModelsChange: (models: string[]) => void
    filterUsers: string[]
    onFilterUsersChange: (users: string[]) => void
}

export function FilterBar({
    timeRange,
    onTimeRangeChange,
    customDateRange,
    onCustomDateRangeChange,
    filterDepts,
    onFilterDeptsChange,
    filterModels,
    onFilterModelsChange,
    filterUsers,
    onFilterUsersChange,
}: FilterBarProps) {
    const { t } = useTranslation()
    const enterpriseUser = useAuthStore(s => s.enterpriseUser)
    // /api/model_configs/all requires AdminAuth — only call it for Admin Key users
    const isAdminKeyUser = !enterpriseUser

    // Fetch model names for autocomplete (admin only)
    const { data: modelConfigs } = useQuery({
        queryKey: ["models", "all"],
        queryFn: () => modelApi.getModels(),
        staleTime: 5 * 60 * 1000,
        enabled: isAdminKeyUser,
    })
    const modelNames = (modelConfigs ?? []).map((m) => m.model)

    // Fetch user names for autocomplete
    const { data: usersData } = useQuery({
        queryKey: ["enterprise", "feishu-users-all"],
        queryFn: () => enterpriseApi.getFeishuUsers(1, 500),
        staleTime: 5 * 60 * 1000,
    })
    const userNames = (usersData?.users ?? []).map((u) => u.name)

    return (
        <div className="flex flex-wrap items-end gap-3">
            {/* Time Range */}
            <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">{t("enterprise.dashboard.timeRange")}</label>
                <Select value={timeRange} onValueChange={(v) => onTimeRangeChange(v as TimeRange)}>
                    <SelectTrigger className="h-8 text-xs w-[130px]">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="7d">{t("enterprise.dashboard.last7Days")}</SelectItem>
                        <SelectItem value="30d">{t("enterprise.dashboard.last30Days")}</SelectItem>
                        <SelectItem value="month">{t("enterprise.dashboard.thisMonth")}</SelectItem>
                        <SelectItem value="last_week">{t("enterprise.dashboard.lastWeek")}</SelectItem>
                        <SelectItem value="last_month">{t("enterprise.dashboard.lastMonth")}</SelectItem>
                        <SelectItem value="custom">{t("enterprise.dashboard.customRange")}</SelectItem>
                    </SelectContent>
                </Select>
            </div>

            {/* Custom date range */}
            {timeRange === "custom" && (
                <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">{t("enterprise.dashboard.selectDateRange")}</label>
                    <DateRangePicker
                        value={customDateRange}
                        onChange={onCustomDateRangeChange}
                        placeholder={t("enterprise.dashboard.selectDateRange")}
                        className="h-8 text-xs"
                    />
                </div>
            )}

            {/* Department filter */}
            <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">{t("enterprise.customReport.filterDepartments")}</label>
                <DepartmentHierarchyFilter
                    selected={filterDepts}
                    onChange={onFilterDeptsChange}
                    t={t}
                />
            </div>

            {/* Model filter */}
            <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">{t("enterprise.customReport.filterModels")}</label>
                <Popover>
                    <PopoverTrigger asChild>
                        <Button variant="outline" size="sm" className="gap-1.5 h-8 justify-start">
                            <Filter className="w-3.5 h-3.5" />
                            {filterModels.length > 0 ? (
                                <Badge variant="secondary" className="px-1.5 py-0 text-xs">
                                    {filterModels.length}
                                </Badge>
                            ) : (
                                <span className="text-xs text-muted-foreground">{t("enterprise.customReport.allModels")}</span>
                            )}
                        </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-[300px] p-3" align="start">
                        <AutocompleteTagInput
                            values={filterModels}
                            onChange={onFilterModelsChange}
                            placeholder={t("enterprise.customReport.addFilter")}
                            suggestions={modelNames}
                        />
                    </PopoverContent>
                </Popover>
            </div>

            {/* User filter */}
            <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">{t("enterprise.customReport.filterUsers")}</label>
                <Popover>
                    <PopoverTrigger asChild>
                        <Button variant="outline" size="sm" className="gap-1.5 h-8 justify-start">
                            <Filter className="w-3.5 h-3.5" />
                            {filterUsers.length > 0 ? (
                                <Badge variant="secondary" className="px-1.5 py-0 text-xs">
                                    {filterUsers.length}
                                </Badge>
                            ) : (
                                <span className="text-xs text-muted-foreground">{t("enterprise.customReport.allUsers")}</span>
                            )}
                        </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-[300px] p-3" align="start">
                        <AutocompleteTagInput
                            values={filterUsers}
                            onChange={onFilterUsersChange}
                            placeholder={t("enterprise.customReport.addFilter")}
                            suggestions={userNames}
                        />
                    </PopoverContent>
                </Popover>
            </div>

        </div>
    )
}
