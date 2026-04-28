import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'

interface TimezoneInputProps {
    value: string
    onChange: (value: string) => void
    disabled?: boolean
    className?: string
}

export function TimezoneInput({
    value,
    onChange,
    disabled = false,
    className = "h-9 w-full sm:w-52"
}: TimezoneInputProps) {
    const { t } = useTranslation()

    return (
        <Input
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder={t('common.timezonePlaceholder')}
            disabled={disabled}
            className={className}
        />
    )
}
