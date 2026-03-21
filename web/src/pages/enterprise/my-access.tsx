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
import {
    enterpriseApi,
    type MyTokenInfo,
    type MyAccessResponse,
    type ModelGroupInfo,
} from "@/api/enterprise"

// Matches core/relay/mode/define.go iota values
const MODEL_TYPES: Record<number, string> = {
    1: "Chat Completions",
    2: "Completions",
    3: "Embeddings",
    4: "Moderations",
    5: "Image Generation",
    6: "Image Edits",
    7: "Audio Speech",
    8: "Audio Transcription",
    9: "Audio Translation",
    10: "Rerank",
    11: "Parse PDF",
    12: "Anthropic",
    15: "Responses",
    20: "Gemini",
}

function maskKey(key: string): string {
    if (key.length <= 8) return key
    return key.slice(0, 6) + "****" + key.slice(-4)
}

function copyToClipboard(text: string, successMsg: string) {
    navigator.clipboard.writeText(text)
    toast.success(successMsg)
}

function formatPrice(price: number, unit: number): string {
    if (price === 0) return "Free"
    const perMillion = (price / (unit || 1000)) * 1_000_000
    return `$${perMillion.toFixed(2)}`
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
function ModelGroupSection({ groups }: { groups: ModelGroupInfo[] }) {
    const { t } = useTranslation()
    const [search, setSearch] = useState("")
    const [typeFilter, setTypeFilter] = useState("all")
    const [openOwners, setOpenOwners] = useState<Set<string>>(() => new Set(groups.map(g => g.owner)))

    const allTypes = useMemo(() => {
        const types = new Set<number>()
        groups.forEach(g => g.models.forEach(m => types.add(m.type)))
        return Array.from(types).sort()
    }, [groups])

    const filteredGroups = useMemo(() => {
        return groups.map(g => ({
            ...g,
            models: g.models.filter(m => {
                const matchSearch = !search || m.model.toLowerCase().includes(search.toLowerCase())
                const matchType = typeFilter === "all" || m.type === Number(typeFilter)
                return matchSearch && matchType
            }),
        })).filter(g => g.models.length > 0)
    }, [groups, search, typeFilter])

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
                        <Select value={typeFilter} onValueChange={setTypeFilter}>
                            <SelectTrigger className="h-9 w-36">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">{t("enterprise.myAccess.allTypes")}</SelectItem>
                                {allTypes.map(type => (
                                    <SelectItem key={type} value={String(type)}>
                                        {MODEL_TYPES[type] || `Type ${type}`}
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
                                <span className="capitalize">{group.owner}</span>
                                <Badge variant="secondary" className="ml-1 text-xs">
                                    {t("enterprise.myAccess.modelCount", { count: group.models.length })}
                                </Badge>
                            </CollapsibleTrigger>
                            <CollapsibleContent>
                                <div className="ml-6 mt-1 border rounded-md overflow-hidden">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="border-b bg-muted/50">
                                                <th className="px-3 py-2 text-left font-medium">Model</th>
                                                <th className="px-3 py-2 text-left font-medium">Type</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.inputPrice")}</th>
                                                <th className="px-3 py-2 text-right font-medium">{t("enterprise.myAccess.outputPrice")}</th>
                                                <th className="px-3 py-2 text-right font-medium">RPM</th>
                                                <th className="px-3 py-2 text-right font-medium">TPM</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {group.models.map(m => (
                                                <tr key={m.model} className="border-b last:border-b-0 hover:bg-muted/30">
                                                    <td className="px-3 py-2 font-mono text-xs">{m.model}</td>
                                                    <td className="px-3 py-2">
                                                        <Badge variant="outline" className="text-xs">
                                                            {MODEL_TYPES[m.type] || `Type ${m.type}`}
                                                        </Badge>
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs">
                                                        {formatPrice(m.input_price, m.price_unit)}
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs">
                                                        {formatPrice(m.output_price, m.price_unit)}
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs">
                                                        {m.rpm || "-"}
                                                    </td>
                                                    <td className="px-3 py-2 text-right tabular-nums text-xs">
                                                        {m.tpm ? m.tpm.toLocaleString() : "-"}
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
            <ModelGroupSection groups={modelGroups} />

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
