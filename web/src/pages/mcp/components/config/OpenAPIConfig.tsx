import { useTranslation } from 'react-i18next'
import { MCPOpenAPIConfig } from '@/api/mcp'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

interface OpenAPIConfigProps {
  config: MCPOpenAPIConfig | undefined
  onChange: (config: MCPOpenAPIConfig) => void
}

const OpenAPIConfig = ({ config, onChange }: OpenAPIConfigProps) => {
  const { t } = useTranslation()
  const openApiConfig: MCPOpenAPIConfig = config ?? {
      openapi_spec: '',
      openapi_content: '',
      v2: false,
      server_addr: '',
      authorization: ''
    }

  const handleChange = (field: keyof MCPOpenAPIConfig, value: string | boolean) => {
    const newConfig = { ...openApiConfig, [field]: value }
    onChange(newConfig)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-2">
        <Switch
          id="v2"
          checked={openApiConfig.v2}
          onCheckedChange={(checked) => handleChange('v2', checked)}
        />
        <Label htmlFor="v2">OpenAPI v2 (Swagger)</Label>
      </div>

      <Tabs defaultValue={openApiConfig.openapi_content ? "content" : "url"}>
        <TabsList className="grid grid-cols-2">
          <TabsTrigger value="url">URL</TabsTrigger>
          <TabsTrigger value="content">JSON / YAML</TabsTrigger>
        </TabsList>

        <TabsContent value="url" className="space-y-4 pt-4">
          <div className="space-y-2">
            <Label htmlFor="openapi_spec">{t("mcp.config.typeConfig.openapi.spec")}</Label>
            <Input
              id="openapi_spec"
              value={openApiConfig.openapi_spec}
              onChange={(e) => handleChange('openapi_spec', e.target.value)}
              placeholder="https://example.com/openapi.json"
            />
          </div>
        </TabsContent>

        <TabsContent value="content" className="space-y-4 pt-4">
          <div className="space-y-2">
            <Label htmlFor="openapi_content">{t("mcp.config.typeConfig.openapi.spec")}</Label>
            <Textarea
              id="openapi_content"
              value={openApiConfig.openapi_content || ''}
              onChange={(e) => handleChange('openapi_content', e.target.value)}
              placeholder="Paste your OpenAPI/Swagger JSON or YAML here"
              className="min-h-48 font-mono text-xs"
            />
          </div>
        </TabsContent>
      </Tabs>

      <div className="space-y-2">
        <Label htmlFor="server_addr">{t("mcp.config.typeConfig.openapi.server")}</Label>
        <Input
          id="server_addr"
          value={openApiConfig.server_addr || ''}
          onChange={(e) => handleChange('server_addr', e.target.value)}
          placeholder="https://api.example.com"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="authorization">{t("mcp.config.typeConfig.openapi.authorization")}</Label>
        <Input
          id="authorization"
          value={openApiConfig.authorization || ''}
          onChange={(e) => handleChange('authorization', e.target.value)}
          placeholder="Bearer token123"
          type="password"
        />
      </div>
    </div>
  )
}

export default OpenAPIConfig
