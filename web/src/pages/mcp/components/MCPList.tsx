import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/use-toast";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PublicMCP, getAllMCPs } from "@/api/mcp";
import { CopyButton } from "@/components/common/CopyButton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { useTranslation } from "react-i18next";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { KeyRound, ShieldAlert } from "lucide-react";

const MCPList = () => {
  const [mcps, setMcps] = useState<PublicMCP[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [authMethod, setAuthMethod] = useState<"query" | "header">("query");
  const { toast } = useToast();
  const { t } = useTranslation();

  useEffect(() => {
    fetchMCPs();
  }, []);

  const fetchMCPs = async () => {
    try {
      setLoading(true);
      const data = await getAllMCPs();
      // 只保留状态为1（已启用）的MCP
      const enabledMCPs = data.filter((mcp) => mcp.status === 1);
      setMcps(enabledMCPs);
    } catch {
      toast({
        title: t("error.loading"),
        description: t("mcp.list.noResults"),
        variant: "destructive",
      });
    } finally {
      setLoading(false);
    }
  };

  const truncateReadme = (readme: string, maxLength = 100) => {
    if (readme.length <= maxLength) return readme;
    return readme.substring(0, maxLength) + "...";
  };

  // 移除Markdown语法以显示纯文本预览
  const stripMarkdown = (markdown: string) => {
    return markdown
      .replace(/#+\s+/g, "") // 移除标题标记
      .replace(/\*\*(.*?)\*\*/g, "$1") // 移除加粗
      .replace(/\*(.*?)\*/g, "$1") // 移除斜体
      .replace(/\[(.*?)\]\(.*?\)/g, "$1") // 移除链接，只保留文本
      .replace(/`{1,3}(.*?)`{1,3}/g, "$1") // 移除代码块
      .replace(/~~(.*?)~~/g, "$1") // 移除删除线
      .replace(/>\s+(.*?)\n/g, "$1\n") // 移除引用
      .replace(/\n\s*[-*+]\s+/g, "\n"); // 移除列表标记
  };

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

  // 获取认证的URL
  const getAuthenticatedUrl = (url: string) => {
    if (authMethod === "query") {
      return `${url}${url.includes("?") ? "&" : "?"}key=your-token`;
    }
    return url;
  };

  // 获取认证详细信息
  const getAuthDetails = () => {
    if (authMethod === "query") {
      return (
        <div>
          <div className="text-xs mb-1">Add query parameter:</div>
          <div className="flex items-center gap-2">
            <code className="block flex-1 font-mono bg-muted p-1 rounded">
              key=<span className="text-blue-500">your-token</span>
            </code>
            <CopyButton text="key=your-token" className="h-6 w-6 p-0" />
          </div>
        </div>
      );
    } else {
      return (
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
      );
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

  if (loading) {
    return <div className="flex justify-center p-8">{t("common.loading")}</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <Input
          className="max-w-xs"
          placeholder={t("mcp.list.search")}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
        <Button onClick={fetchMCPs}>{t("mcp.refresh")}</Button>
      </div>

      {filteredMCPs.length === 0 ? (
        <div className="text-center p-8">{t("mcp.list.noResults")}</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredMCPs.map((mcp) => (
            <Dialog
              key={mcp.id}
            >
              <DialogTrigger asChild>
                <Card className="overflow-hidden cursor-pointer hover:shadow-md transition-shadow">
                  <CardHeader>
                    <div className="flex justify-between items-start">
                      <div>
                        <CardTitle className="flex items-center">
                          {mcp.name}
                        </CardTitle>
                        <div className="text-sm text-muted-foreground">
                          {mcp.id}
                        </div>
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
                    {mcp.readme && (
                      <div className="p-3 bg-muted rounded-md text-sm mb-2">
                        <div className="font-medium mb-1">
                          {t("mcp.description")}:
                        </div>
                        <div className="text-muted-foreground line-clamp-3">
                          {truncateReadme(stripMarkdown(mcp.readme))}
                        </div>
                      </div>
                    )}

                    {(mcp.endpoints.sse || mcp.endpoints.streamable_http) && (
                      <div>
                        <div className="flex items-center justify-between mb-1">
                          <div className="font-medium">
                            {t("mcp.endpoint")}:
                          </div>
                          <div
                            className="flex items-center gap-2"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <div className="text-xs text-muted-foreground">
                              Auth:
                            </div>
                            <Tabs
                              value={authMethod}
                              onValueChange={(v) =>
                                setAuthMethod(v as "query" | "header")
                              }
                              className="h-6"
                            >
                              <TabsList className="h-6 p-0.5">
                                <TabsTrigger
                                  value="query"
                                  className="h-5 text-xs px-1.5 py-0 flex items-center gap-1"
                                >
                                  <KeyRound className="h-3 w-3" />
                                  <span className="hidden sm:inline">
                                    Query
                                  </span>
                                </TabsTrigger>
                                <TabsTrigger
                                  value="header"
                                  className="h-5 text-xs px-1.5 py-0 flex items-center gap-1"
                                >
                                  <ShieldAlert className="h-3 w-3" />
                                  <span className="hidden sm:inline">
                                    Header
                                  </span>
                                </TabsTrigger>
                              </TabsList>
                            </Tabs>
                          </div>
                        </div>

                        {mcp.endpoints.sse && (
                          <div
                            className="mb-1"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <div className="flex items-center justify-between mb-0.5">
                              <div className="text-xs font-medium">SSE:</div>
                              <CopyButton
                                text={formatEndpointUrl(
                                  mcp.endpoints.host,
                                  mcp.endpoints.sse
                                )}
                                className="h-5 w-5 p-0"
                              />
                            </div>
                            <div className="text-xs bg-muted p-1.5 rounded-md overflow-x-auto whitespace-nowrap font-mono">
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
                        )}

                        {mcp.endpoints.streamable_http && (
                          <div onClick={(e) => e.stopPropagation()}>
                            <div className="flex items-center justify-between mb-0.5">
                              <div className="text-xs font-medium">HTTP:</div>
                              <CopyButton
                                text={formatEndpointUrl(
                                  mcp.endpoints.host,
                                  mcp.endpoints.streamable_http
                                )}
                                className="h-5 w-5 p-0"
                              />
                            </div>
                            <div className="text-xs bg-muted p-1.5 rounded-md overflow-x-auto whitespace-nowrap font-mono">
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
                              <div className="text-xs mb-1">
                                Add query parameter:
                              </div>
                              <div className="flex items-center gap-2">
                                <code className="block flex-1 font-mono bg-muted p-1 rounded">
                                  key=
                                  <span className="text-blue-500">
                                    your-token
                                  </span>
                                </code>
                                <CopyButton
                                  text="key=your-token"
                                  className="h-6 w-6 p-0"
                                />
                              </div>
                            </div>
                          ) : (
                            <div>
                              <div className="text-xs mb-1">
                                Add HTTP header:
                              </div>
                              <div className="flex items-center gap-2">
                                <code className="block flex-1 font-mono bg-muted p-1 rounded">
                                  Authorization: Bearer{" "}
                                  <span className="text-blue-500">
                                    your-token
                                  </span>
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
                  </CardContent>
                </Card>
              </DialogTrigger>
              <DialogContent className="max-w-3xl max-h-[80vh] overflow-y-auto">
                <DialogHeader>
                  <DialogTitle className="flex items-center">
                    {mcp.name}
                  </DialogTitle>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="flex items-center space-x-2">
                    <span className="font-medium">ID:</span>
                    <span>{mcp.id}</span>
                  </div>

                  {mcp.tags && mcp.tags.length > 0 && (
                    <div>
                      <div className="font-medium mb-1">{t("mcp.tags")}:</div>
                      <div className="flex flex-wrap gap-1">
                        {mcp.tags.map((tag) => (
                          <Badge key={tag} variant="outline">
                            {tag}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}

                  {mcp.readme && (
                    <div>
                      <div className="font-medium mb-1">
                        {t("mcp.description")}:
                      </div>
                      <div className="p-4 bg-muted rounded-md max-h-[300px] overflow-y-auto">
                        <div className="prose prose-sm dark:prose-invert max-w-none">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>
                            {mcp.readme}
                          </ReactMarkdown>
                        </div>
                      </div>
                    </div>
                  )}

                  {(mcp.endpoints.sse || mcp.endpoints.streamable_http) && (
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <div className="font-medium">{t("mcp.endpoint")}:</div>
                        <div className="flex items-center gap-2">
                          <div className="text-xs text-muted-foreground">
                            Auth:
                          </div>
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

                      {mcp.endpoints.sse && (
                        <div className="mb-3">
                          <div className="flex items-center justify-between mb-1">
                            <div className="text-sm font-medium">
                              {t("mcp.list.endpointsSse")}:
                            </div>
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

                      {mcp.endpoints.streamable_http && (
                        <div>
                          <div className="flex items-center justify-between mb-1">
                            <div className="text-sm font-medium">
                              {t("mcp.list.endpointsHttp")}:
                            </div>
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
                        {getAuthDetails()}
                      </div>
                    </div>
                  )}

                  <div className="flex items-center space-x-2">
                    <span className="font-medium">
                      {t("mcp.list.createdAt")}:
                    </span>
                    <span>{new Date(mcp.created_at).toLocaleString()}</span>
                  </div>

                  <div className="flex items-center space-x-2">
                    <span className="font-medium">
                      {t("mcp.list.updatedAt")}:
                    </span>
                    <span>{new Date(mcp.update_at).toLocaleString()}</span>
                  </div>
                </div>
              </DialogContent>
            </Dialog>
          ))}
        </div>
      )}
    </div>
  );
};

export default MCPList;
