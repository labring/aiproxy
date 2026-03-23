import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/use-toast";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import {
  AlertCircle,
  Trash2,
  Plus,
  Settings,
  KeyRound,
  ShieldAlert,
} from "lucide-react";
import {
  PublicMCP,
  createMCP,
  updateMCP,
  getAllMCPs,
  deleteMCP,
  updateMCPStatus,
  PublicMCPProxyConfig,
  MCPOpenAPIConfig,
} from "@/api/mcp";
import {
  Dialog,
  DialogContent,
  DialogTrigger,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogHeader,
} from "@/components/ui/dialog";
import { CopyButton } from "@/components/common/CopyButton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import ProxyConfig from "./config/ProxyConfig";
import OpenAPIConfig from "./config/OpenAPIConfig";

const initialMCP: Omit<PublicMCP, "created_at" | "update_at" | "endpoints"> = {
  id: "",
  name: "",
  status: 1, // Enabled by default
  type: "mcp_proxy_sse", // Default type
  readme: "",
  tags: [],
  logo_url: "",
};

const MCPConfig = () => {
  const [mcps, setMCPs] = useState<PublicMCP[]>([]);
  const [newMCP, setNewMCP] =
    useState<Omit<PublicMCP, "created_at" | "update_at" | "endpoints">>(
      initialMCP
    );
  const [editMCP, setEditMCP] =
    useState<Omit<PublicMCP, "created_at" | "update_at" | "endpoints">>(
      initialMCP
    );
  const [tagInput, setTagInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [mcpToDelete, setMcpToDelete] = useState<PublicMCP | null>(null);
  const [authMethod, setAuthMethod] = useState<"query" | "header">("query");
  const { toast } = useToast();

  useEffect(() => {
    fetchMCPs();
  }, []);

  const fetchMCPs = async () => {
    try {
      setLoading(true);
      const data = await getAllMCPs();
      setMCPs(data);
    } catch {
      toast({
        title: "Error",
        description: "Failed to fetch MCPs",
        variant: "destructive",
      });
    } finally {
      setLoading(false);
    }
  };

  const handleCreateChange = (
    field: keyof typeof newMCP,
    value: string | number | string[] | PublicMCPProxyConfig | MCPOpenAPIConfig
  ) => {
    setNewMCP((prev) => ({ ...prev, [field]: value }));
  };

  const handleEditChange = (
    field: keyof typeof editMCP,
    value: string | number | string[] | PublicMCPProxyConfig | MCPOpenAPIConfig
  ) => {
    setEditMCP((prev) => ({ ...prev, [field]: value }));
  };

  const handleAddTag = () => {
    if (tagInput.trim()) {
      if (isEditing) {
        setEditMCP((prev) => ({
          ...prev,
          tags: [...(prev.tags || []), tagInput.trim()],
        }));
      } else {
        setNewMCP((prev) => ({
          ...prev,
          tags: [...(prev.tags || []), tagInput.trim()],
        }));
      }
      setTagInput("");
    }
  };

  const handleRemoveTag = (tag: string) => {
    if (isEditing) {
      setEditMCP((prev) => ({
        ...prev,
        tags: (prev.tags || []).filter((t) => t !== tag),
      }));
    } else {
      setNewMCP((prev) => ({
        ...prev,
        tags: (prev.tags || []).filter((t) => t !== tag),
      }));
    }
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      const mcpData = isEditing ? editMCP : newMCP;

      // Basic validation
      if (!mcpData.id.trim() || !mcpData.name.trim() || !mcpData.type) {
        toast({
          title: "Validation Error",
          description: "ID, name, and type are required fields",
          variant: "destructive",
        });
        return;
      }

      // Type-specific validation
      if (
        mcpData.type === "mcp_proxy_sse" ||
        mcpData.type === "mcp_proxy_streamable"
      ) {
        if (!mcpData.proxy_config?.url) {
          toast({
            title: "Validation Error",
            description: "Backend URL is required for proxy MCP",
            variant: "destructive",
          });
          return;
        }
      } else if (mcpData.type === "mcp_openapi") {
        if (
          !mcpData.openapi_config?.openapi_spec &&
          !mcpData.openapi_config?.openapi_content
        ) {
          toast({
            title: "Validation Error",
            description: "OpenAPI specification URL or content is required",
            variant: "destructive",
          });
          return;
        }
      }

      // Create or update MCP
      if (isEditing) {
        await updateMCP(mcpData.id, mcpData as PublicMCP);
        toast({
          title: "Success",
          description: "MCP updated successfully",
        });
      } else {
        await createMCP(mcpData as PublicMCP);
        toast({
          title: "Success",
          description: "MCP created successfully",
        });
      }

      // Reset form and refresh the list
      setNewMCP(initialMCP);
      setEditMCP(initialMCP);
      setIsEditing(false);
      setShowCreateForm(false);
      fetchMCPs();
    } catch {
      toast({
        title: "Error",
        description: "Failed to save MCP configuration",
        variant: "destructive",
      });
    } finally {
      setLoading(false);
    }
  };

  const handleReset = () => {
    if (isEditing) {
      setEditMCP(initialMCP);
    } else {
      setNewMCP(initialMCP);
    }
    setTagInput("");
  };

  const handleStatusToggle = async (id: string, currentStatus: number) => {
    // 检查是否为mcp_embed类型
    const mcpToToggle = mcps.find((mcp) => mcp.id === id);

    if (mcpToToggle?.type === "mcp_embed") {
      toast({
        title: "Not Allowed",
        description:
          "Embedded MCP servers status cannot be changed from this interface.",
        variant: "destructive",
      });
      return;
    }

    const newStatus = currentStatus === 1 ? 2 : 1; // Toggle between enabled (1) and disabled (2)

    try {
      await updateMCPStatus(id, newStatus);
      setMCPs(
        mcps.map((mcp) => (mcp.id === id ? { ...mcp, status: newStatus } : mcp))
      );
      toast({
        title: "Success",
        description: `MCP ${
          newStatus === 1 ? "enabled" : "disabled"
        } successfully`,
      });
    } catch {
      toast({
        title: "Error",
        description: "Failed to update MCP status",
        variant: "destructive",
      });
    }
  };

  const handleConfirmDelete = async () => {
    if (!mcpToDelete) return;

    try {
      await deleteMCP(mcpToDelete.id);
      setMCPs(mcps.filter((mcp) => mcp.id !== mcpToDelete.id));
      toast({
        title: "Success",
        description: "MCP deleted successfully",
      });
      setDeleteConfirmOpen(false);
      setMcpToDelete(null);
    } catch {
      toast({
        title: "Error",
        description: "Failed to delete MCP",
        variant: "destructive",
      });
    }
  };

  const handleDeleteClick = (mcp: PublicMCP) => {
    // 检查是否为mcp_embed类型
    if (mcp.type === "mcp_embed") {
      toast({
        title: "Not Allowed",
        description:
          "Embedded MCP servers cannot be deleted from this interface.",
        variant: "destructive",
      });
      return;
    }

    setMcpToDelete(mcp);
    setDeleteConfirmOpen(true);
  };

  const handleEdit = (mcpToEdit: PublicMCP) => {
    // 如果是mcp_embed类型，显示警告并阻止编辑
    if (mcpToEdit.type === "mcp_embed") {
      toast({
        title: "Not Allowed",
        description: "Embedded MCP servers cannot be edited.",
        variant: "destructive",
      });
      return;
    }

    // 提取需要的字段
    const {
      id,
      name,
      status,
      type,
      readme,
      tags,
      logo_url,
      proxy_config,
      openapi_config,
      embed_config,
    } = mcpToEdit;
    setEditMCP({
      id,
      name,
      status,
      type,
      readme,
      tags,
      logo_url,
      proxy_config,
      openapi_config,
      embed_config,
    });
    setIsEditing(true);
    setShowCreateForm(true);
  };

  const handleOpenCreateForm = () => {
    setNewMCP(initialMCP); // 确保创建表单始终是空的
    setIsEditing(false);
    setShowCreateForm(true);
  };

  const handleTypeChange = (type: string) => {
    if (isEditing) {
      // 当编辑已有MCP时，只更新类型，不重置配置
      setEditMCP((prev) => ({ ...prev, type }));
    } else {
      // 当创建新MCP时，根据类型初始化相应配置
      const updatedMCP = { ...newMCP, type };

      if (type === "mcp_proxy_sse" || type === "mcp_proxy_streamable") {
        if (!updatedMCP.proxy_config) {
          updatedMCP.proxy_config = {
            url: "",
            headers: {},
            querys: {},
            reusing: {},
          };
        }
      } else if (type === "mcp_openapi") {
        if (!updatedMCP.openapi_config) {
          updatedMCP.openapi_config = {
            openapi_spec: "",
            v2: false,
          };
        }
      }

      setNewMCP(updatedMCP);
    }
  };

  const filteredMCPs = mcps.filter(
    (mcp) =>
      mcp.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      mcp.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      mcp.tags?.some((tag) =>
        tag.toLowerCase().includes(searchTerm.toLowerCase())
      )
  );

  // 获取当前正在使用的MCP数据（编辑时使用editMCP，创建时使用newMCP）
  const currentMCP = isEditing ? editMCP : newMCP;

  // 根据当前MCP类型决定是否显示类型特定的配置
  const showProxyConfig =
    currentMCP.type === "mcp_proxy_sse" ||
    currentMCP.type === "mcp_proxy_streamable";
  const showOpenAPIConfig = currentMCP.type === "mcp_openapi";

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">MCP Configuration</h2>
        <Dialog
          open={showCreateForm}
          onOpenChange={(open) => {
            if (open) {
              // 如果是打开对话框，不做处理，因为handleOpenCreateForm会处理
            } else {
              // 如果是关闭对话框，重置状态
              setShowCreateForm(false);
            }
          }}
        >
          <DialogTrigger asChild>
            <Button onClick={handleOpenCreateForm}>
              <Plus className="mr-2 h-4 w-4" />
              Create New MCP
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
            <Card className="border-0 shadow-none">
              <CardHeader>
                <CardTitle>
                  {isEditing ? "Edit MCP" : "Create New MCP"}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-6">
                  <h3 className="text-lg font-medium">Basic Information</h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label htmlFor="id">
                        ID <span className="text-red-500">*</span>
                      </Label>
                      <Input
                        id="id"
                        value={currentMCP.id}
                        onChange={(e) =>
                          isEditing
                            ? handleEditChange("id", e.target.value)
                            : handleCreateChange("id", e.target.value)
                        }
                        placeholder="e.g., my-mcp-server"
                        disabled={isEditing}
                      />
                      <p className="text-xs text-muted-foreground">
                        Unique identifier for the MCP, alphanumeric with dashes
                        only
                      </p>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="name">
                        Name <span className="text-red-500">*</span>
                      </Label>
                      <Input
                        id="name"
                        value={currentMCP.name}
                        onChange={(e) =>
                          isEditing
                            ? handleEditChange("name", e.target.value)
                            : handleCreateChange("name", e.target.value)
                        }
                        placeholder="e.g., My MCP Server"
                      />
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="type">
                        Type <span className="text-red-500">*</span>
                      </Label>
                      <Select
                        value={currentMCP.type}
                        onValueChange={handleTypeChange}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Select MCP type" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="mcp_proxy_sse">
                            MCP Proxy SSE
                          </SelectItem>
                          <SelectItem value="mcp_proxy_streamable">
                            MCP Proxy Streamable
                          </SelectItem>
                          <SelectItem value="mcp_openapi">
                            MCP OpenAPI
                          </SelectItem>
                          <SelectItem value="mcp_docs">
                            MCP Documentation
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="logo_url">Logo URL</Label>
                      <Input
                        id="logo_url"
                        value={currentMCP.logo_url}
                        onChange={(e) =>
                          isEditing
                            ? handleEditChange("logo_url", e.target.value)
                            : handleCreateChange("logo_url", e.target.value)
                        }
                        placeholder="https://example.com/logo.png"
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="tags">Tags</Label>
                    <div className="flex gap-2">
                      <Input
                        id="tags"
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        placeholder="Add a tag"
                        onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) =>
                          e.key === "Enter" &&
                          (e.preventDefault(), handleAddTag())
                        }
                      />
                      <Button type="button" onClick={handleAddTag}>
                        Add
                      </Button>
                    </div>
                    {currentMCP.tags && currentMCP.tags.length > 0 && (
                      <div className="flex flex-wrap gap-2 mt-2">
                        {currentMCP.tags.map((tag) => (
                          <div
                            key={tag}
                            className="bg-muted text-muted-foreground rounded-md px-2 py-1 text-sm flex items-center"
                          >
                            {tag}
                            <button
                              type="button"
                              className="ml-2 text-red-500 hover:text-red-700"
                              onClick={() => handleRemoveTag(tag)}
                            >
                              ×
                            </button>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="readme">Readme</Label>
                    <Textarea
                      id="readme"
                      value={currentMCP.readme}
                      onChange={(e) =>
                        isEditing
                          ? handleEditChange("readme", e.target.value)
                          : handleCreateChange("readme", e.target.value)
                      }
                      placeholder="Markdown supported"
                      className="min-h-[200px]"
                    />
                    <p className="text-xs text-muted-foreground">
                      Provide documentation for using this MCP
                    </p>
                  </div>
                </div>

                {showProxyConfig && (
                  <div className="space-y-6 border-t pt-6">
                    <h3 className="text-lg font-medium">Proxy Configuration</h3>
                    <ProxyConfig
                      config={currentMCP.proxy_config}
                      onChange={(config) =>
                        isEditing
                          ? handleEditChange("proxy_config", config)
                          : handleCreateChange("proxy_config", config)
                      }
                    />
                  </div>
                )}

                {showOpenAPIConfig && (
                  <div className="space-y-6 border-t pt-6">
                    <h3 className="text-lg font-medium">
                      OpenAPI Configuration
                    </h3>
                    <OpenAPIConfig
                      config={currentMCP.openapi_config}
                      onChange={(config) =>
                        isEditing
                          ? handleEditChange("openapi_config", config)
                          : handleCreateChange("openapi_config", config)
                      }
                    />
                  </div>
                )}

                <div className="flex justify-end space-x-2 pt-4 border-t">
                  <Button variant="outline" onClick={handleReset}>
                    Reset
                  </Button>
                  <Button onClick={handleSubmit} disabled={loading}>
                    {loading
                      ? "Saving..."
                      : isEditing
                      ? "Update MCP"
                      : "Create MCP"}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </DialogContent>
        </Dialog>
      </div>

      {/* 删除确认对话框 */}
      <Dialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center">
              <AlertCircle className="h-5 w-5 text-red-500 mr-2" />
              Delete MCP
            </DialogTitle>
            <DialogDescription>
              Are you sure you want to delete the MCP "{mcpToDelete?.name}"?
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <div className="p-4 bg-muted rounded-md mb-4">
            <div className="font-medium">{mcpToDelete?.name}</div>
            <div className="text-sm text-muted-foreground">
              ID: {mcpToDelete?.id}
            </div>
            <div className="text-sm text-muted-foreground">
              Type: {mcpToDelete?.type}
            </div>
          </div>
          <DialogFooter className="sm:justify-end">
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteConfirmOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleConfirmDelete}
            >
              <Trash2 className="h-4 w-4 mr-2" />
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Tabs defaultValue="all">
        <TabsList>
          <TabsTrigger value="all">All MCPs</TabsTrigger>
          <TabsTrigger value="enabled">Enabled</TabsTrigger>
          <TabsTrigger value="disabled">Disabled</TabsTrigger>
        </TabsList>

        <div className="mt-4 flex justify-between items-center">
          <Input
            className="max-w-xs"
            placeholder="Search by name, ID, or tag..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
          <Button onClick={fetchMCPs}>Refresh</Button>
        </div>

        <TabsContent value="all" className="mt-4">
          {renderMCPList(filteredMCPs)}
        </TabsContent>

        <TabsContent value="enabled" className="mt-4">
          {renderMCPList(filteredMCPs.filter((mcp) => mcp.status === 1))}
        </TabsContent>

        <TabsContent value="disabled" className="mt-4">
          {renderMCPList(filteredMCPs.filter((mcp) => mcp.status === 2))}
        </TabsContent>
      </Tabs>
    </div>
  );

  function renderMCPList(mcpList: PublicMCP[]) {
    if (loading) {
      return <div className="flex justify-center p-8">Loading MCPs...</div>;
    }

    if (mcpList.length === 0) {
      return <div className="text-center p-8">No MCPs found</div>;
    }

    // 获取当前协议
    const protocol = window.location.protocol;

    // 格式化URL，确保带有正确的协议前缀
    const formatEndpointUrl = (host: string, path: string) => {
      // 如果host已经包含协议，直接返回完整URL
      if (host.startsWith("http://") || host.startsWith("https://")) {
        return `${host}${path}`;
      }

      // 否则，根据当前页面协议添加协议前缀
      return `${protocol}//${host}${path}`;
    };

    // 生成带有认证的URL
    const getAuthenticatedUrl = (url: string) => {
      if (authMethod === "query") {
        return `${url}${url.includes("?") ? "&" : "?"}key=your-token`;
      }
      return url;
    };

    // 获取MCP类型的显示信息
    const getTypeInfo = (type: string) => {
      switch (type) {
        case "mcp_proxy_sse":
          return {
            label: "Proxy SSE",
            color:
              "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300",
          };
        case "mcp_proxy_streamable":
          return {
            label: "Proxy Streamable",
            color:
              "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300",
          };
        case "mcp_openapi":
          return {
            label: "OpenAPI",
            color:
              "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300",
          };
        case "mcp_docs":
          return {
            label: "Docs",
            color:
              "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300",
          };
        case "mcp_embed":
          return {
            label: "Embed",
            color:
              "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300",
          };
        default:
          return {
            label: type,
            color:
              "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300",
          };
      }
    };

    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {mcpList.map((mcp) => (
          <Card key={mcp.id} className="overflow-hidden">
            <CardHeader>
              <div className="flex justify-between items-start">
                <div>
                  <CardTitle className="flex items-center">
                    {mcp.name}
                    <Badge
                      className="ml-2"
                      variant={mcp.status === 1 ? "default" : "secondary"}
                    >
                      {mcp.status === 1 ? "Enabled" : "Disabled"}
                    </Badge>
                  </CardTitle>
                  <div className="text-sm text-muted-foreground">{mcp.id}</div>
                  <div className="mt-1">
                    <span
                      className={`text-xs px-2 py-1 rounded-full ${
                        getTypeInfo(mcp.type).color
                      }`}
                    >
                      {getTypeInfo(mcp.type).label}
                    </span>
                  </div>
                </div>
                <div className="flex space-x-2">
                  {mcp.type !== "mcp_embed" && (
                    <>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleEdit(mcp)}
                      >
                        <Settings className="h-4 w-4" />
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => handleDeleteClick(mcp)}
                      >
                        Delete
                      </Button>
                      <Button
                        size="sm"
                        variant={mcp.status === 1 ? "destructive" : "default"}
                        onClick={() => handleStatusToggle(mcp.id, mcp.status)}
                      >
                        {mcp.status === 1 ? "Disable" : "Enable"}
                      </Button>
                    </>
                  )}
                </div>
              </div>
              <div className="flex flex-wrap gap-1 mt-1">
                {mcp.tags?.map((tag) => (
                  <Badge key={tag} variant="outline">
                    {tag}
                  </Badge>
                ))}
              </div>
            </CardHeader>
            <CardContent className="space-y-2">
              {(mcp.endpoints?.sse || mcp.endpoints?.streamable_http) && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <div className="font-medium">Endpoints:</div>
                    <div className="flex items-center gap-2">
                      <div className="text-xs text-muted-foreground">Auth:</div>
                      <Tabs
                        value={authMethod}
                        onValueChange={(v) =>
                          setAuthMethod(v as "query" | "header")
                        }
                        className="h-7"
                      >
                        <TabsList className="h-7 p-0.5">
                          <TabsTrigger
                            value="query"
                            className="h-6 text-xs px-2 py-0.5 flex items-center gap-1"
                          >
                            <KeyRound className="h-3 w-3" />
                            Query
                          </TabsTrigger>
                          <TabsTrigger
                            value="header"
                            className="h-6 text-xs px-2 py-0.5 flex items-center gap-1"
                          >
                            <ShieldAlert className="h-3 w-3" />
                            Header
                          </TabsTrigger>
                        </TabsList>
                      </Tabs>
                    </div>
                  </div>

                  {mcp.endpoints?.sse && (
                    <div className="mb-3">
                      <div className="flex items-center justify-between mb-1">
                        <div className="text-sm font-medium">SSE:</div>
                        <CopyButton
                          text={formatEndpointUrl(
                            mcp.endpoints.host,
                            mcp.endpoints.sse
                          )}
                        />
                      </div>
                      <div className="relative">
                        <div className="text-xs bg-muted p-2 rounded-md overflow-x-auto whitespace-nowrap font-mono">
                          {authMethod === "query"
                            ? getAuthenticatedUrl(
                                formatEndpointUrl(
                                  mcp.endpoints.host,
                                  mcp.endpoints.sse
                                )
                              )
                            : formatEndpointUrl(
                                mcp.endpoints.host,
                                mcp.endpoints.sse
                              )}
                        </div>
                      </div>
                    </div>
                  )}

                  {mcp.endpoints?.streamable_http && (
                    <div>
                      <div className="flex items-center justify-between mb-1">
                        <div className="text-sm font-medium">HTTP:</div>
                        <CopyButton
                          text={formatEndpointUrl(
                            mcp.endpoints.host,
                            mcp.endpoints.streamable_http
                          )}
                        />
                      </div>
                      <div className="relative">
                        <div className="text-xs bg-muted p-2 rounded-md overflow-x-auto whitespace-nowrap font-mono">
                          {authMethod === "query"
                            ? getAuthenticatedUrl(
                                formatEndpointUrl(
                                  mcp.endpoints.host,
                                  mcp.endpoints.streamable_http
                                )
                              )
                            : formatEndpointUrl(
                                mcp.endpoints.host,
                                mcp.endpoints.streamable_http
                              )}
                        </div>
                      </div>
                    </div>
                  )}

                  <div className="mt-2 text-xs text-muted-foreground bg-muted/50 p-2 rounded-md">
                    <div className="flex items-center gap-1 mb-1">
                      <ShieldAlert className="h-3 w-3" />
                      <span className="font-medium">
                        Authentication Required:
                      </span>
                    </div>
                    {authMethod === "query" ? (
                      <div>
                        <div className="text-xs mb-1">Add query parameter:</div>
                        <div className="flex items-center gap-2">
                          <code className="block flex-1 font-mono bg-muted p-1 rounded">
                            key=
                            <span className="text-blue-500">your-token</span>
                          </code>
                          <CopyButton
                            text="key=your-token"
                            className="h-6 w-6 p-0"
                          />
                        </div>
                      </div>
                    ) : (
                      <div>
                        <div className="text-xs mb-1">Add HTTP header:</div>
                        <div className="flex items-center gap-2">
                          <code className="block flex-1 font-mono bg-muted p-1 rounded">
                            Authorization: Bearer{" "}
                            <span className="text-blue-500">your-token</span>
                          </code>
                          <CopyButton
                            text="Authorization: Bearer your-token"
                            className="h-6 w-6 p-0"
                          />
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              <div className="text-sm text-muted-foreground mt-2">
                <div>Created: {new Date(mcp.created_at).toLocaleString()}</div>
                <div>Updated: {new Date(mcp.update_at).toLocaleString()}</div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }
};

export default MCPConfig;
