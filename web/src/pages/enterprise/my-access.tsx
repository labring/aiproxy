import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Copy, Eye, EyeOff, Plus, Ban, ChevronDown, ChevronRight, Search,
} from "lucide-react"
import { toast } from "sonner"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import type { DateRange } from "react-day-picker"
import {
    enterpriseApi,
    type MyTokenInfo,
    type MyAccessResponse,
    type ModelGroupInfo,
    type MyStatsResponse,
} from "@/api/enterprise"
import { getTimeRange, formatAmount, formatNumber, type TimeRange } from "@/lib/enterprise"

// Semantic color groups for endpoint badges
const EP_COLORS = {
    chat: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
    anthropic: "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
    responses: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
    embeddings: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
    image: "bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200",
    misc: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200",
    video: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
} as const

// Endpoint path → short display label and color class
const ENDPOINT_LABELS: Record<string, { label: string; color: string }> = {
    "POST /v1/chat/completions": { label: "Chat", color: EP_COLORS.chat },
    "POST /v1/completions": { label: "Completions", color: EP_COLORS.chat },
    "POST /v1/messages": { label: "Anthropic", color: EP_COLORS.anthropic },
    "POST /v1/responses": { label: "Responses", color: EP_COLORS.responses },
    "POST /v1/embeddings": { label: "Embeddings", color: EP_COLORS.embeddings },
    "POST /v1/moderations": { label: "Moderations", color: EP_COLORS.misc },
    "POST /v1/images/generations": { label: "Image Gen", color: EP_COLORS.image },
    "POST /v1/images/edits": { label: "Image Edit", color: EP_COLORS.image },
    "POST /v1/audio/speech": { label: "TTS", color: EP_COLORS.misc },
    "POST /v1/audio/transcriptions": { label: "STT", color: EP_COLORS.misc },
    "POST /v1/audio/translations": { label: "Translate", color: EP_COLORS.misc },
    "POST /v1/rerank": { label: "Rerank", color: EP_COLORS.misc },
    "POST /v1/parse/pdf": { label: "Parse PDF", color: EP_COLORS.misc },
    "POST /v1/video/generations/jobs": { label: "Video Gen", color: EP_COLORS.video },
    "GET /v1/video/generations/jobs/{id}": { label: "Video Status", color: EP_COLORS.video },
}

// Translate a server-side type_name (e.g. "chat") via i18n keys "enterprise.myAccess.typeName_chat".
function typeNameLabel(t: (k: never) => string, name: string): string {
    const key = `enterprise.myAccess.typeName_${name}` as never
    const translated = t(key)
    // Fallback to raw name if no translation found (key returned as-is)
    return translated === key ? name : translated
}

// Display names for model owners that need special casing (all-caps abbreviations, etc.).
// Owners not listed here fall back to CSS `capitalize` (first-letter uppercase).
const OWNER_DISPLAY_NAMES: Record<string, string> = {
    ppio: "PPIO",
    baai: "BAAI",
    xai: "xAI",
    chatglm: "ChatGLM",
    funaudiollm: "FunAudioLLM",
}

function ownerDisplayName(owner: string): string {
    return OWNER_DISPLAY_NAMES[owner.toLowerCase()] ?? owner
}

// Build the full endpoint URL from base URL and endpoint descriptor.
// e.g. baseUrl="https://api.example.com/v1", ep="POST /v1/chat/completions"
//   → "https://api.example.com/v1/chat/completions"
function getEndpointUrl(baseUrl: string, ep: string): string {
    // Extract path from "METHOD /v1/..." → "/v1/..."
    const path = ep.replace(/^\S+\s+/, "")
    // Strip the /v1 prefix since baseUrl already ends with /v1
    const suffix = path.replace(/^\/v1\/?/, "/")
    // Ensure no double slashes when joining
    const base = baseUrl.replace(/\/+$/, "")
    return base + suffix
}

function maskKey(key: string): string {
    if (key.length <= 8) return key
    return key.slice(0, 6) + "****" + key.slice(-4)
}

function copyToClipboard(text: string, successMsg: string) {
    navigator.clipboard.writeText(text)
    toast.success(successMsg)
}

function formatPrice(price: number, unit: number, freeLabel: string): string {
    if (price === 0) return freeLabel
    const perMillion = (price / (unit || 1000)) * 1_000_000
    return `¥${perMillion.toFixed(2)}`
}

