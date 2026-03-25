import { get } from './index'
import { RuntimeMetricsResponse } from '@/types/runtime-metrics'

export const monitorApi = {
    getRuntimeMetrics: async (): Promise<RuntimeMetricsResponse> => {
        const response = await get<RuntimeMetricsResponse>('monitor/runtime_metrics')
        return response
    },
}
