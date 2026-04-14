import { useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import type { DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import { Download, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { logApi } from '@/api/log'
import { DateRangePicker } from '@/components/common/DateRangePicker'
import { TimezoneInput } from '@/components/common/TimezoneInput'
import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { LogExportOrder, LogExportParams, LogFilters } from '@/types/log'
import {
    DEFAULT_TIMEZONE,
    unixMsToZonedDate,
    zonedBoundaryToUnixMs,
} from '@/utils/timezone'

type LogExportScope = 'global' | 'group'

interface LogExportDialogProps {
    scope: LogExportScope
    currentFilters?: LogFilters
    groupId?: string
    disabled?: boolean
}

interface ExportFormState {
    model: string
    tokenName: string
    channel: string
    requestId: string
    upstreamId: string
    ip: string
    user: string
    code: string
    codeType: 'all' | 'success' | 'error'
    order: LogExportOrder
    timezone: string
    dateRange?: DateRange
    includeDetail: boolean
    includeChannel: boolean
    includeRetryAt: boolean
    limitEntries: boolean
    maxEntries: string
    chunkInterval: string
}

const getDefaultDateRange = (): DateRange => {
    const today = new Date()
    const oneDayAgo = new Date()
    oneDayAgo.setDate(today.getDate() - 1)
    return { from: oneDayAgo, to: today }
}

const trimToUndefined = (value: string) => {
    const trimmed = value.trim()
    return trimmed || undefined
}

const parseOptionalInteger = (value: string, label: string) => {
    const trimmed = value.trim()
    if (!trimmed) {
        return { value: undefined }
    }

    const parsed = Number.parseInt(trimmed, 10)
    if (Number.isNaN(parsed)) {
        return { error: label }
    }

    return { value: parsed }
}

const buildInitialState = (currentFilters?: LogFilters): ExportFormState => {
    const timezone = currentFilters?.timezone?.trim() || DEFAULT_TIMEZONE
    const dateRange =
        currentFilters?.start_timestamp || currentFilters?.end_timestamp
            ? {
                from: currentFilters?.start_timestamp
                    ? unixMsToZonedDate(currentFilters.start_timestamp, timezone)
                    : undefined,
                to: currentFilters?.end_timestamp
                    ? unixMsToZonedDate(currentFilters.end_timestamp, timezone)
                    : undefined,
            }
            : getDefaultDateRange()

    return {
        model: currentFilters?.model || '',
        tokenName: currentFilters?.token_name || '',
        channel: typeof currentFilters?.channel === 'number' ? String(currentFilters.channel) : '',
        requestId: '',
        upstreamId: '',
        ip: '',
        user: '',
        code: '',
        codeType: currentFilters?.code_type || 'all',
        order: 'desc',
        timezone,
        dateRange,
        includeDetail: false,
        includeChannel: false,
        includeRetryAt: false,
        limitEntries: false,
        maxEntries: '10000',
        chunkInterval: '30m',
    }
}

const FormField = ({
    label,
    children,
}: {
    label: string
    children: ReactNode
}) => (
    <div className="space-y-2">
        <Label>{label}</Label>
        {children}
    </div>
)

const ToggleRow = ({
    label,
    description,
    checked,
    onCheckedChange,
}: {
    label: string
    description: string
    checked: boolean
    onCheckedChange: (checked: boolean) => void
}) => (
    <div className="flex items-start justify-between gap-4 rounded-md border border-border/70 bg-muted/30 px-3 py-3">
        <div className="space-y-1">
            <Label>{label}</Label>
            <p className="text-xs text-muted-foreground">{description}</p>
        </div>
        <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </div>
)

export function LogExportDialog({
    scope,
    currentFilters,
    groupId,
    disabled = false,
}: LogExportDialogProps) {
    const { t } = useTranslation()
    const [open, setOpen] = useState(false)
    const [exporting, setExporting] = useState(false)
    const [form, setForm] = useState<ExportFormState>(() => buildInitialState(currentFilters))

    useEffect(() => {
        if (open) {
            setForm(buildInitialState(currentFilters))
        }
    }, [open, currentFilters])

    const keywordNotice = useMemo(
        () => Boolean(currentFilters?.keyword?.trim()),
        [currentFilters?.keyword]
    )

    const title =
        scope === 'group'
            ? t('log.export.groupTitle', { group: groupId || '-' })
            : t('log.export.globalTitle')

    const handleExport = async () => {
        const timezone = form.timezone.trim() || DEFAULT_TIMEZONE
        const chunkInterval = form.chunkInterval.trim()
        if (!chunkInterval) {
            toast.error(t('log.export.validation.chunkInterval'))
            return
        }

        const parsedCode = parseOptionalInteger(form.code, t('log.export.validation.statusCode'))
        if (parsedCode.error) {
            toast.error(parsedCode.error)
            return
        }

        const parsedChannel = parseOptionalInteger(form.channel, t('log.export.validation.channel'))
        if (parsedChannel.error) {
            toast.error(parsedChannel.error)
            return
        }

        let maxEntries = 0
        if (form.limitEntries) {
            const parsedMaxEntries = Number.parseInt(form.maxEntries.trim(), 10)
            if (Number.isNaN(parsedMaxEntries) || parsedMaxEntries <= 0) {
                toast.error(t('log.export.validation.maxEntries'))
                return
            }
            maxEntries = parsedMaxEntries
        }

        const payload: LogExportParams = {
            model: trimToUndefined(form.model),
            token_name: scope === 'group' ? trimToUndefined(form.tokenName) : undefined,
            channel: scope === 'global' ? parsedChannel.value : undefined,
            request_id: trimToUndefined(form.requestId),
            upstream_id: trimToUndefined(form.upstreamId),
            ip: trimToUndefined(form.ip),
            user: trimToUndefined(form.user),
            code_type: form.codeType,
            code: parsedCode.value,
            timezone,
            include_detail: form.includeDetail,
            include_channel: scope === 'group' ? form.includeChannel : undefined,
            include_retry_at: scope === 'group' ? form.includeRetryAt : undefined,
            max_entries: maxEntries,
            chunk_interval: chunkInterval,
            order: form.order,
        }

        if (form.dateRange?.from) {
            payload.start_timestamp = zonedBoundaryToUnixMs(form.dateRange.from, timezone, false)
        }
        if (form.dateRange?.to) {
            payload.end_timestamp = zonedBoundaryToUnixMs(form.dateRange.to, timezone, true)
        }

        setExporting(true)
        try {
            if (scope === 'group' && groupId) {
                await logApi.exportGroupLogs(groupId, payload)
            } else {
                await logApi.exportLogs(payload)
            }
            toast.success(t('log.export.success'))
            setOpen(false)
        } catch (error) {
            const message = error instanceof Error ? error.message : t('log.export.failed')
            toast.error(message)
        } finally {
            setExporting(false)
        }
    }

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                <Button variant="outline" size="sm" disabled={disabled}>
                    <Download className="mr-2 h-4 w-4" />
                    {t('log.export.trigger')}
                </Button>
            </DialogTrigger>

            <DialogContent className="sm:max-w-3xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{title}</DialogTitle>
                    <DialogDescription>
                        {scope === 'group'
                            ? t('log.export.groupDescription')
                            : t('log.export.globalDescription')}
                        {keywordNotice ? ` ${t('log.export.keywordNotice')}` : ''}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-5">
                    <section className="space-y-3">
                        <div>
                            <h3 className="text-sm font-semibold">{t('log.export.sections.time')}</h3>
                            <p className="text-xs text-muted-foreground">
                                {t('log.export.sections.timeDescription')}
                            </p>
                        </div>

                        <div className="grid gap-4 md:grid-cols-2">
                            <FormField label={t('log.export.fields.dateRange')}>
                                <DateRangePicker
                                    value={form.dateRange}
                                    onChange={(dateRange) => setForm(prev => ({ ...prev, dateRange }))}
                                    placeholder={t('log.filters.dateRangePlaceholder')}
                                    className="h-9"
                                    disabled={exporting}
                                />
                            </FormField>

                            <FormField label={t('log.export.fields.timezone')}>
                                <TimezoneInput
                                    value={form.timezone}
                                    onChange={(timezone) => setForm(prev => ({ ...prev, timezone }))}
                                    disabled={exporting}
                                    className="h-9 w-full"
                                />
                            </FormField>

                            <FormField label={t('log.export.fields.order')}>
                                <Select
                                    value={form.order}
                                    onValueChange={(order: LogExportOrder) => setForm(prev => ({ ...prev, order }))}
                                    disabled={exporting}
                                >
                                    <SelectTrigger className="h-9">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="desc">{t('log.export.order.createdDesc')}</SelectItem>
                                        <SelectItem value="asc">{t('log.export.order.createdAsc')}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </FormField>

                            <FormField label={t('log.export.fields.chunkInterval')}>
                                <Input
                                    value={form.chunkInterval}
                                    onChange={(event) => setForm(prev => ({ ...prev, chunkInterval: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.chunkInterval')}
                                />
                            </FormField>
                        </div>
                    </section>

                    <section className="space-y-3">
                        <div>
                            <h3 className="text-sm font-semibold">{t('log.export.sections.filters')}</h3>
                            <p className="text-xs text-muted-foreground">
                                {t('log.export.sections.filtersDescription')}
                            </p>
                        </div>

                        <div className="grid gap-4 md:grid-cols-2">
                            <FormField label={t('log.export.fields.model')}>
                                <Input
                                    value={form.model}
                                    onChange={(event) => setForm(prev => ({ ...prev, model: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.model')}
                                />
                            </FormField>

                            {scope === 'group' ? (
                                <FormField label={t('log.export.fields.tokenName')}>
                                    <Input
                                        value={form.tokenName}
                                        onChange={(event) => setForm(prev => ({ ...prev, tokenName: event.target.value }))}
                                        disabled={exporting}
                                        className="h-9"
                                        placeholder={t('log.export.placeholders.tokenName')}
                                    />
                                </FormField>
                            ) : (
                                <FormField label={t('log.export.fields.channel')}>
                                    <Input
                                        type="number"
                                        min={1}
                                        value={form.channel}
                                        onChange={(event) => setForm(prev => ({ ...prev, channel: event.target.value }))}
                                        disabled={exporting}
                                        className="h-9"
                                        placeholder={t('log.export.placeholders.channel')}
                                    />
                                </FormField>
                            )}

                            <FormField label={t('log.export.fields.statusType')}>
                                <Select
                                    value={form.codeType}
                                    onValueChange={(codeType: ExportFormState['codeType']) => setForm(prev => ({ ...prev, codeType }))}
                                    disabled={exporting}
                                >
                                    <SelectTrigger className="h-9">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="all">{t('log.filters.statusAll')}</SelectItem>
                                        <SelectItem value="success">{t('log.filters.statusSuccess')}</SelectItem>
                                        <SelectItem value="error">{t('log.filters.statusError')}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </FormField>

                            <FormField label={t('log.export.fields.statusCode')}>
                                <Input
                                    type="number"
                                    min={0}
                                    value={form.code}
                                    onChange={(event) => setForm(prev => ({ ...prev, code: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.statusCode')}
                                />
                            </FormField>

                            <FormField label={t('log.export.fields.requestId')}>
                                <Input
                                    value={form.requestId}
                                    onChange={(event) => setForm(prev => ({ ...prev, requestId: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.requestId')}
                                />
                            </FormField>

                            <FormField label={t('log.export.fields.upstreamId')}>
                                <Input
                                    value={form.upstreamId}
                                    onChange={(event) => setForm(prev => ({ ...prev, upstreamId: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.upstreamId')}
                                />
                            </FormField>

                            <FormField label={t('log.export.fields.ip')}>
                                <Input
                                    value={form.ip}
                                    onChange={(event) => setForm(prev => ({ ...prev, ip: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.ip')}
                                />
                            </FormField>

                            <FormField label={t('log.export.fields.user')}>
                                <Input
                                    value={form.user}
                                    onChange={(event) => setForm(prev => ({ ...prev, user: event.target.value }))}
                                    disabled={exporting}
                                    className="h-9"
                                    placeholder={t('log.export.placeholders.user')}
                                />
                            </FormField>
                        </div>
                    </section>

                    <section className="space-y-3">
                        <div>
                            <h3 className="text-sm font-semibold">{t('log.export.sections.options')}</h3>
                            <p className="text-xs text-muted-foreground">
                                {t('log.export.sections.optionsDescription')}
                            </p>
                        </div>

                        <div className="space-y-3">
                            <ToggleRow
                                label={t('log.export.fields.includeDetail')}
                                description={t('log.export.hints.includeDetail')}
                                checked={form.includeDetail}
                                onCheckedChange={(includeDetail) => setForm(prev => ({ ...prev, includeDetail }))}
                            />

                            {scope === 'group' && (
                                <>
                                    <ToggleRow
                                        label={t('log.export.fields.includeChannel')}
                                        description={t('log.export.hints.includeChannel')}
                                        checked={form.includeChannel}
                                        onCheckedChange={(includeChannel) => setForm(prev => ({ ...prev, includeChannel }))}
                                    />

                                    <ToggleRow
                                        label={t('log.export.fields.includeRetryAt')}
                                        description={t('log.export.hints.includeRetryAt')}
                                        checked={form.includeRetryAt}
                                        onCheckedChange={(includeRetryAt) => setForm(prev => ({ ...prev, includeRetryAt }))}
                                    />
                                </>
                            )}

                            <ToggleRow
                                label={t('log.export.fields.limitEntries')}
                                description={t('log.export.hints.limitEntries')}
                                checked={form.limitEntries}
                                onCheckedChange={(limitEntries) => setForm(prev => ({ ...prev, limitEntries }))}
                            />

                            {form.limitEntries && (
                                <FormField label={t('log.export.fields.maxEntries')}>
                                    <Input
                                        type="number"
                                        min={1}
                                        value={form.maxEntries}
                                        onChange={(event) => setForm(prev => ({ ...prev, maxEntries: event.target.value }))}
                                        disabled={exporting}
                                        className="h-9"
                                        placeholder={t('log.export.placeholders.maxEntries')}
                                    />
                                </FormField>
                            )}
                        </div>
                    </section>
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => setOpen(false)} disabled={exporting}>
                        {t('common.cancel')}
                    </Button>
                    <Button onClick={handleExport} disabled={exporting}>
                        {exporting ? (
                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                            <Download className="mr-2 h-4 w-4" />
                        )}
                        {exporting ? t('log.export.exporting') : t('log.export.confirm')}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