// --- Personal Stats Section ---
function PersonalStatsSection() {
    const { t } = useTranslation()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()

    const { start, end } = useMemo(() => {
        if (timeRange === "custom" && customDateRange?.from) {
            const s = Math.floor(customDateRange.from.getTime() / 1000)
            const e = customDateRange.to
                ? Math.floor(customDateRange.to.getTime() / 1000) + 86399
                : s + 86399
            return { start: s, end: e }
        }
        return getTimeRange(timeRange)
    }, [timeRange, customDateRange])

    const { data, isLoading } = useQuery<MyStatsResponse>({
        queryKey: ["my-stats", start, end],
        queryFn: () => enterpriseApi.getMyStats(start, end),
    })

    const tierColor = (tier: number) => {
        if (tier === 1) return "bg-green-500"
        if (tier === 2) return "bg-yellow-500"
        return "bg-red-500"
    }

    const tierBadgeVariant = (tier: number): "default" | "secondary" | "destructive" => {
        if (tier === 1) return "default"
        if (tier === 2) return "secondary"
        return "destructive"
    }

    if (isLoading) {
        return (
            <div className="space-y-4">
                <div className="h-6 w-48 bg-muted animate-pulse rounded" />
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    {[1, 2, 3, 4].map(i => (
                        <div key={i} className="h-24 bg-muted animate-pulse rounded" />
                    ))}
                </div>
                <div className="h-32 bg-muted animate-pulse rounded" />
            </div>
        )
    }

    const usage = data?.usage
    const quota = data?.quota

    return (
        <div className="space-y-4">
            {/* Header + time range selector */}
            <div className="flex items-center justify-between flex-wrap gap-2">
                <h2 className="text-lg font-semibold">{t("enterprise.myAccess.personalStats" as never)}</h2>
                <div className="flex items-center gap-2">
                    <Select value={timeRange} onValueChange={v => setTimeRange(v as TimeRange)}>
                        <SelectTrigger className="h-9 w-36">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="7d">{t("enterprise.myAccess.last7Days" as never)}</SelectItem>
                            <SelectItem value="30d">{t("enterprise.myAccess.last30Days" as never)}</SelectItem>
                            <SelectItem value="month">{t("enterprise.myAccess.thisMonth" as never)}</SelectItem>
                            <SelectItem value="last_week">{t("enterprise.myAccess.lastWeek" as never)}</SelectItem>
                            <SelectItem value="last_month">{t("enterprise.myAccess.lastMonth" as never)}</SelectItem>
                            <SelectItem value="custom">{t("enterprise.myAccess.customRange" as never)}</SelectItem>
                        </SelectContent>
                    </Select>
                    {timeRange === "custom" && (
                        <DateRangePicker value={customDateRange} onChange={setCustomDateRange} />
                    )}
                </div>
            </div>

            {/* Usage stat cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <Card>
                    <CardContent className="pt-4 pb-4">
                        <p className="text-xs text-muted-foreground">{t("enterprise.myAccess.totalConsumption" as never)}</p>
                        <p className="text-2xl font-bold tabular-nums mt-1">{formatAmount(usage?.total_amount ?? 0)}</p>
                    </CardContent>
                </Card>
                <Card>
                    <CardContent className="pt-4 pb-4">
                        <p className="text-xs text-muted-foreground">{t("enterprise.myAccess.totalTokens" as never)}</p>
                        <p className="text-2xl font-bold tabular-nums mt-1">{formatNumber(usage?.total_tokens ?? 0)}</p>
                    </CardContent>
                </Card>
                <Card>
                    <CardContent className="pt-4 pb-4">
                        <p className="text-xs text-muted-foreground">{t("enterprise.myAccess.totalRequests" as never)}</p>
                        <p className="text-2xl font-bold tabular-nums mt-1">{formatNumber(usage?.total_requests ?? 0)}</p>
                    </CardContent>
                </Card>
                <Card>
                    <CardContent className="pt-4 pb-4">
                        <p className="text-xs text-muted-foreground">{t("enterprise.myAccess.uniqueModels" as never)}</p>
                        <p className="text-2xl font-bold tabular-nums mt-1">{usage?.unique_models ?? 0}</p>
                    </CardContent>
                </Card>
            </div>

            {/* Top models + quota status */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {/* Top models */}
                <Card>
                    <CardHeader className="pb-2">
                        <CardTitle className="text-sm font-semibold">{t("enterprise.myAccess.topModels" as never)}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        {!usage?.top_models?.length ? (
                            <p className="text-sm text-muted-foreground text-center py-4">—</p>
                        ) : (
                            <table className="w-full text-sm">
                                <tbody>
                                    {usage.top_models.map(m => (
                                        <tr key={m.model} className="border-b last:border-0">
                                            <td className="py-2 font-mono text-xs truncate max-w-[160px]">{m.model}</td>
                                            <td className="py-2 text-right tabular-nums text-xs">{formatAmount(m.used_amount)}</td>
                                            <td className="py-2 text-right tabular-nums text-xs text-muted-foreground">{formatNumber(m.request_count)} req</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        )}
                    </CardContent>
                </Card>

                {/* Quota status */}
                <Card>
                    <CardHeader className="pb-2">
                        <CardTitle className="text-sm font-semibold">{t("enterprise.myAccess.quotaStatus" as never)}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        {!quota ? (
                            <p className="text-sm text-muted-foreground text-center py-4">
                                {t("enterprise.myAccess.noPolicy" as never)}
                            </p>
                        ) : (
                            <div className="space-y-3">
                                {/* Progress bar */}
                                <div className="space-y-1">
                                    <div className="flex justify-between text-xs text-muted-foreground">
                                        <span>{formatAmount(quota.period_used)} / {formatAmount(quota.period_quota)}</span>
                                        <span>{((quota.period_used / quota.period_quota) * 100).toFixed(1)}%</span>
                                    </div>
                                    <div className="h-2 bg-muted rounded-full overflow-hidden">
                                        <div
                                            className={`h-full rounded-full transition-all ${tierColor(quota.current_tier)}`}
                                            style={{ width: `${Math.min((quota.period_used / quota.period_quota) * 100, 100)}%` }}
                                        />
                                    </div>
                                </div>

                                {/* Badges */}
                                <div className="flex items-center gap-2 flex-wrap">
                                    <Badge variant="outline" className="text-xs">{quota.policy_name}</Badge>
                                    <Badge variant={tierBadgeVariant(quota.current_tier)} className="text-xs">
                                        {t("enterprise.myAccess.currentTier" as never)} {quota.current_tier}
                                    </Badge>
                                    <Badge variant="secondary" className="text-xs capitalize">{quota.period_type}</Badge>
                                </div>

                                {/* Block warning */}
                                {quota.block_at_tier3 && quota.current_tier === 3 && (
                                    <p className="text-xs text-red-600 dark:text-red-400 font-medium">
                                        ⚠ {t("enterprise.myAccess.blocked" as never)}
                                    </p>
                                )}
                            </div>
                        )}
                    </CardContent>
                </Card>
            </div>
        </div>
    )
}

// --- Token Row ---
function TokenRow({ token, onDisable }: {
    token: MyTokenInfo
    onDisable: (id: number) => void
}) {
    const { t } = useTranslation()
    const [visible, setVisible] = useState(false)
    const disabled = token.status === 2

    return (
        <tr className={disabled ? "opacity-50" : ""}>
            <td className="px-4 py-3 text-sm font-medium">{token.name || "-"}</td>
            <td className="px-4 py-3 text-sm font-mono">
                <span className="inline-flex items-center gap-1.5">
                    {visible ? token.key : maskKey(token.key)}
                    <button
                        onClick={() => setVisible(!visible)}
                        className="text-muted-foreground hover:text-foreground"
                        title={visible ? t("enterprise.myAccess.hideKey") : t("enterprise.myAccess.showKey")}
                    >
                        {visible ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                    </button>
                    <button
                        onClick={() => copyToClipboard(token.key, t("enterprise.myAccess.copied"))}
                        className="text-muted-foreground hover:text-foreground"
                        title={t("enterprise.myAccess.copyKey")}
                    >
                        <Copy className="w-3.5 h-3.5" />
                    </button>
                </span>
            </td>
            <td className="px-4 py-3 text-sm">
                <Badge variant={disabled ? "secondary" : "default"}>
                    {disabled ? t("enterprise.myAccess.disabled") : t("enterprise.myAccess.enabled")}
                </Badge>
            </td>
            <td className="px-4 py-3 text-sm text-muted-foreground">
                {new Date(token.created_at).toLocaleDateString()}
            </td>
            <td className="px-4 py-3 text-sm text-right tabular-nums">
                ¥{(token.used_amount || 0).toFixed(4)}
            </td>
            <td className="px-4 py-3 text-sm text-right tabular-nums">
                {token.request_count || 0}
            </td>
            <td className="px-4 py-3 text-sm">
                {!disabled && (
                    <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => onDisable(token.id)}
                    >
                        <Ban className="w-3.5 h-3.5 mr-1" />
                        {t("enterprise.myAccess.disableKey")}
                    </Button>
                )}
            </td>
        </tr>
    )
}

// --- Quick Start Snippets ---
function QuickStartSection({ baseUrl }: { baseUrl: string }) {
    const { t } = useTranslation()
    const [openItems, setOpenItems] = useState<Set<string>>(new Set())

    const toggle = (key: string) => {
        setOpenItems(prev => {
            const next = new Set(prev)
            if (next.has(key)) next.delete(key)
            else next.add(key)
            return next
        })
    }

    const snippets = [
        {
            key: "python",
            title: "OpenAI Python SDK",
            code: `from openai import OpenAI

client = OpenAI(
    api_key="your-api-key",  # ${t("enterprise.myAccess.copyKey")}
    base_url="${baseUrl}"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)`,
        },
        {
            key: "nodejs",
            title: "OpenAI Node.js SDK",
            code: `import OpenAI from 'openai';

const client = new OpenAI({
    apiKey: 'your-api-key',  // ${t("enterprise.myAccess.copyKey")}
    baseURL: '${baseUrl}',
});

const response = await client.chat.completions.create({
    model: 'gpt-4o',
    messages: [{ role: 'user', content: 'Hello!' }],
});
console.log(response.choices[0].message.content);`,
        },
        {
            key: "anthropic",
            title: "Anthropic Python SDK",
            code: `import anthropic

client = anthropic.Anthropic(
    api_key="your-api-key",  # ${t("enterprise.myAccess.copyKey")}
    base_url="${baseUrl.replace(/\/v1\/?$/, "")}"  # ${t("enterprise.myAccess.anthropicNote")}
)
# Endpoint: POST ${baseUrl.replace(/\/v1\/?$/, "/v1")}/messages

message = client.messages.create(
    model="claude-sonnet-4-20250514",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello!"}]
)
print(message.content[0].text)`,
        },
        {
            key: "cursor",
            title: "Cursor",
            code: `# Cursor Settings > Models > OpenAI API Key
# API Key: your-api-key
# Base URL: ${baseUrl}
#
# Then select any available model from the model list.`,
        },
        {
            key: "cherry",
            title: "Cherry Studio",
            code: `# Cherry Studio Settings > AI Provider > OpenAI Compatible
# API Key: your-api-key
# API Base URL: ${baseUrl}`,
        },
    ]

    return (
        <Card>
            <CardHeader>
                <CardTitle className="text-base">{t("enterprise.myAccess.quickStart")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
                {snippets.map(s => (
                    <Collapsible key={s.key} open={openItems.has(s.key)} onOpenChange={() => toggle(s.key)}>
                        <CollapsibleTrigger className="flex items-center gap-2 w-full px-3 py-2 rounded-md hover:bg-muted text-sm font-medium">
                            {openItems.has(s.key) ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                            {s.title}
                        </CollapsibleTrigger>
                        <CollapsibleContent>
                            <div className="relative mt-1 ml-6">
                                <pre className="bg-muted p-4 rounded-md text-xs overflow-x-auto whitespace-pre">
                                    {s.code}
                                </pre>
                                <Button
                                    variant="ghost"
                                    size="icon"
                                    className="absolute top-2 right-2 h-7 w-7"
                                    onClick={() => copyToClipboard(s.code, t("enterprise.myAccess.copied"))}
                                >
                                    <Copy className="w-3.5 h-3.5" />
                                </Button>
                            </div>
                        </CollapsibleContent>
                    </Collapsible>
                ))}
            </CardContent>
        </Card>
    )
}

// --- Model Group Accordion ---
function ModelGroupSection({ groups, baseUrl }: { groups: ModelGroupInfo[]; baseUrl: string }) {
    const { t } = useTranslation()
    const [search, setSearch] = useState("")
    const [endpointFilter, setEndpointFilter] = useState("all")
    const [openOwners, setOpenOwners] = useState<Set<string>>(() => new Set(groups.map(g => g.owner)))

    // Collect unique endpoints across all models, ordered by ENDPOINT_LABELS definition order
    const allEndpoints = useMemo(() => {
        const seen = new Set<string>()
        groups.forEach(g => g.models.forEach(m => m.supported_endpoints?.forEach(ep => seen.add(ep))))
        const orderMap = new Map(Object.keys(ENDPOINT_LABELS).map((ep, i) => [ep, i]))
        return Array.from(seen).sort((a, b) => {
            const ia = orderMap.get(a) ?? Infinity
            const ib = orderMap.get(b) ?? Infinity
            if (ia === ib) return a.localeCompare(b)
            return ia - ib
        })
    }, [groups])

    const filteredGroups = useMemo(() => {
        return groups.map(g => ({
            ...g,
            models: g.models.filter(m => {
                const matchSearch = !search || m.model.toLowerCase().includes(search.toLowerCase())
                const matchEndpoint = endpointFilter === "all" || m.supported_endpoints?.includes(endpointFilter)
                return matchSearch && matchEndpoint
            }),
        })).filter(g => g.models.length > 0)
    }, [groups, search, endpointFilter])

    const toggleOwner = (owner: string) => {
        setOpenOwners(prev => {
            const next = new Set(prev)
            if (next.has(owner)) next.delete(owner)
            else next.add(owner)
            return next
        })
    }

    return (
        <Card>
            <CardHeader>
                <div className="flex items-center justify-between">
                    <CardTitle className="text-base">{t("enterprise.myAccess.availableModels")}</CardTitle>
                    <div className="flex items-center gap-2">
                        <div className="relative">
                            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                            <Input
                                placeholder={t("enterprise.myAccess.searchModels")}
                                className="pl-8 h-9 w-56"
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                            />
                        </div>
                        <Select value={endpointFilter} onValueChange={setEndpointFilter}>
                            <SelectTrigger className="h-9 w-40">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">{t("enterprise.myAccess.allTypes")}</SelectItem>
                                {allEndpoints.map(ep => (
                                    <SelectItem key={ep} value={ep}>
                                        {ENDPOINT_LABELS[ep]?.label ?? ep}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                </div>
            </CardHeader>
            <CardContent className="space-y-2">
                {filteredGroups.length === 0 ? (
                    <p className="text-sm text-muted-foreground text-center py-8">{t("enterprise.myAccess.noModels")}</p>
                ) : (
                    filteredGroups.map(group => (
                        <Collapsible
                            key={group.owner}
                            open={openOwners.has(group.owner)}
                            onOpenChange={() => toggleOwner(group.owner)}
                        >
                            <CollapsibleTrigger className="flex items-center gap-2 w-full px-3 py-2 rounded-md hover:bg-muted text-sm font-medium">
                                {openOwners.has(group.owner) ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                                <span>{ownerDisplayName(group.owner)}</span>
                                <Badge variant="secondary" className="ml-1 text-xs">
                                    {t("enterprise.myAccess.modelCount", { count: group.models.length })}
                                </Badge>
                            </CollapsibleTrigger>
                            <CollapsibleContent>
                                <div className="ml-6 mt-1 border rounded-md overflow-x-auto">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="border-b bg-muted/50">
                                                <th className="px-3 py-2 text-left font-medium">Model</th>
                                                <th className="px-3 py-2 text-left font-medium">{t("enterprise.myAccess.endpoints")}</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.context" as never)}</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.maxOutput" as never)}</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.inputPrice")}</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.outputPrice")}</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.limits" as never)}</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {group.models.map(m => (
                                                <tr key={m.model} className="border-b last:border-b-0 hover:bg-muted/30">
                                                    <td className="px-3 py-2">
                                                        <button
                                                            className="font-mono text-xs hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer transition-colors"
                                                            onClick={() => copyToClipboard(m.model, t("enterprise.myAccess.copied"))}
                                                            title={t("enterprise.myAccess.copyModelId" as never)}
                                                        >
                                                            {m.model}
                                                        </button>
                                                    </td>
                                                    <td className="px-3 py-2">
                                                        <div className="flex flex-wrap gap-1">
                                                            {m.supported_endpoints?.length > 0 ? (
                                                                m.supported_endpoints.map(ep => {
                                                                    const info = ENDPOINT_LABELS[ep]
                                                                    const fullUrl = getEndpointUrl(baseUrl, ep)
                                                                    return (
                                                                        <button
                                                                            key={ep}
                                                                            className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium cursor-pointer hover:opacity-80 ${info?.color || EP_COLORS.misc}`}
                                                                            title={fullUrl}
                                                                            onClick={() => copyToClipboard(fullUrl, `${info?.label || ep} ${t("enterprise.myAccess.endpointCopied")}`)}
                                                                        >
                                                                            {info?.label || ep}
                                                                        </button>
                                                                    )
                                                                })
                                                            ) : (
                                                                <Badge variant="outline" className="text-xs">
                                                                    {typeNameLabel(t, m.type_name)}
                                                                </Badge>
                                                            )}
                                                        </div>
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs text-muted-foreground">
                                                        {m.max_context ? formatNumber(m.max_context) : "-"}
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs text-muted-foreground">
                                                        {m.max_output ? formatNumber(m.max_output) : "-"}
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs">
                                                        {formatPrice(m.input_price, m.price_unit, t("enterprise.myAccess.free" as never))}
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs">
                                                        {formatPrice(m.output_price, m.price_unit, t("enterprise.myAccess.free" as never))}
                                                    </td>
                                                    <td className="px-3 py-2 text-right text-xs">
                                                        <div className="tabular-nums">{m.rpm || "-"} <span className="text-muted-foreground">RPM</span></div>
                                                        <div className="tabular-nums text-muted-foreground">{m.tpm ? m.tpm.toLocaleString() : "-"} TPM</div>
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </CollapsibleContent>
                        </Collapsible>
                    ))
                )}
            </CardContent>
        </Card>
    )
}

// --- Main Page ---
export default function MyAccessPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [createDialogOpen, setCreateDialogOpen] = useState(false)
    const [newKeyName, setNewKeyName] = useState("")
    const [newlyCreatedKey, setNewlyCreatedKey] = useState<MyTokenInfo | null>(null)
    const [disableConfirmId, setDisableConfirmId] = useState<number | null>(null)

    const { data, isLoading } = useQuery<MyAccessResponse>({
        queryKey: ["my-access"],
        queryFn: () => enterpriseApi.getMyAccess(),
    })

    const createMutation = useMutation({
        mutationFn: (name: string) => enterpriseApi.createMyToken(name),
        onSuccess: (token) => {
            setNewlyCreatedKey(token)
            setCreateDialogOpen(false)
            setNewKeyName("")
            queryClient.invalidateQueries({ queryKey: ["my-access"] })
            toast.success(t("enterprise.myAccess.createSuccess"))
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    const disableMutation = useMutation({
        mutationFn: (id: number) => enterpriseApi.disableMyToken(id),
        onSuccess: () => {
            setDisableConfirmId(null)
            queryClient.invalidateQueries({ queryKey: ["my-access"] })
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    if (isLoading) {
        return (
            <div className="p-6 space-y-4">
                <div className="h-8 w-48 bg-muted animate-pulse rounded" />
                <div className="h-32 bg-muted animate-pulse rounded" />
                <div className="h-64 bg-muted animate-pulse rounded" />
            </div>
        )
    }

    const baseUrl = data?.base_url || ""
    const tokens = data?.tokens || []
    const modelGroups = data?.model_groups || []

    return (
        <div className="p-6 space-y-6 max-w-6xl">
            <h1 className="text-2xl font-bold">{t("enterprise.myAccess.title")}</h1>

            {/* Personal Usage Overview */}
            <PersonalStatsSection />

            {/* Base URL */}
            <Card>
                <CardContent className="pt-6">
                    <div className="flex items-center gap-3">
                        <Label className="text-sm font-medium whitespace-nowrap">
                            {t("enterprise.myAccess.baseUrl")}
                        </Label>
                        <code className="flex-1 px-3 py-2 bg-muted rounded text-sm font-mono">{baseUrl}</code>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => copyToClipboard(baseUrl, t("enterprise.myAccess.copied"))}
                        >
                            <Copy className="w-3.5 h-3.5 mr-1" />
                            {t("enterprise.myAccess.copyKey")}
                        </Button>
                    </div>
                </CardContent>
            </Card>

            {/* API Keys */}
            <Card>
                <CardHeader>
                    <div className="flex items-center justify-between">
                        <CardTitle className="text-base">{t("enterprise.myAccess.apiKeys")}</CardTitle>
                        <Button size="sm" onClick={() => setCreateDialogOpen(true)}>
                            <Plus className="w-4 h-4 mr-1" />
                            {t("enterprise.myAccess.createKey")}
                        </Button>
                    </div>
                </CardHeader>
                <CardContent>
                    {tokens.length === 0 ? (
                        <p className="text-sm text-muted-foreground text-center py-8">
                            {t("enterprise.myAccess.noKeys")}
                        </p>
                    ) : (
                        <div className="border rounded-md overflow-x-auto">
                            <table className="w-full">
                                <thead>
                                    <tr className="border-b bg-muted/50">
                                        <th className="px-4 py-3 text-left text-sm font-medium">{t("enterprise.myAccess.tokenName")}</th>
                                        <th className="px-4 py-3 text-left text-sm font-medium">Key</th>
                                        <th className="px-4 py-3 text-left text-sm font-medium">Status</th>
                                        <th className="px-4 py-3 text-left text-sm font-medium">Created</th>
                                        <th className="px-4 py-3 text-right text-sm font-medium">{t("enterprise.myAccess.usedAmount")}</th>
                                        <th className="px-4 py-3 text-right text-sm font-medium">{t("enterprise.myAccess.requestCount")}</th>
                                        <th className="px-4 py-3 text-left text-sm font-medium">{t("enterprise.myAccess.actions")}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {tokens.map(token => (
                                        <TokenRow
                                            key={token.id}
                                            token={token}
                                            onDisable={setDisableConfirmId}
                                        />
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                </CardContent>
            </Card>

            {/* Quick Start */}
            <QuickStartSection baseUrl={baseUrl} />

            {/* Available Models */}
            <ModelGroupSection groups={modelGroups} baseUrl={baseUrl} />

            {/* Create Key Dialog */}
            <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.myAccess.createKey")}</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-4 py-2">
                        <div className="space-y-2">
                            <Label>{t("enterprise.myAccess.keyName")}</Label>
                            <Input
                                placeholder={t("enterprise.myAccess.keyNamePlaceholder")}
                                value={newKeyName}
                                onChange={e => setNewKeyName(e.target.value)}
                                maxLength={32}
                                onKeyDown={e => {
                                    if (e.key === "Enter" && newKeyName.trim()) {
                                        createMutation.mutate(newKeyName.trim())
                                    }
                                }}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
                            {t("common.cancel" as never)}
                        </Button>
                        <Button
                            onClick={() => createMutation.mutate(newKeyName.trim())}
                            disabled={!newKeyName.trim() || createMutation.isPending}
                        >
                            {t("enterprise.myAccess.createKey")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Newly Created Key Dialog */}
            <Dialog open={!!newlyCreatedKey} onOpenChange={() => setNewlyCreatedKey(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.myAccess.newKeyTitle")}</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-4 py-2">
                        <p className="text-sm text-amber-600 dark:text-amber-400 font-medium">
                            {t("enterprise.myAccess.createKeyHint")}
                        </p>
                        <div className="flex items-center gap-2">
                            <code className="flex-1 px-3 py-2 bg-muted rounded text-sm font-mono break-all">
                                {newlyCreatedKey?.key}
                            </code>
                            <Button
                                variant="outline"
                                size="icon"
                                onClick={() =>
                                    newlyCreatedKey && copyToClipboard(newlyCreatedKey.key, t("enterprise.myAccess.copied"))
                                }
                            >
                                <Copy className="w-4 h-4" />
                            </Button>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button onClick={() => setNewlyCreatedKey(null)}>OK</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Disable Confirm Dialog */}
            <Dialog open={disableConfirmId !== null} onOpenChange={() => setDisableConfirmId(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.myAccess.disableKey")}</DialogTitle>
                    </DialogHeader>
                    <p className="text-sm text-muted-foreground">
                        {t("enterprise.myAccess.disableKeyConfirm")}
                    </p>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDisableConfirmId(null)}>
                            {t("common.cancel" as never)}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={() => disableConfirmId !== null && disableMutation.mutate(disableConfirmId)}
                            disabled={disableMutation.isPending}
                        >
                            {t("enterprise.myAccess.disableKey")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
