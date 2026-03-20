import type { CustomReportResponse } from "@/api/enterprise"
import type { ChartType } from "./types"
import { ReportChart } from "./ReportChart"
import { ReportTable } from "./ReportTable"

export function SplitView({
    data,
    dimensions,
    measures,
    chartType,
    lang,
    sortBy,
    sortOrder,
    onSort,
}: {
    data: CustomReportResponse
    dimensions: string[]
    measures: string[]
    chartType: ChartType
    lang: string
    sortBy: string | undefined
    sortOrder: "asc" | "desc"
    onSort: (key: string, order: "asc" | "desc") => void
}) {
    return (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-0 lg:divide-x">
            {/* Chart */}
            <div className="p-4">
                <ReportChart
                    data={data}
                    dimensions={dimensions}
                    measures={measures}
                    chartType={chartType}
                    lang={lang}
                />
            </div>

            {/* Table */}
            <div className="border-t lg:border-t-0">
                <ReportTable
                    data={data}
                    dimensions={dimensions}
                    sortBy={sortBy}
                    sortOrder={sortOrder}
                    onSort={onSort}
                />
            </div>
        </div>
    )
}
