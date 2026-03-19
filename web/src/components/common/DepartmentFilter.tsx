import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Filter, Check, ChevronsUpDown } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { enterpriseApi } from "@/api/enterprise"

interface DepartmentFilterProps {
    selected: string[]
    onChange: (ids: string[]) => void
    timeRange: { start: number; end: number }
    disabled?: boolean
}

export function DepartmentFilter({ selected, onChange, timeRange, disabled }: DepartmentFilterProps) {
    const { t, i18n } = useTranslation()
    const lang = i18n.language
    const [open, setOpen] = useState(false)

    const { data: deptData } = useQuery({
        queryKey: ["enterprise", "departments", timeRange.start, timeRange.end],
        queryFn: () => enterpriseApi.getDepartmentSummary(timeRange.start, timeRange.end),
    })

    const departments = deptData?.departments ?? []

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
                <Button variant="outline" size="sm" className="gap-1.5" disabled={disabled}>
                    <Filter className="w-3.5 h-3.5" />
                    {t("enterprise.customReport.filterDepartments")}
                    {selected.length > 0 && (
                        <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-xs">
                            {selected.length}
                        </Badge>
                    )}
                    <ChevronsUpDown className="w-3.5 h-3.5 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[280px] p-0" align="start">
                <div className="flex items-center justify-between px-3 py-2 border-b">
                    <span className="text-xs text-muted-foreground">
                        {selected.length > 0
                            ? `${selected.length} ${lang.startsWith("zh") ? "个已选" : "selected"}`
                            : t("enterprise.customReport.allDepartments")}
                    </span>
                    <div className="flex gap-1">
                        {selected.length > 0 ? (
                            <Button variant="ghost" size="sm" className="h-6 text-xs px-2" onClick={() => onChange([])}>
                                {t("enterprise.customReport.clearSelection")}
                            </Button>
                        ) : (
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-6 text-xs px-2"
                                onClick={() => onChange(departments.map((d) => d.department_id))}
                            >
                                {t("enterprise.customReport.selectAll")}
                            </Button>
                        )}
                    </div>
                </div>
                <div className="max-h-[240px] overflow-y-auto py-1">
                    {departments.map((dept) => {
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
                                <span className="truncate">{dept.department_name || dept.department_id}</span>
                                <span className="text-xs text-muted-foreground ml-auto">{dept.member_count}</span>
                            </button>
                        )
                    })}
                    {departments.length === 0 && (
                        <div className="text-center text-muted-foreground text-sm py-4">
                            {t("enterprise.customReport.allDepartments")}
                        </div>
                    )}
                </div>
            </PopoverContent>
        </Popover>
    )
}
