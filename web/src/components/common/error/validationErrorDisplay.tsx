import { FieldErrors } from 'react-hook-form'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { AlertCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface ValidationErrorDisplayProps {
    errors: FieldErrors<Record<string, unknown>>
    title?: string
    className?: string
}

export function ValidationErrorDisplay({
    errors,
    title,
    className = ""
}: ValidationErrorDisplayProps) {
    const { t } = useTranslation()

    // 如果没有错误，不显示组件
    if (!errors || Object.keys(errors).length === 0) {
        return null
    }

    // 递归处理嵌套错误对象
    const flattenErrors = (obj: Record<string, unknown>, prefix = ''): string[] => {
        const messages: string[] = []

        for (const [key, value] of Object.entries(obj)) {
            const fullKey = prefix ? `${prefix}.${key}` : key

            if (value && typeof value === 'object') {
                if ('message' in value && typeof value.message === 'string') {
                    // 这是一个错误对象
                    messages.push(`${fullKey}: ${value.message}`)
                } else {
                    // 递归处理嵌套对象
                    messages.push(...flattenErrors(value as Record<string, unknown>, fullKey))
                }
            }
        }

        return messages
    }

    const errorMessages = flattenErrors(errors)

    return (
        <Alert variant="destructive" className={className}>
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>
                {title || t('common.validation.error.title')}
            </AlertTitle>
            <AlertDescription>
                <div className="mt-2 space-y-1">
                    {errorMessages.map((message, index) => (
                        <div key={index} className="text-sm">
                            • {message}
                        </div>
                    ))}
                </div>
            </AlertDescription>
        </Alert>
    )
}