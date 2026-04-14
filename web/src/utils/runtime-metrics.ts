import type { RuntimeChannelModelMetric, RuntimeMetricsResponse } from '@/types/runtime-metrics'

export const getChannelModelMetric = (
    runtimeMetrics: RuntimeMetricsResponse | undefined,
    channelId: number | string | undefined,
    model: string,
): RuntimeChannelModelMetric | undefined => {
    if (!runtimeMetrics || channelId === undefined || channelId === null || !model) {
        return undefined
    }

    return runtimeMetrics.channel_models?.[String(channelId)]?.[model]
}

export const isChannelModelTemporarilyExcluded = (
    runtimeMetrics: RuntimeMetricsResponse | undefined,
    channelId: number | string | undefined,
    model: string,
): boolean => {
    return Boolean(getChannelModelMetric(runtimeMetrics, channelId, model)?.banned)
}

export const getTemporarilyExcludedModels = (
    runtimeMetrics: RuntimeMetricsResponse | undefined,
    channelId: number | string | undefined,
    models: string[],
): string[] => {
    return models.filter((model) => isChannelModelTemporarilyExcluded(runtimeMetrics, channelId, model))
}
