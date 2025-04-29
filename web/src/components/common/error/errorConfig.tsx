import {
    AlertCircle,
    WifiOff,
    Clock,
    ShieldAlert,
    ServerOff,
    Bug,
    Ban,
    FileWarning
} from "lucide-react"
import { ErrorType, ErrorTypeValue } from './errorTypes'
import { ReactElement } from 'react'

// 错误配置类型
export interface ErrorConfig {
    icon: ReactElement
    titleKey: string
    descriptionKey: string
}

// 错误配置映射
export const errorConfigs: Record<ErrorTypeValue, ErrorConfig> = {
    [ErrorType.NETWORK]: {
        icon: <WifiOff className="h-5 w-5" />,
        titleKey: 'error.network.title',
        descriptionKey: 'error.network.description'
    },
    [ErrorType.TIMEOUT]: {
        icon: <Clock className="h-5 w-5" />,
        titleKey: 'error.timeout.title',
        descriptionKey: 'error.timeout.description'
    },
    [ErrorType.FORBIDDEN]: {
        icon: <Ban className="h-5 w-5" />,
        titleKey: 'error.forbidden.title',
        descriptionKey: 'error.forbidden.description'
    },
    [ErrorType.UNAUTHORIZED]: {
        icon: <ShieldAlert className="h-5 w-5" />,
        titleKey: 'error.unauthorized.title',
        descriptionKey: 'error.unauthorized.description'
    },
    [ErrorType.SERVER]: {
        icon: <ServerOff className="h-5 w-5" />,
        titleKey: 'error.server.title',
        descriptionKey: 'error.server.description'
    },
    [ErrorType.CLIENT]: {
        icon: <FileWarning className="h-5 w-5" />,
        titleKey: 'error.client.title',
        descriptionKey: 'error.client.description'
    },
    [ErrorType.VALIDATION]: {
        icon: <Bug className="h-5 w-5" />,
        titleKey: 'error.validation.title',
        descriptionKey: 'error.validation.description'
    },
    [ErrorType.UNKNOWN]: {
        icon: <AlertCircle className="h-5 w-5" />,
        titleKey: 'error.unknown.title',
        descriptionKey: 'error.unknown.description'
    }
}