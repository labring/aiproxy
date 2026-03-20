import { useTranslation } from "react-i18next"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import { BarChart3 } from "lucide-react"
import { type ChartType, CHART_TYPE_OPTIONS } from "./types"

export function ChartTypePicker({
    value,
    onChange,
}: {
    value: ChartType
    onChange: (type: ChartType) => void
}) {
    const { t } = useTranslation()

    const current = CHART_TYPE_OPTIONS.find((o) => o.type === value) ?? CHART_TYPE_OPTIONS[0]

    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button variant="outline" size="sm" className="gap-1.5 text-xs">
                    <BarChart3 className="w-3.5 h-3.5" />
                    {t(current.labelKey as never)}
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-[200px] p-2">
                <div className="grid grid-cols-3 gap-1">
                    {CHART_TYPE_OPTIONS.map((opt) => (
                        <button
                            key={opt.type}
                            type="button"
                            className={`flex flex-col items-center gap-1 p-2 rounded-md text-xs transition-colors ${
                                value === opt.type
                                    ? "bg-[#6A6DE6] text-white"
                                    : "hover:bg-muted"
                            }`}
                            onClick={() => onChange(opt.type)}
                        >
                            <span className="text-base">{opt.icon}</span>
                            <span className="truncate w-full text-center text-[10px]">
                                {t(opt.labelKey as never)}
                            </span>
                        </button>
                    ))}
                </div>
            </DropdownMenuContent>
        </DropdownMenu>
    )
}
