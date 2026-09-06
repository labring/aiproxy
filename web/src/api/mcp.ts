import { get, post, put, del } from './index'

type ParamType = 'header' | 'query'

export interface PublicMCPProxyReusingParam extends ReusingParam {
  type: ParamType
}

export interface ReusingParam {
  name: string
  description: string
  required: boolean
}

export interface PublicMCPProxyConfig {
  url: string
  querys: Record<string, string>
  headers: Record<string, string>
  reusing: Record<string, PublicMCPProxyReusingParam>
}

export interface MCPOpenAPIConfig {
  openapi_spec: string
  openapi_content?: string
  v2: boolean
  server_addr?: string
  authorization?: string
}

export interface MCPEmbeddingConfig {
  init: Record<string, string>
  reusing: Record<string, ReusingParam>
}

export interface PublicMCP {
  id: string
  name: string
  status: number
  type: string
  created_at: number
  update_at: number
  readme: string
  tags: string[]
  logo_url: string
  proxy_config?: PublicMCPProxyConfig
  openapi_config?: MCPOpenAPIConfig
  embed_config?: MCPEmbeddingConfig
  endpoints: {
    host: string
    sse: string
    streamable_http: string
  }
}

export interface MCPListResponse {
  mcps: PublicMCP[]
  total: number
}

export interface EmbedMCPConfigTemplate {
  name: string
  required: boolean
  example: string
  description: string
}

export interface EmbedMCP {
  id: string
  enabled: boolean
  name: string
  name_cn?: string
  readme: string
  readme_cn?: string
  tags: string[]
  config_templates: Record<string, EmbedMCPConfigTemplate>
  embed_config?: MCPEmbeddingConfig | null
}

export interface SaveEmbedMCPRequest {
  id: string
  enabled: boolean
  init_config: Record<string, string>
}

export interface PublicMCPReusingParam {
  mcp_id: string
  group_id: string
  params: Record<string, string>
}

// API functions
export const getMCPs = (params: {
  page: number
  per_page: number
  type?: string
  keyword?: string
  status?: number
}) => {
  return get<MCPListResponse>('/mcp/publics/', { params })
}

export const getAllMCPs = (params?: { status?: number }) => {
  return get<PublicMCP[]>('/mcp/publics/all', { params })
}

export const getMCPById = (id: string) => {
  return get<PublicMCP>(`/mcp/public/${id}`)
}

export const createMCP = (data: PublicMCP) => {
  return post<PublicMCP>('/mcp/public/', data)
}

export const updateMCP = (id: string, data: PublicMCP) => {
  return put<PublicMCP>(`/mcp/public/${id}`, data)
}

export const updateMCPStatus = (id: string, status: number) => {
  return post(`/mcp/public/${id}/status`, { status })
}

export const deleteMCP = (id: string) => {
  return del(`/mcp/public/${id}`)
}

// Embed MCP API functions
export const getEmbedMCPs = () => {
  return get<EmbedMCP[]>('/embedmcp/')
}

export const saveEmbedMCP = (data: SaveEmbedMCPRequest) => {
  return post('/embedmcp/', data)
}

// MCP Reusing Params API functions
export const getMCPReusingParams = (mcpId: string, groupId: string) => {
  return get<PublicMCPReusingParam>(`/mcp/public/${mcpId}/group/${groupId}/params`)
}

export const saveMCPReusingParams = (
  mcpId: string,
  groupId: string,
  data: PublicMCPReusingParam
) => {
  return post(`/mcp/public/${mcpId}/group/${groupId}/params`, data)
}
