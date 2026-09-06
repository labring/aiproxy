import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2 } from 'lucide-react'
import { PublicMCPProxyConfig, PublicMCPProxyReusingParam } from '@/api/mcp'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'

interface ProxyConfigProps {
  config: PublicMCPProxyConfig | undefined
  onChange: (config: PublicMCPProxyConfig) => void
}

const ProxyConfig = ({ config, onChange }: ProxyConfigProps) => {
  const { t } = useTranslation()
  const proxyConfig: PublicMCPProxyConfig = config ?? {
      url: '',
      headers: {},
      querys: {},
      reusing: {}
    }

  // 添加键值对的临时状态
  const [newHeaderKey, setNewHeaderKey] = useState('')
  const [newHeaderValue, setNewHeaderValue] = useState('')
  const [newQueryKey, setNewQueryKey] = useState('')
  const [newQueryValue, setNewQueryValue] = useState('')
  const [newReusingKey, setNewReusingKey] = useState('')
  const [newReusingParam, setNewReusingParam] = useState<PublicMCPProxyReusingParam>({
    name: '',
    description: '',
    required: false,
    type: 'header'
  })

  const handleURLChange = (url: string) => {
    const newConfig = { ...proxyConfig, url }
    onChange(newConfig)
  }

  const addHeader = () => {
    if (!newHeaderKey.trim()) return

    const newHeaders = {
      ...proxyConfig.headers,
      [newHeaderKey]: newHeaderValue
    }

    const newConfig = {
      ...proxyConfig,
      headers: newHeaders
    }

    onChange(newConfig)
    setNewHeaderKey('')
    setNewHeaderValue('')
  }

  const removeHeader = (key: string) => {
    const newHeaders = { ...proxyConfig.headers }
    delete newHeaders[key]

    const newConfig = {
      ...proxyConfig,
      headers: newHeaders
    }

    onChange(newConfig)
  }

  const addQuery = () => {
    if (!newQueryKey.trim()) return

    const newQuerys = {
      ...proxyConfig.querys,
      [newQueryKey]: newQueryValue
    }

    const newConfig = {
      ...proxyConfig,
      querys: newQuerys
    }

    onChange(newConfig)
    setNewQueryKey('')
    setNewQueryValue('')
  }

  const removeQuery = (key: string) => {
    const newQuerys = { ...proxyConfig.querys }
    delete newQuerys[key]

    const newConfig = {
      ...proxyConfig,
      querys: newQuerys
    }

    onChange(newConfig)
  }

  const addReusingParam = () => {
    if (!newReusingKey.trim() || !newReusingParam.name.trim()) return

    const newReusingParams = {
      ...proxyConfig.reusing,
      [newReusingKey]: { ...newReusingParam }
    }

    const newConfig = {
      ...proxyConfig,
      reusing: newReusingParams
    }

    onChange(newConfig)
    setNewReusingKey('')
    setNewReusingParam({
      name: '',
      description: '',
      required: false,
      type: 'header'
    })
  }

  const removeReusingParam = (key: string) => {
    const newReusingParams = { ...proxyConfig.reusing }
    delete newReusingParams[key]

    const newConfig = {
      ...proxyConfig,
      reusing: newReusingParams
    }

    onChange(newConfig)
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Label htmlFor="url">URL <span className="text-destructive">*</span></Label>
        <Input
          id="url"
          value={proxyConfig.url}
          onChange={(e) => handleURLChange(e.target.value)}
          placeholder="https://example.com/api"
        />

      </div>

      <Tabs defaultValue="headers">
        <TabsList className="grid grid-cols-3">
          <TabsTrigger value="headers">{t("mcp.config.typeConfig.proxy.headers")}</TabsTrigger>
          <TabsTrigger value="query">{t("mcp.config.typeConfig.proxy.queryParams")}</TabsTrigger>
          <TabsTrigger value="reusing">{t("mcp.config.typeConfig.proxy.reuseParams")}</TabsTrigger>
        </TabsList>

        <TabsContent value="headers" className="space-y-4 pt-4">
          <div className="space-y-2">
            <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
              <Input
                className="col-span-2 sm:col-span-1"
                aria-label={t("mcp.config.typeConfig.proxy.headerName")}
                placeholder={t("mcp.config.typeConfig.proxy.headerName")}
                value={newHeaderKey}
                onChange={(e) => setNewHeaderKey(e.target.value)}
              />
              <Input
                aria-label={t("mcp.config.typeConfig.proxy.headerValue")}
                placeholder={t("mcp.config.typeConfig.proxy.headerValue")}
                value={newHeaderValue}
                onChange={(e) => setNewHeaderValue(e.target.value)}
              />
              <Button type="button" variant="outline" size="icon" aria-label={t("mcp.config.typeConfig.proxy.addHeader")} title={t("mcp.config.typeConfig.proxy.addHeader")} onClick={addHeader}><Plus className="size-4" /></Button>
            </div>
          </div>

          {Object.keys(proxyConfig.headers).length === 0 ? (
            <div className="text-center text-muted-foreground py-4">
              {t("common.noResult")}
            </div>
          ) : (
            <div className="space-y-2">
              {Object.entries(proxyConfig.headers).map(([key, value]) => (
                <div key={key} className="flex items-center gap-2 p-2 bg-muted rounded-md">
                  <div className="min-w-0 flex-1 break-all">
                    <div className="font-medium">{key}</div>
                    <div className="text-sm text-muted-foreground">{value}</div>
                  </div>
                  <Button variant="ghost" size="icon" aria-label={t("ui.removeItem", { name: key })} onClick={() => removeHeader(key)}><Trash2 className="size-4" /></Button>
                </div>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="query" className="space-y-4 pt-4">
          <div className="space-y-2">
            <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
              <Input
                className="col-span-2 sm:col-span-1"
                aria-label={t("mcp.config.typeConfig.proxy.paramName")}
                placeholder={t("mcp.config.typeConfig.proxy.paramName")}
                value={newQueryKey}
                onChange={(e) => setNewQueryKey(e.target.value)}
              />
              <Input
                aria-label={t("mcp.config.typeConfig.proxy.paramValue")}
                placeholder={t("mcp.config.typeConfig.proxy.paramValue")}
                value={newQueryValue}
                onChange={(e) => setNewQueryValue(e.target.value)}
              />
              <Button type="button" variant="outline" size="icon" aria-label={t("mcp.config.typeConfig.proxy.addQueryParam")} title={t("mcp.config.typeConfig.proxy.addQueryParam")} onClick={addQuery}><Plus className="size-4" /></Button>
            </div>
          </div>

          {Object.keys(proxyConfig.querys).length === 0 ? (
            <div className="text-center text-muted-foreground py-4">
              {t("common.noResult")}
            </div>
          ) : (
            <div className="space-y-2">
              {Object.entries(proxyConfig.querys).map(([key, value]) => (
                <div key={key} className="flex items-center gap-2 p-2 bg-muted rounded-md">
                  <div className="min-w-0 flex-1 break-all">
                    <div className="font-medium">{key}</div>
                    <div className="text-sm text-muted-foreground">{value}</div>
                  </div>
                  <Button variant="ghost" size="icon" aria-label={t("ui.removeItem", { name: key })} onClick={() => removeQuery(key)}><Trash2 className="size-4" /></Button>
                </div>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="reusing" className="space-y-4 pt-4">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="reusingKey">{t("mcp.config.id")}</Label>
              <Input
                id="reusingKey"
                placeholder="e.g., api_key"
                value={newReusingKey}
                onChange={(e) => setNewReusingKey(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="reusingName">{t("mcp.config.name")}</Label>
              <Input
                id="reusingName"
                placeholder="e.g., API Key"
                value={newReusingParam.name}
                onChange={(e) => setNewReusingParam({...newReusingParam, name: e.target.value})}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="reusingDescription">{t("mcp.description")}</Label>
              <Textarea
                id="reusingDescription"
                placeholder="Describe what this parameter is for"
                value={newReusingParam.description}
                onChange={(e) => setNewReusingParam({...newReusingParam, description: e.target.value})}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="reusingType">{t("mcp.config.type")}</Label>
              <Select
                value={newReusingParam.type}
                onValueChange={(value: 'header' | 'query') => setNewReusingParam({...newReusingParam, type: value})}
              >
                <SelectTrigger id="reusingType">
                  <SelectValue placeholder="Select parameter type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="header">Header</SelectItem>
                  <SelectItem value="query">Query Parameter</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center space-x-2">
              <Switch
                id="required"
                checked={newReusingParam.required}
                onCheckedChange={(checked) => setNewReusingParam({...newReusingParam, required: checked})}
              />
              <Label htmlFor="required">{t("ui.required")}</Label>
            </div>

            <Button type="button" onClick={addReusingParam}>
              {t("mcp.config.typeConfig.proxy.addQueryParam")}
            </Button>
          </div>

          {Object.keys(proxyConfig.reusing).length === 0 ? (
            <div className="text-center text-muted-foreground py-4">
              {t("common.noResult")}
            </div>
          ) : (
            <div className="space-y-2">
              {Object.entries(proxyConfig.reusing).map(([key, param]) => (
                <Card key={key}>
                  <CardHeader>
                    <CardTitle className="text-base flex justify-between">
                      <span>{key}</span>
                      <Button variant="ghost" size="icon" aria-label={t("ui.removeItem", { name: key })} onClick={() => removeReusingParam(key)}><Trash2 className="size-4" /></Button>
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-1">
                      <div className="text-sm">
                        <span className="font-medium">Name:</span> {param.name}
                      </div>
                      <div className="text-sm">
                        <span className="font-medium">Type:</span> {param.type}
                      </div>
                      <div className="text-sm">
                        <span className="font-medium">Required:</span> {param.required ? 'Yes' : 'No'}
                      </div>
                      {param.description && (
                        <div className="text-sm">
                          <span className="font-medium">Description:</span> {param.description}
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}

export default ProxyConfig
