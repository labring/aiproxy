import React, { Suspense } from 'react'
import { useTheme } from '@/handler/ThemeContext'
import { Skeleton } from '@/components/ui/skeleton'

// 动态导入 react-json-view 以避免 SSR 问题
const ReactJson = React.lazy(() => import('react-json-view'))

interface JsonViewerProps {
    src: unknown
    name?: string | false
    collapsed?: boolean | number
    enableClipboard?: boolean
    displayDataTypes?: boolean
    displayObjectSize?: boolean
    collapseStringsAfterLength?: number
}

export function JsonViewer({
    src,
    name = false,
    collapsed = 2,
    enableClipboard = true,
    displayDataTypes = false,
    displayObjectSize = false,
    collapseStringsAfterLength = 100,
}: JsonViewerProps) {
    const { theme } = useTheme()

    let parsedSrc = src

    // 尝试解析字符串形式的JSON
    if (typeof src === 'string') {
        try {
            parsedSrc = JSON.parse(src)
        } catch {
            // 如果解析失败，显示原始字符串
            parsedSrc = { value: src }
        }
    }

    return (
        <div className="json-viewer-container">
            <Suspense fallback={<Skeleton className="h-20 w-full" />}>
                <ReactJson
                    src={parsedSrc as object}
                    theme={theme === 'dark' ? 'tomorrow' : 'rjv-default'}
                    name={name}
                    collapsed={collapsed}
                    enableClipboard={enableClipboard}
                    displayDataTypes={displayDataTypes}
                    displayObjectSize={displayObjectSize}
                    collapseStringsAfterLength={collapseStringsAfterLength}
                    style={{
                        backgroundColor: 'transparent',
                        fontSize: '13px',
                        fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
                        padding: '8px',
                        borderRadius: '6px',
                        border: '1px solid hsl(var(--border))',
                    }}
                />
            </Suspense>
        </div>
    )
} 