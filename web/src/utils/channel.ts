import { DEFAULT_PRIORITY } from '@/types/channel'

export const DEFAULT_CHANNEL_SET = 'default'
export const MAX_CHANNEL_PRIORITY = 1000000

// Keep effective values aligned with Channel.GetPriority and Channel.GetSets.
export const getChannelPriority = (priority?: number | null) =>
    Math.min(priority || DEFAULT_PRIORITY, MAX_CHANNEL_PRIORITY)

export const getChannelSets = (sets?: readonly string[] | null): readonly string[] =>
    sets?.length ? sets : [DEFAULT_CHANNEL_SET]
