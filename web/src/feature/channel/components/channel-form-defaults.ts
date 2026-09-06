import type { Channel } from '@/types/channel'
import { DEFAULT_PRIORITY } from '@/types/channel'
import { getChannelPriority } from '@/utils/channel'

export const getChannelFormDefaults = (
    channel: Channel | null | undefined
) => channel
    ? {
        type: channel.type,
        name: channel.name,
        remark: channel.remark || '',
        key: channel.key,
        base_url: channel.base_url,
        proxy_url: channel.proxy_url,
        models: channel.models || [],
        model_mapping: channel.model_mapping || {},
        sets: channel.sets || [],
        priority: getChannelPriority(channel.priority),
        backup_only: channel.backup_only ?? false,
        skip_tls_verify: channel.skip_tls_verify ?? false,
        enabled_no_permission_ban: channel.enabled_no_permission_ban ?? false,
        warn_error_rate: channel.warn_error_rate,
        max_error_rate: channel.max_error_rate !== undefined && channel.max_error_rate > 0
            ? channel.max_error_rate
            : undefined,
        configs_text: channel.configs ? JSON.stringify(channel.configs, null, 2) : ''
    }
    : {
        type: 0,
        name: '',
        remark: '',
        key: '',
        base_url: '',
        proxy_url: '',
        models: [],
        model_mapping: {},
        sets: [],
        priority: DEFAULT_PRIORITY,
        backup_only: false,
        skip_tls_verify: false,
        enabled_no_permission_ban: false,
        warn_error_rate: undefined,
        max_error_rate: undefined,
        configs_text: ''
    }
