import type { TFunction } from "i18next"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { CustomReportResponse } from "@/api/enterprise"
import { getLabel, formatCellValue, sortDimKeys } from "./types"

export function PivotTable({
    data,
    dim1,
    dim2,
    measures,
    selectedMeasure,
    onMeasureChange,
    lang,
    t,
}: {
    data: CustomReportResponse
    dim1: string
    dim2: string
    measures: string[]
    selectedMeasure: string
    onMeasureChange: (m: string) => void
    lang: string
    t: TFunction
}) {
    // Build pivot map: dim1Value -> dim2Value -> measure value
    const pivotMap = new Map<string, Map<string, unknown>>()
    const dim2Values = new Set<string>()

    for (const row of data.rows) {
        const d1 = String(row[dim1] ?? "")
        const d2 = String(row[dim2] ?? "")
        dim2Values.add(d2)
        if (!pivotMap.has(d1)) pivotMap.set(d1, new Map())
        pivotMap.get(d1)!.set(d2, row[selectedMeasure])
    }

    const dim1Keys = sortDimKeys(Array.from(pivotMap.keys()), dim1)
    const dim2Keys = sortDimKeys(Array.from(dim2Values), dim2)

    return (
        <div>
            {measures.length > 1 && (
                <div className="flex items-center gap-2 p-4 pb-2">
                    <span className="text-sm text-muted-foreground">{t("enterprise.customReport.pivotMeasure")}:</span>
                    <Select value={selectedMeasure} onValueChange={onMeasureChange}>
                        <SelectTrigger className="w-[200px] h-8">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {measures.map((m) => (
                                <SelectItem key={m} value={m}>{getLabel(m, lang)}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            )}
            <div className="overflow-x-auto">
                <table className="w-full text-sm">
                    <thead>
                        <tr className="border-b bg-muted/40">
                            <th className="px-4 py-3 text-left font-medium text-muted-foreground whitespace-nowrap sticky left-0 bg-muted/40 z-10">
                                {getLabel(dim1, lang)} \ {getLabel(dim2, lang)}
                            </th>
                            {dim2Keys.map((d2) => (
                                <th key={d2} className="px-4 py-3 text-right font-medium text-muted-foreground whitespace-nowrap">
                                    {formatCellValue(dim2, d2)}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {dim1Keys.map((d1) => (
                            <tr key={d1} className="border-b last:border-0 hover:bg-muted/20 transition-colors">
                                <td className="px-4 py-2.5 font-medium whitespace-nowrap sticky left-0 bg-background z-10">
                                    {formatCellValue(dim1, d1)}
                                </td>
                                {dim2Keys.map((d2) => (
                                    <td key={d2} className="px-4 py-2.5 text-right whitespace-nowrap">
                                        {formatCellValue(selectedMeasure, pivotMap.get(d1)?.get(d2))}
                                    </td>
                                ))}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
